package cut

// Person names an asset's subject.
type Person struct{ Name string }

// Greeting uses an unexported helper of the assets file.
func (p Person) Greeting() string { return formatID(p.Name) }
