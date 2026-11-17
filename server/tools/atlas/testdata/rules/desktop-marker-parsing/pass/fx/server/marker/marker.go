package marker

import "fx/yaml"

// Read parses a marker.
func Read(b []byte) error { return yaml.Unmarshal(b, nil) }
