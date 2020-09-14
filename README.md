# ttools

ttools is utility packages for writing tests.  
ttools currently has following functionality.  

- incremental unique number ID generator (goroutine safe)
- SQL and processing time tracing for database/sql
- auto DB records cleaner for database/sql

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

delete all inserted records which registed by driver after db.Close called.  
This deletion records hook is emit only once per driver. (not per opend connection)  
This hook ignores `foreign_key_checks`, and resets `auto_increment`.  

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

	// Open `ttools-mysql` driver which includes hooks functionality
	db, err := sql.Open("ttools-mysql", "user:password@protocol(host:port)/dbname")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// delete all inserted records by regstered driver
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
