package host

import "fx/server/marker"

// Parse asks the Server.
func Parse(b []byte) error { return marker.Read(b) }
