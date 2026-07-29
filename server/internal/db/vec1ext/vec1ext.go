// Package vec1ext statically registers the vendored SQLite Vec1 extension.
package vec1ext

/*
#cgo CFLAGS: -O3 -DNDEBUG
#cgo linux LDFLAGS: -lm

int sqlite3_vec1_extra_init(const char *);
*/
import "C"

// Auto registers Vec1 for all SQLite connections opened after this call.
// Repeated registration is harmless because sqlite3_auto_extension de-duplicates
// identical entry points.
func Auto() {
	C.sqlite3_vec1_extra_init(nil)
}
