package service

import "fx/engine"

// Driver leaks the engine above the catalog.
const Driver = engine.Name
