package sql

import (
	"context"
	"database/sql"
	d "database/sql/driver"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	proxy "github.com/shogo82148/go-sql-proxy"
)

var (
	driverName        string = "tsql"
	sqlTracingEnabled bool
	regexpInsert      *regexp.Regexp                     = regexp.MustCompile(`INSERT`)
	regexpTable       *regexp.Regexp                     = regexp.MustCompile(`^INSERT INTO(.*)\(.*VALUES.*$`)
	inserted          map[*proxy.Conn]map[string][]int64 = map[*proxy.Conn]map[string][]int64{}
	deletionQuery     string                             = `DELETE FROM %s WHERE id = ?`
)

// Open returns *sql.DB which contained following hooks
// SQL Tracing: show executed queries and processing time by query. no display is default.
// Auto Deletion: delete all inserted records after close the connection.
func Open(driver d.Driver, dataSource string) (*sql.DB, error) {
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

			uq := strings.ToUpper(stmt.QueryString)
			if regexpInsert.MatchString(uq) {
				cap := regexpTable.FindStringSubmatch(uq)
				if len(cap) < 2 {
					return fmt.Errorf("failed to parse table name from query: %s", stmt.QueryString)
				}
				table := strings.TrimSpace(cap[1])
				if _, ok := inserted[stmt.Conn]; !ok {
					inserted[stmt.Conn] = map[string][]int64{}
				}
				if _, ok := inserted[stmt.Conn][table]; !ok {
					inserted[stmt.Conn][table] = []int64{}
				}
				if res != nil {
					lastInsertId, err := res.LastInsertId()
					if err != nil {
						return err
					}
					inserted[stmt.Conn][table] = append(inserted[stmt.Conn][table], lastInsertId)
				}
			}
			return nil
		},
		PreClose: func(c context.Context, conn *proxy.Conn) (interface{}, error) {
			if _, ok := inserted[conn]; !ok {
				return nil, nil
			}
			for table, ids := range inserted[conn] {
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
			}
			return nil, nil
		},
	})
}

func SQLTracing(enabled bool) {
	sqlTracingEnabled = enabled
}
