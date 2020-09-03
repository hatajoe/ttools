package sql

import (
	"context"
	"database/sql"
	d "database/sql/driver"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	proxy "github.com/shogo82148/go-sql-proxy"
)

var (
	db                *sql.DB
	sqlTracingEnabled bool
	openCount         int64
	driverName        string             = "tsql"
	mu                *sync.Mutex        = &sync.Mutex{}
	regexpNewLine     *regexp.Regexp     = regexp.MustCompile(`\n`)
	regexpInsert      *regexp.Regexp     = regexp.MustCompile(`INSERT`)
	regexpTable       *regexp.Regexp     = regexp.MustCompile(`INSERT INTO(.*)\(.*VALUES`)
	inserted          map[string][]int64 = map[string][]int64{}
	deletionQuery     string             = `DELETE FROM %s WHERE id = ?`
)

// Open returns *sql.DB which contained following hooks
// SQL Tracing: show executed queries and processing time by query. no display is default.
// Auto Deletion: delete all inserted records after close the connection.
func Open(driver d.Driver, dataSource string) (*sql.DB, error) {
	atomic.AddInt64(&openCount, 1)
	if db != nil {
		return db, nil
	}

	alreadyRegistered := false
	drivers := sql.Drivers()
	for _, dn := range drivers {
		if driverName == dn {
			alreadyRegistered = true
		}
	}
	if !alreadyRegistered {
		sql.Register(driverName, SetHooks(driver))
	}
	return sql.Open(driverName, dataSource)
}

// SetHooks set hooks to driver
func SetHooks(driver d.Driver) d.Driver {
	return proxy.NewProxyContext(driver, &proxy.HooksContext{
		PreExec: func(_ context.Context, _ *proxy.Stmt, _ []d.NamedValue) (interface{}, error) {
			return time.Now(), nil
		},
		PostExec: func(_ context.Context, ctx interface{}, stmt *proxy.Stmt, args []d.NamedValue, res d.Result, _ error) error {
			if sqlTracingEnabled {
				log.Printf("Query: %s; args = %#v (%s conn:%p)\n", stmt.QueryString, args, time.Since(ctx.(time.Time)), stmt.Conn)
			}

			q := regexpNewLine.ReplaceAllString(stmt.QueryString, "")
			uq := strings.ToUpper(q)
			if regexpInsert.MatchString(uq) {
				mu.Lock()
				defer mu.Unlock()

				cap := regexpTable.FindStringSubmatch(uq)
				if len(cap) < 2 {
					return fmt.Errorf("failed to parse table name from query: %s", stmt.QueryString)
				}
				table := strings.ToLower(strings.TrimSpace(cap[1]))
				if _, ok := inserted[table]; !ok {
					inserted[table] = []int64{}
				}
				if res != nil {
					lastInsertId, err := res.LastInsertId()
					if err != nil {
						return err
					}
					inserted[table] = append(inserted[table], lastInsertId)
				}
			}
			return nil
		},
		PreClose: func(c context.Context, conn *proxy.Conn) (interface{}, error) {
			mu.Lock()
			defer mu.Unlock()

			if 0 < atomic.AddInt64(&openCount, -1) {
				return nil, nil
			}
			if _, err := conn.ExecContext(c, `SET foreign_key_checks = 0`, nil); err != nil {
				return nil, err
			}
			for table, ids := range inserted {
				if len(ids) <= 0 {
					continue
				}
				stmt, err := conn.PrepareContext(c, fmt.Sprintf(deletionQuery, table))
				if err != nil {
					return nil, err
				}
				if s, ok := stmt.(*proxy.Stmt); ok {
					for _, id := range ids {
						_, err := s.ExecContext(c, []d.NamedValue{{Ordinal: 1, Value: id}})
						if err != nil {
							return nil, err
						}
					}
				}
				if _, err := conn.ExecContext(c, fmt.Sprintf("ALTER TABLE %s auto_increment = 1", table), nil); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	})
}

func SQLTracing(enabled bool) {
	sqlTracingEnabled = enabled
}
