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
	sqlTracingEnabled bool
	openCount         int64
	mu                *sync.Mutex        = &sync.Mutex{}
	regexpNewLine     *regexp.Regexp     = regexp.MustCompile(`\n`)
	regexpInsert      *regexp.Regexp     = regexp.MustCompile(`INSERT`)
	regexpTable       *regexp.Regexp     = regexp.MustCompile(`INSERT INTO(.*)\(.*VALUES`)
	inserted          map[string][]int64 = map[string][]int64{}
	deletionQuery     string             = `DELETE FROM %s WHERE id = ?`
)

// Register driver which will be set followin hooks
// SQL Tracing: show executed queries and processing time by query. no display is default.
// Auto Deletion: delete all inserted records after close the last opend connection.
func Register(driverName string, driver d.Driver) {
	sql.Register(driverName, setHooks(driver))
}

// Open connection by registered driver
func Open(driverName string, dataSource string) (*sql.DB, error) {
	atomic.AddInt64(&openCount, 1)
	return sql.Open(driverName, dataSource)
}

// Tracing set flag which determines to show executed SQL in the console
func Tracing(enabled bool) {
	sqlTracingEnabled = enabled
}

func setHooks(driver d.Driver) d.Driver {
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
				log.Printf("`SET foreign_key_checks` is not supported, but continue to preClose hooks. err: %v\n", err)
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
					log.Printf("`ALTER TABLE %s auto_increment = 1` is not supported, but continue to preClose hooks. err: %v\n", table, err)
				}
			}
			return nil, nil
		},
	})
}
