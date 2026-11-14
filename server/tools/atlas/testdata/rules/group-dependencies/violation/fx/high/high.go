package high

import "fx/low"

// Read returns the lower layer's value.
func Read() int { return low.Value }
