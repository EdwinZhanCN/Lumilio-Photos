package sourcing

// reader is the narrow read port.
type reader interface{ Read() string }

// Materializer holds only a narrow port.
type Materializer struct{ source reader }
