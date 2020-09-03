package sql

import (
	"context"
	"testing"

	"github.com/hatajoe/ttools/gen"
	"github.com/mattn/go-sqlite3"
)

func Test_Open(t *testing.T) {

	SQLTracing(true)

	ctx := context.Background()

	db, err := Open(&sqlite3.SQLiteDriver{}, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err = db.ExecContext(ctx, `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO t1(id) 
		VALUES(?)`, gen.ID()); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	row := db.QueryRow(`SELECT count(*) FROM t1`)
	if err := row.Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 10 {
		t.Errorf("table records count is expected to be 20, but %d count detected", count)
	}
}
