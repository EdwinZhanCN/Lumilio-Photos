package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SQLConfig tells the SQL rules where statements come from. Rules read SQL
// that is parsed, never Go source text: sqlc query files mapped to their
// generated methods, constant SQL handed to an executor method, and the
// triggers of the migrations.
type SQLConfig struct {
	// Queries are sqlc query sets: `-- name: X` in Files becomes method X of
	// Type, and a call to that method performs the statement.
	Queries []SQLQueries `yaml:"queries"`
	// Executors are the types whose Exec/Query/QueryRow/Prepare methods
	// take SQL as their first string argument.
	Executors []string `yaml:"executors"`
	// Triggers are migration files; each CREATE TRIGGER in them is an owner
	// named trigger:<name>.
	Triggers []string `yaml:"triggers"`
}

// SQLQueries is one sqlc query set.
type SQLQueries struct {
	Files []string `yaml:"files"`
	Type  string   `yaml:"type"`
}

var executorMethods = map[string]bool{
	"Exec": true, "ExecContext": true, "Query": true, "QueryContext": true,
	"QueryRow": true, "QueryRowContext": true, "Prepare": true, "PrepareContext": true,
}

var sqlcName = regexp.MustCompile(`(?m)^--\s*name:\s*([A-Za-z0-9_]+)\s+:([a-z]+)`)

// sqlUse is one statement some code performs.
type sqlUse struct {
	owner     string // function key or trigger:<name>
	pkgDir    string
	file      string
	where     string
	statement sqlStatement
}

// sqlFacts are every SQL statement the mapped code performs, plus the
// executor calls whose SQL is not a constant.
type sqlFacts struct {
	uses       []sqlUse
	unresolved []Finding
	// sites counts, per package directory, the executor calls that pass
	// hand-written SQL, constant or not.
	sites map[string]int
	// queries maps a query method key to its statements.
	queries map[string][]sqlStatement
}

func (a *Atlas) sqlFacts() *sqlFacts {
	if a.sql != nil {
		return a.sql
	}
	facts := &sqlFacts{queries: map[string][]sqlStatement{}, sites: map[string]int{}}
	a.sql = facts
	index := a.typed()
	config := a.Config.SQL
	queryPackages := map[string]bool{}
	for _, set := range config.Queries {
		named := index.lookupType(strings.TrimPrefix(set.Type, "*"))
		if named == nil {
			a.problem("atlas.yaml", "sql.queries type %s does not resolve", set.Type)
			continue
		}
		queryPackages[named.(*types.Named).Obj().Pkg().Path()] = true
		for _, pattern := range set.Files {
			files, _ := filepath.Glob(filepath.Join(a.Root, pattern))
			if len(files) == 0 {
				a.problem("atlas.yaml", "sql.queries files %s match nothing", pattern)
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					a.problem("atlas.yaml", "read %s: %v", file, err)
					continue
				}
				text := string(data)
				matches := sqlcName.FindAllStringSubmatchIndex(text, -1)
				for i, match := range matches {
					end := len(text)
					if i+1 < len(matches) {
						end = matches[i+1][0]
					}
					name := text[match[2]:match[3]]
					facts.queries["("+set.Type+")."+name] = parseSQL(text[match[1]:end])
				}
			}
		}
	}
	executors := map[string]bool{}
	executorTypes := map[string]bool{}
	for _, executor := range config.Executors {
		executorTypes[strings.TrimPrefix(executor, "*")] = true
		named := index.lookupType(strings.TrimPrefix(executor, "*"))
		if named == nil {
			a.problem("atlas.yaml", "sql.executors type %s does not resolve", executor)
			continue
		}
		var t types.Type = named
		if strings.HasPrefix(executor, "*") {
			t = types.NewPointer(named)
		}
		methods := types.NewMethodSet(t)
		for i := 0; i < methods.Len(); i++ {
			if fn, ok := methods.At(i).Obj().(*types.Func); ok && executorMethods[fn.Name()] {
				executors[funcKey(fn)] = true
			}
		}
	}
	harvested := map[*funcBody]bool{}
	queryKeys := map[string]bool{}
	for key := range facts.queries {
		queryKeys[key] = true
	}
	for _, fn := range index.funcs {
		info := fn.pkg.typed.info
		inQueryPackage := queryPackages[fn.pkg.importPath]
		// An executor's own methods forward their caller's SQL; the caller
		// is where the statement is resolved.
		forwarding := false
		if fn.obj != nil {
			if named := receiverNamed(fn.obj); named != nil && executorTypes[typeKey(named)] {
				forwarding = true
			}
		}
		for _, ref := range index.references(fn, fn.body) {
			if key, ok := index.matchesTarget(ref.callee, queryKeys); ok && !inQueryPackage {
				for _, statement := range facts.queries[key] {
					facts.uses = append(facts.uses, sqlUse{owner: fn.key, pkgDir: fn.pkg.dir, file: ref.file, where: where(ref.file, ref.pos), statement: statement})
				}
				continue
			}
			if _, ok := index.matchesTarget(ref.callee, executors); !ok || ref.call == nil || inQueryPackage || forwarding {
				continue
			}
			argument := sqlArgument(info, ref.call)
			if argument == nil {
				continue
			}
			facts.sites[fn.pkg.dir]++
			value := info.Types[argument].Value
			if value == nil || value.Kind() != constant.String {
				facts.unresolved = append(facts.unresolved, Finding{
					Key:     fn.key + " executes non-constant SQL",
					Where:   where(ref.file, ref.pos),
					Message: fmt.Sprintf("%s passes SQL that is not a constant to %s; table rules see only the SQL constants written in it", fn.key, funcKey(ref.callee)),
				})
				// The statement is chosen at run time, often from a table of
				// constants in the same function: attribute every SQL constant
				// the function contains to it, so table rules still see them.
				if !harvested[fn] {
					harvested[fn] = true
					for _, statement := range sqlConstants(info, fn.body) {
						facts.uses = append(facts.uses, sqlUse{owner: fn.key, pkgDir: fn.pkg.dir, file: ref.file, where: where(ref.file, ref.pos), statement: statement})
					}
				}
				continue
			}
			for _, statement := range parseSQL(constant.StringVal(value)) {
				facts.uses = append(facts.uses, sqlUse{owner: fn.key, pkgDir: fn.pkg.dir, file: ref.file, where: where(ref.file, ref.pos), statement: statement})
			}
		}
	}
	for _, pattern := range config.Triggers {
		files, _ := filepath.Glob(filepath.Join(a.Root, pattern))
		if len(files) == 0 {
			a.problem("atlas.yaml", "sql.triggers %s matches nothing", pattern)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			rel := filepath.ToSlash(mustRel(a.Root, file))
			for _, statement := range parseSQL(string(data)) {
				if statement.Trigger != "" {
					facts.uses = append(facts.uses, sqlUse{owner: "trigger:" + statement.Trigger, file: rel, where: rel, statement: statement})
				}
			}
		}
	}
	return facts
}

// sqlConstants parses every string constant in a body that begins like a
// SQL statement.
func sqlConstants(info *types.Info, body ast.Node) []sqlStatement {
	var out []sqlStatement
	ast.Inspect(body, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok {
			return true
		}
		value := info.Types[literal].Value
		if value == nil || value.Kind() != constant.String {
			return true
		}
		text := constant.StringVal(value)
		tokens := lexSQL(text)
		if len(tokens) == 0 || !isKeyword(tokens[0], "select", "insert", "update", "delete", "with", "replace") {
			return true
		}
		out = append(out, parseSQL(text)...)
		return true
	})
	return out
}

// sqlArgument returns the first string-typed argument of an executor call,
// which database/sql and its wrappers take as the statement.
func sqlArgument(info *types.Info, call *ast.CallExpr) ast.Expr {
	for _, argument := range call.Args {
		if basic, ok := info.TypeOf(argument).Underlying().(*types.Basic); ok && basic.Info()&types.IsString != 0 {
			return argument
		}
	}
	return nil
}

func (a *Atlas) staticSQLFindings(Rule) []Finding {
	return append([]Finding(nil), a.sqlFacts().unresolved...)
}

func (a *Atlas) tablesFindings(rule Rule) []Finding {
	facts := a.sqlFacts()
	var findings []Finding
	seen := map[string]bool{}
	for _, use := range facts.uses {
		for _, access := range use.statement.Accesses {
			table, ok := rule.Tables[access.Table]
			if !ok {
				continue
			}
			allowed, restricted := table.allowed(access.Op)
			if !restricted || inScope(allowed, use.ownerScope(), use.file) || inScope(allowed, use.owner, "") {
				continue
			}
			key := use.owner + " " + access.Op + " " + access.Table
			if seen[key] {
				continue
			}
			seen[key] = true
			findings = append(findings, Finding{Key: key, Where: use.where, Message: fmt.Sprintf("%s performs %s on %s, which only %s may do", use.owner, access.Op, access.Table, strings.Join(allowed, ", "))})
		}
	}
	// A table is known when a migration creates it or some statement uses
	// it (River creates its own tables); a rule on any other name is a typo.
	known := map[string]bool{}
	for table := range a.sqlTables {
		known[table] = true
	}
	for _, use := range facts.uses {
		for _, access := range use.statement.Accesses {
			known[access.Table] = true
		}
	}
	for _, table := range sortedKeys(rule.Tables) {
		if !known[table] {
			a.ruleProblem(rule, "table %s is neither created by a migration nor used by any statement", table)
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })
	return findings
}

func (use sqlUse) ownerScope() string {
	if use.pkgDir != "" {
		return use.pkgDir
	}
	return use.owner
}

// allowed returns the scopes allowed to perform op and whether op is
// restricted at all.
func (table TableAccess) allowed(op string) ([]string, bool) {
	var specific []string
	switch op {
	case "read":
		return table.Read, table.Read != nil
	case "insert":
		specific = table.Insert
	case "update":
		specific = table.Update
	case "delete":
		specific = table.Delete
	}
	if specific != nil {
		return specific, true
	}
	return table.Write, table.Write != nil
}
