package sql

import (
	"context"
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

// Tracing set flag which determines to show executed SQL in the console
func Tracing(enabled bool) {
	sqlTracingEnabled = enabled
}

// SetHooks sets following context hooks
// PreOpen: count opening connection in order to delete inserted records when last opend connection is closed.
// PreExec: start count for to record SQL execution time
// PostExec: show executed Query and execution time if sqlTracingEnabled is set true
// PreClose: delete all inserted records when last opened connection is closed.
func SetHooks(driver d.Driver) d.Driver {
	return proxy.NewProxyContext(driver, &proxy.HooksContext{
		PreOpen: func(c context.Context, name string) (interface{}, error) {
			atomic.AddInt64(&openCount, 1)
			return nil, nil
		},
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
		PreQuery: func(c context.Context, stmt *proxy.Stmt, args []d.NamedValue) (interface{}, error) {
			return time.Now(), nil
		},
		PostQuery: func(c context.Context, ctx interface{}, stmt *proxy.Stmt, args []d.NamedValue, rows d.Rows, err error) error {
			if sqlTracingEnabled {
				log.Printf("Query: %s; args = %#v (%s conn:%p)\n", stmt.QueryString, args, time.Since(ctx.(time.Time)), stmt.Conn)
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
