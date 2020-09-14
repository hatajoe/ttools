package sql_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	tsql "github.com/hatajoe/ttools/driver/sql"
	_ "github.com/hatajoe/ttools/driver/sql/sqlite3"
	"github.com/hatajoe/ttools/gen"
)

var (
	testDBFilePath = "./test.db"
	createQuery    = `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`
	insertQuery    = `
INSERT INTO t1(id)
VALUES(?)
`
	selectQuery = `SELECT COUNT(*) FROM t1 where id IN(%s)`
)

func Test(t *testing.T) {
	if err := removeDB(); err != nil {
		t.Logf("%v. continue to test without removing %s", err, testDBFilePath)
	}

	tsql.Tracing(true)

	db, err := sql.Open("ttools-sqlite3", testDBFilePath)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err = db.ExecContext(ctx, createQuery); err != nil {
		t.Fatal(err)
	}

	fn := func() chan error {
		errCh := make(chan error)
		go func() {
			wg := &sync.WaitGroup{}
			for _, c := range []int{10, 20, 30} {
				wg.Add(1)
				go func(c int) {
					defer wg.Done()

					ctx := context.Background()
					db, err := sql.Open("ttools-sqlite3", testDBFilePath)
					if err != nil {
						errCh <- err
						return
					}
					defer func() {
						if err := db.Close(); err != nil {
							errCh <- err
						}
					}()

					ids := make([]string, 0, c)
					for i := 0; i < c; i++ {
						if res, err := db.ExecContext(ctx, insertQuery, gen.ID()); err != nil {
							errCh <- err
							return
						} else {
							id, err := res.LastInsertId()
							if err != nil {
								errCh <- err
								return
							}
							ids = append(ids, strconv.Itoa(int(id)))
						}
					}
					cnt, err := count(db, fmt.Sprintf(selectQuery, strings.Join(ids, ",")))
					if err != nil {
						errCh <- err
						return
					}

					if cnt != c {
						t.Errorf("table records count is expected to be %d, but %d count detected", c, cnt)
					}
				}(c)
			}
			wg.Wait()
			close(errCh)
		}()
		return errCh
	}

	for err := range fn() {
		if err != nil {
			t.Error(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("ttools-sqlite3", testDBFilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := count(db, `SELECT COUNT(*) FROM t1`)
	if err != nil {
		t.Fatal(err)
	}

	if cnt != 0 {
		t.Errorf("table records count is expected to be 0, but %d count detected", cnt)
	}

	if err := removeDB(); err != nil {
		t.Logf("%v. failed to remove %s, please remove the test db file manually", err, testDBFilePath)
	}
}

func count(db *sql.DB, query string) (int, error) {
	var count int
	row := db.QueryRow(query)
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func removeDB() error {
	return os.Remove(testDBFilePath)
}
