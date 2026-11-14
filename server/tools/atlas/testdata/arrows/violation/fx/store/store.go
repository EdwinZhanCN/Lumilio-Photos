package store

// Store persists records.
type Store struct{}

// NewStore builds a Store.
func NewStore() *Store { return &Store{} }

// Save writes one record.
func (s *Store) Save(id string) error { return nil }

// Load is never called.
func (s *Store) Load(id string) error { return nil }
