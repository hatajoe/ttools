package mysql

import (
	"database/sql"

	"github.com/go-sql-driver/mysql"
	tsql "github.com/hatajoe/ttools/driver/sql"
)

func init() {
	sql.Register("ttools-mysql", tsql.SetHooks(&mysql.MySQLDriver{}))
}
