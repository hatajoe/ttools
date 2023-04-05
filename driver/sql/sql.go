package sql

import (
	"context"
	d "database/sql/driver"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	proxy "github.com/shogo82148/go-sql-proxy"
)

var (
	sqlTracingEnabled bool   = false
	dbName            string = "ttools"
	openCount         int64  = 0
)

// Tracing set flag which determines to show executed SQL in the console
func Tracing(enabled bool) {
	sqlTracingEnabled = enabled
}

// TestDatabase set database name that used while in test
func TestDatabase(db string) {
	dbName = db
}

// SetHooks sets context hooks
func SetHooks(driver d.Driver) d.Driver {
	return proxy.NewProxyContext(driver, &proxy.HooksContext{
		PreOpen: func(c context.Context, name string) (interface{}, error) {
			atomic.AddInt64(&openCount, 1)
			return nil, nil
		},
		PostOpen: func(c context.Context, ctx interface{}, conn *proxy.Conn, err error) error {
			if err != nil {
				return err
			}
			if _, err := conn.ExecContext(c, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName), []d.NamedValue{}); err != nil {
				return err
			}
			if _, err := conn.ExecContext(c, fmt.Sprintf("USE %s", dbName), []d.NamedValue{}); err != nil {
				return err
			}
			return err
		},
		PreExec: func(_ context.Context, _ *proxy.Stmt, _ []d.NamedValue) (interface{}, error) {
			return time.Now(), nil
		},
		PostExec: func(c context.Context, ctx interface{}, stmt *proxy.Stmt, args []d.NamedValue, res d.Result, _ error) error {
			if sqlTracingEnabled {
				log.Printf("Query: %s; args = %#v (%s conn:%p)\n", stmt.QueryString, args, time.Since(ctx.(time.Time)), stmt.Conn)
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
		PreBegin: func(c context.Context, conn *proxy.Conn) (interface{}, error) {
			return time.Now(), nil
		},
		PostBegin: func(c context.Context, ctx interface{}, conn *proxy.Conn, err error) error {
			if sqlTracingEnabled {
				log.Printf("Query: BEGIN; (%s conn:%p)\n", time.Since(ctx.(time.Time)), conn)
			}
			return nil
		},
		PreCommit: func(c context.Context, tx *proxy.Tx) (interface{}, error) {
			return time.Now(), nil
		},
		PostCommit: func(c context.Context, ctx interface{}, tx *proxy.Tx, err error) error {
			if sqlTracingEnabled {
				log.Printf("Query: COMMIT; (%s conn:%p)\n", time.Since(ctx.(time.Time)), tx.Conn)
			}
			return nil
		},
		PreRollback: func(c context.Context, tx *proxy.Tx) (interface{}, error) {
			return time.Now(), nil
		},
		PostRollback: func(c context.Context, ctx interface{}, tx *proxy.Tx, err error) error {
			if sqlTracingEnabled {
				log.Printf("Query: ROLLBACK; (%s conn:%p)\n", time.Since(ctx.(time.Time)), tx.Conn)
			}
			return nil
		},
		PreClose: func(c context.Context, conn *proxy.Conn) (interface{}, error) {
			if 0 < atomic.AddInt64(&openCount, -1) {
				return nil, nil
			}
			if _, err := conn.ExecContext(c, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName), []d.NamedValue{}); err != nil {
				log.Println("DROP DATABASE is not supported by sqlite3.")
			}
			return nil, nil
		},
	})
}
