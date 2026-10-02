package main

// Config is docs/atlas/atlas.yaml: the human-owned half of the architecture
// map. It names groups, the groups each may depend on, and the reviewed
// package-level exceptions. Everything else is derived from source.
type Config struct {
	GitHub     string      `yaml:"github"`
	Branch     string      `yaml:"branch"`
	GoModules  []GoModule  `yaml:"goModules"`
	Exclude    []string    `yaml:"exclude"`
	Groups     []Group     `yaml:"groups"`
	Exceptions []Exception `yaml:"exceptions"`
}

// GoModule is one Go module whose packages belong on the map.
type GoModule struct {
	Dir  string `yaml:"dir"`
	Path string `yaml:"path"`
	Side string `yaml:"side"`
}

// Group is a named architectural slice. Packages opt in with an
// //atlas:group directive in doc.go; Web modules are assigned by Match.
type Group struct {
	ID      string   `yaml:"id" json:"id"`
	Title   string   `yaml:"title" json:"title"`
	Side    string   `yaml:"side" json:"side"`
	Summary string   `yaml:"summary" json:"summary"`
	Uses    []string `yaml:"uses" json:"uses"`
	Match   []string `yaml:"match" json:"match,omitempty"`
}

// Exception is a reviewed package-level edge that crosses an undeclared group
// boundary. Reason is mandatory so the debt stays explained.
type Exception struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Reason string `yaml:"reason"`
}

// View is one authored diagram under docs/atlas/views/<kind>/<id>.yaml.
type View struct {
	ID          string       `yaml:"id" json:"id"`
	Kind        string       `yaml:"kind" json:"kind"`
	Title       string       `yaml:"title" json:"title"`
	Summary     string       `yaml:"summary" json:"summary"`
	Direction   string       `yaml:"direction" json:"direction,omitempty"`
	Enum        string       `yaml:"enum" json:"enum,omitempty"`
	Nodes       []Node       `yaml:"nodes" json:"nodes"`
	Edges       []Edge       `yaml:"edges" json:"edges,omitempty"`
	Steps       []Step       `yaml:"steps" json:"-"`
	Transitions []Transition `yaml:"transitions" json:"transitions,omitempty"`
	Notes       []Note       `yaml:"notes" json:"notes,omitempty"`
	Related     []string     `yaml:"related" json:"related,omitempty"`

	Source   string `yaml:"-" json:"source"`
	Derived  bool   `yaml:"-" json:"derived"`
	Mermaid  string `yaml:"-" json:"mermaid"`
	Category string `yaml:"-" json:"category"`
}

// Node is a participant, process, store, state, or module in a view.
type Node struct {
	ID     string `yaml:"id" json:"id"`
	Label  string `yaml:"label" json:"label"`
	Anchor string `yaml:"anchor" json:"anchor,omitempty"`
	Shape  string `yaml:"shape" json:"shape,omitempty"`
	Group  string `yaml:"group" json:"group,omitempty"`
	Note   string `yaml:"note" json:"note,omitempty"`
	Member string `yaml:"member" json:"member,omitempty"`
}

// Edge connects two nodes in an architecture or data-flow view.
type Edge struct {
	From   string `yaml:"from" json:"from"`
	To     string `yaml:"to" json:"to"`
	Label  string `yaml:"label" json:"label,omitempty"`
	Anchor string `yaml:"anchor" json:"anchor,omitempty"`
	Style  string `yaml:"style" json:"style,omitempty"`
}

// Step is one sequence-diagram message, note, or control block.
type Step struct {
	From   string  `yaml:"from"`
	To     string  `yaml:"to"`
	Label  string  `yaml:"label"`
	Anchor string  `yaml:"anchor"`
	Reply  bool    `yaml:"reply"`
	Async  bool    `yaml:"async"`
	Note   string  `yaml:"note"`
	Over   string  `yaml:"over"`
	Block  string  `yaml:"block"`
	Steps  []Step  `yaml:"steps"`
	Else   []Block `yaml:"else"`
}

// Block is an else branch of an alt block.
type Block struct {
	Label string `yaml:"label"`
	Steps []Step `yaml:"steps"`
}

// Transition is one lifecycle edge between states.
type Transition struct {
	From   string `yaml:"from" json:"from"`
	To     string `yaml:"to" json:"to"`
	Label  string `yaml:"label" json:"label,omitempty"`
	Anchor string `yaml:"anchor" json:"anchor,omitempty"`
}

// Note is a titled markdown paragraph rendered under the diagram.
type Note struct {
	Title string `yaml:"title" json:"title"`
	Body  string `yaml:"body" json:"body"`
}

// Module is one Go package or Web module on the map.
type Module struct {
	ID         string   `json:"id"`
	Side       string   `json:"side"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	ImportPath string   `json:"importPath,omitempty"`
	Group      string   `json:"group"`
	Synopsis   string   `json:"synopsis"`
	DocHTML    string   `json:"docHtml"`
	DocFile    string   `json:"docFile,omitempty"`
	Files      int      `json:"files"`
	Lines      int      `json:"lines"`
	Imports    []string `json:"imports"`
	ImportedBy []string `json:"importedBy"`
}

// Symbol is one anchorable declaration.
type Symbol struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Module   string   `json:"module"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Kind     string   `json:"kind"`
	Hash     string   `json:"-"`
	Exported bool     `json:"-"`
	Members  []string `json:"-"`
}

// ResolvedAnchor is an anchor after resolution, as shown in the site.
type ResolvedAnchor struct {
	Anchor string `json:"anchor"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Module string `json:"module,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Hash   string `json:"-"`
	Stale  bool   `json:"stale,omitempty"`

	symbol *Symbol
}

// Problem is one failed Atlas rule.
type Problem struct {
	Where   string `json:"where"`
	Message string `json:"message"`
}
