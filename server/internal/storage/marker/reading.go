package marker

import (
	"errors"
	"io/fs"
)

type State string

const (
	Valid              State = "valid"
	Absent             State = "absent"
	PermissionDenied   State = "permission_denied"
	Unknown            State = "unknown"
	Corrupt            State = "corrupt"
	UnsupportedVersion State = "unsupported_version"
)

// Reader is structurally incapable of creating probes or changing markers.
type Reader interface{ ReadFile(string) ([]byte, error) }
type Reading[T any] struct {
	State   State
	Version string
	UUID    string
	Config  *T
	Err     error
}

func Read[T any](reader Reader, path string, decode func([]byte) Reading[T]) Reading[T] {
	data, err := reader.ReadFile(path)
	if err != nil {
		state := Unknown
		switch {
		case errors.Is(err, fs.ErrNotExist):
			state = Absent
		case errors.Is(err, fs.ErrPermission):
			state = PermissionDenied
		}
		return Reading[T]{State: state, Err: err}
	}
	return decode(data)
}
