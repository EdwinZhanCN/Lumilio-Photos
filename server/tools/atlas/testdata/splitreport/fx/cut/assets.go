package cut

// Asset is used by every other file.
type Asset struct{ ID string }

func formatID(id string) string { return "asset:" + id }

// Describe uses the people file, closing a cycle with it.
func (a Asset) Describe() string { return Person{Name: a.ID}.Greeting() }
