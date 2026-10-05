package sqlitedriver

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"modernc.org/sqlite"
)

// Engine names the driver every build carries.
const Engine = "modernc.org/sqlite"

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("regexp", 2,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			pattern, ok1 := asString(args[0])
			text, ok2 := asString(args[1])
			if !ok1 || !ok2 {
				return nil, nil
			}
			matched, err := regexp.MatchString(pattern, text)
			if err != nil {
				return nil, err
			}
			return matched, nil
		})
	base := registered()
	sql.Register(Name, dsnTranslator{base})
	sql.Register(LogsDriverName, dsnTranslator{base})
}

// registered is the driver modernc registers as "sqlite": the only one the
// package-level functions (REGEXP above) apply to. A &sqlite.Driver{} built
// here starts empty, and every REGEXP query failed with "no such function".
func registered() driver.Driver {
	db, err := sql.Open("sqlite", "")
	if err != nil {
		panic("sqlitedriver: modernc.org/sqlite is not registered: " + err.Error())
	}
	defer db.Close()
	return db.Driver()
}

func asString(v driver.Value) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case []byte:
		return string(s), true
	}
	return "", false
}

// dsnTranslator opens modernc.org/sqlite with the go-sqlite3 connection
// strings the callers write. The two drivers spell connection options
// differently, and modernc ignores the ones it does not know — so without
// this, "_foreign_keys=on" would quietly open a database without foreign keys.
type dsnTranslator struct{ base driver.Driver }

func (t dsnTranslator) Open(name string) (driver.Conn, error) {
	dsn, err := translateDSN(name)
	if err != nil {
		return nil, err
	}
	return t.base.Open(dsn)
}

// mattnPragmas maps go-sqlite3's DSN options to the PRAGMA each one sets.
var mattnPragmas = map[string]string{
	"_foreign_keys": "foreign_keys",
	"_fk":           "foreign_keys",
	"_journal_mode": "journal_mode",
	"_journal":      "journal_mode",
	"_busy_timeout": "busy_timeout",
	"_timeout":      "busy_timeout",
	"_synchronous":  "synchronous",
	"_sync":         "synchronous",
}

// timeFormat is how a time.Time is written: "sqlite" is the layout
// go-sqlite3 wrote ("2006-01-02 15:04:05.999999999-07:00"), so rows written
// before this driver and after it compare and sort the same as text. Left to
// modernc's default, a new row would read "… +0200 CEST", and every
// `created_at > ?` over mixed rows would be wrong.
const timeFormat = "sqlite"

// busyTimeoutMS is how long a connection waits for a lock another one holds.
// go-sqlite3 waited 5 s by default; modernc waits 0 and fails at once with
// "database is locked (SQLITE_BUSY)" — the gateway writes from several
// goroutines at a time, so without this every overlap would be an error.
const busyTimeoutMS = 5000

// translateDSN rewrites go-sqlite3 options as modernc's _pragma=name(value),
// drops the ones that have no meaning here (_regexp: REGEXP is registered for
// every connection), sets the time format, and passes everything else through.
func translateDSN(name string) (string, error) {
	path, query, _ := strings.Cut(name, "?")
	in, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("sqlite: bad connection options %q: %w", query, err)
	}
	out := url.Values{}
	for key, values := range in {
		switch {
		case key == "_regexp":
		case mattnPragmas[key] != "":
			for _, v := range values {
				out.Add("_pragma", fmt.Sprintf("%s(%s)", mattnPragmas[key], v))
			}
		default:
			out[key] = values
		}
	}
	if !hasPragma(out, "busy_timeout") {
		out.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	}
	if out.Get("_time_format") == "" {
		out.Set("_time_format", timeFormat)
	}
	return path + "?" + out.Encode(), nil
}

// hasPragma reports whether the options already set the named pragma.
func hasPragma(v url.Values, name string) bool {
	for _, p := range v["_pragma"] {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), name+"(") {
			return true
		}
	}
	return false
}
