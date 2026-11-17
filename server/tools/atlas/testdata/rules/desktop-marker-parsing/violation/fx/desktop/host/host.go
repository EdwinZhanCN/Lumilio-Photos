package host

import "fx/yaml"

// Parse reads a marker itself.
func Parse(b []byte) error { return yaml.Unmarshal(b, nil) }
