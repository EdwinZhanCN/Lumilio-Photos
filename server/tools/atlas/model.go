package main

// Config is docs/atlas/atlas.yaml: the human-owned half of the architecture
// map. It names groups, the groups each may depend on, the reviewed
// package-level exceptions, and the rules the code must obey. Everything else
// is derived from source.
type Config struct {
	GitHub     string      `yaml:"github"`
	Branch     string      `yaml:"branch"`
	GoModules  []GoModule  `yaml:"goModules"`
	Exclude    []string    `yaml:"exclude"`
	Groups     []Group     `yaml:"groups"`
	Exceptions []Exception `yaml:"exceptions"`
	Rules      []Rule      `yaml:"rules"`
	SQL        SQLConfig   `yaml:"sql"`
	Docs       []string    `yaml:"docs"`
	// UndocumentedRoutes are real routes deliberately absent from OpenAPI,
	// such as infrastructure probes, that documents may still name.
	UndocumentedRoutes []string `yaml:"undocumentedRoutes"`
}

// GoModule is one Go module whose packages belong on the map.
type GoModule struct {
	Dir  string `yaml:"dir"`
	Path string `yaml:"path"`
	Side string `yaml:"side"`
}

// Group is a named architectural slice. Every Go package and Web module
// belongs to exactly one group, the one whose Match paths contain it.
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

// Rule is one architecture rule. ID names it in findings, baselines, and
// fixtures (server/tools/atlas/testdata/rules/<id>); Why names the invariant
// it protects. Exactly one kind field is set; rules.go maps each kind to its
// evaluator.
type Rule struct {
	ID  string `yaml:"id" json:"id"`
	Why string `yaml:"why" json:"why"`

	// GroupUses checks every import between groups against groups[].uses
	// and the reviewed exceptions.
	GroupUses bool `yaml:"group_uses" json:"groupUses,omitempty"`
	// Calls limits who may refer to the target functions and methods.
	Calls *CallsRule `yaml:"calls" json:"calls,omitempty"`
	// Constructs limits who may build values of the target types.
	Constructs *ConstructsRule `yaml:"constructs" json:"constructs,omitempty"`
	// ForbidTypes keeps declarations in scope from holding the types.
	ForbidTypes *ForbidTypesRule `yaml:"forbid_types" json:"forbidTypes,omitempty"`
	// ForbidImports keeps packages or files in scope from importing paths.
	ForbidImports *ForbidImportsRule `yaml:"forbid_imports" json:"forbidImports,omitempty"`
	// StaticSQL requires every SQL statement handed to an executor to be a
	// constant, so the tables rules see every statement.
	StaticSQL bool `yaml:"static_sql" json:"staticSql,omitempty"`
	// Tables limits which code may read or write catalog tables.
	Tables map[string]TableAccess `yaml:"tables" json:"tables,omitempty"`
	// TxPurity forbids slow or unbounded work inside a transaction.
	TxPurity *TxPurityRule `yaml:"tx_purity" json:"txPurity,omitempty"`
	// Budgets caps the production lines of every package.
	Budgets *BudgetsRule `yaml:"budgets" json:"budgets,omitempty"`
}

// A scope is a list of patterns, each a package directory
// (server/internal/db), a directory glob (server/internal/storage/**), or a
// file (server/app/app.go).

// CallsRule: every reference to a target outside Only (or inside Forbid)
// is a finding. A reference through an interface method counts when a
// target implements it.
type CallsRule struct {
	Targets []string `yaml:"targets" json:"targets"`
	Only    []string `yaml:"only" json:"only,omitempty"`
	Forbid  []string `yaml:"forbid" json:"forbid,omitempty"`
}

// ConstructsRule: a composite literal or new() of a target type outside
// Only is a finding.
type ConstructsRule struct {
	Types []string `yaml:"types" json:"types"`
	Only  []string `yaml:"only" json:"only"`
}

// ForbidTypesRule: a field, parameter, result, or variable declared in
// scope whose type mentions a target type is a finding.
type ForbidTypesRule struct {
	In    []string `yaml:"in" json:"in"`
	Types []string `yaml:"types" json:"types"`
}

// ForbidImportsRule: an import of one of Imports (paths or path/**) by a
// file in From is a finding.
type ForbidImportsRule struct {
	From    []string `yaml:"from" json:"from"`
	Imports []string `yaml:"imports" json:"imports"`
}

// TableAccess lists, per operation, the scopes that may perform it. An
// operation that is not listed is unrestricted. write covers insert,
// update, and delete unless one of them is listed itself. A scope entry
// "trigger:<name>" names a trigger in the baseline migration.
type TableAccess struct {
	Read   []string `yaml:"read" json:"read,omitempty"`
	Write  []string `yaml:"write" json:"write,omitempty"`
	Insert []string `yaml:"insert" json:"insert,omitempty"`
	Update []string `yaml:"update" json:"update,omitempty"`
	Delete []string `yaml:"delete" json:"delete,omitempty"`
}

// TxPurityRule: nothing reachable from a transaction body may call into a
// forbidden package or function. Roots are functions with a parameter of
// one of TxTypes and the statements after a Begin call up to its Commit.
type TxPurityRule struct {
	TxTypes        []string `yaml:"tx_types" json:"txTypes"`
	Begin          []string `yaml:"begin" json:"begin"`
	ForbidPackages []string `yaml:"forbid_packages" json:"forbidPackages"`
	ForbidCalls    []string `yaml:"forbid_calls" json:"forbidCalls"`
	// ForbidCountQueries forbids sqlc queries whose result is a single
	// COUNT aggregate, which scan a table range unboundedly.
	ForbidCountQueries bool `yaml:"forbid_count_queries" json:"forbidCountQueries,omitempty"`
	// Allow lists pure functions of a forbidden package (os.IsNotExist)
	// and reviewed calls that stay allowed.
	Allow []string `yaml:"allow" json:"allow,omitempty"`
}

// BudgetsRule: a package whose production lines exceed Default (or its
// recorded ceiling) is a finding. Ceilings live in atlas.lock.json and
// only fall.
type BudgetsRule struct {
	Default int `yaml:"default" json:"default"`
}

// Finding is one violation of a rule. Key identifies it by symbols, never by
// line, so a baseline entry survives unrelated edits; Where is the location
// shown to a reader.
type Finding struct {
	Rule    string `json:"rule"`
	Key     string `json:"key"`
	Where   string `json:"where"`
	Message string `json:"message"`
}

// lockFile is docs/atlas/atlas.lock.json: the verified hash of every view
// anchor, and each rule's baseline of tolerated findings.
type lockFile struct {
	Views     map[string]map[string]string `json:"views"`
	Baselines map[string][]string          `json:"baselines"`
	// Ceilings are the recorded line ceilings of budgets rules, by rule id
	// and package. They only fall.
	Ceilings map[string]map[string]int `json:"ceilings,omitempty"`
}

// View is one authored diagram under docs/atlas/views/<kind>/<id>.yaml.
type View struct {
	ID          string       `yaml:"id" json:"id"`
	Kind        string       `yaml:"kind" json:"kind"`
	Title       string       `yaml:"title" json:"title"`
	Summary     string       `yaml:"summary" json:"summary"`
	Direction   string       `yaml:"direction" json:"direction,omitempty"`
	Layout      string       `yaml:"layout" json:"layout,omitempty"`
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
	Rule    string `json:"rule,omitempty"`
}
