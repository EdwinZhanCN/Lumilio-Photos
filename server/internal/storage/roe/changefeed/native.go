package changefeed

type sequencedEvent struct {
	sequence uint64
	event    Event
}

// NewNative selects the host-native change source. Unsupported builds return
// the explicit periodic fallback rather than pretending to own a valid cursor.
func NewNative() Feed { return newNative() }
