# ttools

ttools is utility packages for writing tests.
ttools currently has following functionality.

- incremental unique number ID generator (goroutine safe)
- SQL and processing time tracing for database/sql
- auto DB setup and cleanup

# Usage

## unique number ID generator

generate unique number ID which can be called by multiple goroutine in safe.

```go
package some_test

import (
	"sync"
	"testing"

	"github.com/go-sql-driver/gen"
)

func Test(t *testing.T) {
	expected := 50000

	wg := &sync.WaitGroup{}
	for i := 0; i < expected-1; i++ {
		wg.Add(1)
		go func () {
			gen.ID()
			wg.Done()
		}()
	}
	wg.Wait()

	if id := ID(); int64(expected) != id {
		t.Errorf("ID returns %d was expected, but %d was returned", expected, id)
	}
}
```

## tracing and auto DB records cleaner

Create database before sql.Open called, and drop after db.Close called.

```go
package some_test

import (
	"database/sql"
	"testing"

	"github.com/hatajoe/ttools/driver/sql/mysql"
	tsql "github.com/hatajoe/ttools/driver/sql"
)

func Test(t *testing.T) {
	// if pass the true for sql.Tracing, show executed SQL and processing time to the console
	tsql.Tracing(true)
	// set database name used in while testing
	// this is created after sql.Open, and is droped before db.Close (default database name is `ttools`)
	tsql.TestDatabase("test")

	// Open `ttools-mysql` driver
	// dbname will be ignored, ttools uses the database specified at tsql.DatabaseName
	db, err := sql.Open("ttools-mysql", "user:password@protocol(host:port)/dbname")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// drop database before closing the connection
		if err := db.Close(); err != nil {
			t.Fatal()
		}
	}()
}
```

currently supporting drivers following:

- sqlite3: `ttools-sqlite3`
- mysql: `ttools-mysql`

# LICENCE

MIT
