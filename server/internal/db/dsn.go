package db

import (
	"sort"
	"strings"

	"server/config"
)

// isSocketHost reports whether host is a Unix-socket *directory* path rather
// than a TCP hostname. PostgreSQL uses a socket directory when the host begins
// with "/" (the desktop runtime points here); TCP deployments (docker/web) use
// a hostname like "db" or an IP, which never starts with "/".
func isSocketHost(host string) bool {
	return strings.HasPrefix(host, "/")
}

// socketDSN builds a libpq keyword/value connection string for a Unix-socket
// directory host. A URL-form DSN (postgres://user:pw@host:port/db) cannot carry
// a filesystem path — with slashes and spaces, e.g.
// "/Users/me/Library/Application Support/Lumilio Photos/postgres/17/run" — in
// the host position, so the keyword/value form is used instead. pgx accepts it
// via pgxpool.ParseConfig, the pgx stdlib sql driver, and pgxpool.New, so it
// works for the connection pool and for both migration paths.
func socketDSN(cfg config.DatabaseConfig, extra map[string]string) string {
	parts := []string{
		kv("host", cfg.Host),
		kv("port", cfg.Port),
		kv("user", cfg.User),
		kv("password", cfg.Password),
		kv("dbname", cfg.DBName),
		kv("sslmode", cfg.SSL),
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, kv(k, extra[k]))
	}
	return strings.Join(parts, " ")
}

func kv(key, val string) string {
	return key + "=" + quoteDSNValue(val)
}

// quoteDSNValue quotes a libpq connection value: an empty value or one
// containing a space, single quote, or backslash is wrapped in single quotes
// with those characters backslash-escaped (per the PostgreSQL connection-string
// rules). Other values are returned unchanged.
func quoteDSNValue(v string) string {
	if v == "" {
		return "''"
	}
	if strings.ContainsAny(v, " '\\") {
		r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
		return "'" + r.Replace(v) + "'"
	}
	return v
}
