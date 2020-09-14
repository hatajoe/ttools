package sqlite3

import (
	"database/sql"

	tsql "github.com/hatajoe/ttools/driver/sql"
	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("ttools-sqlite3", tsql.SetHooks(&sqlite3.SQLiteDriver{}))
}
