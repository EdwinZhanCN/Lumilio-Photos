package main

import (
	"strings"
	"unicode"
)

// sqlAccess is one table a statement touches and how.
type sqlAccess struct {
	Table string
	Op    string // read, insert, update, delete
}

// sqlStatement is one parsed statement: what it reads and writes, and
// whether its result is a single COUNT aggregate.
type sqlStatement struct {
	Accesses []sqlAccess
	Count    bool
	// Trigger is set for CREATE TRIGGER; Accesses are then its body's.
	Trigger string
}

// sqlToken is one lexical token: a lower-cased keyword or identifier, an
// unquoted identifier, or a punctuation character.
type sqlToken struct {
	text   string // lower-cased for words; the identifier for quoted names
	word   bool
	quoted bool
}

// lexSQL splits SQLite SQL into tokens, dropping comments, string literals,
// and blob literals, and unquoting "identifiers", `identifiers`, and
// [identifiers].
func lexSQL(sql string) []sqlToken {
	var tokens []sqlToken
	runes := []rune(sql)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i += 2
		case r == '\'':
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
			tokens = append(tokens, sqlToken{text: "'"})
		case r == '"' || r == '`' || r == '[':
			closer := r
			if r == '[' {
				closer = ']'
			}
			start := i + 1
			i++
			for i < len(runes) && runes[i] != closer {
				i++
			}
			tokens = append(tokens, sqlToken{text: strings.ToLower(string(runes[start:min(i, len(runes))])), word: true, quoted: true})
			i++
		case r == '_' || unicode.IsLetter(r):
			start := i
			for i < len(runes) && (runes[i] == '_' || runes[i] == '$' || unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i])) {
				i++
			}
			tokens = append(tokens, sqlToken{text: strings.ToLower(string(runes[start:i])), word: true})
		case unicode.IsDigit(r):
			for i < len(runes) && (unicode.IsDigit(runes[i]) || unicode.IsLetter(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, sqlToken{text: "0"})
		default:
			tokens = append(tokens, sqlToken{text: string(r)})
			i++
		}
	}
	return tokens
}

// parseSQL parses every statement of a SQL text.
func parseSQL(sql string) []sqlStatement {
	var statements []sqlStatement
	tokens := lexSQL(sql)
	for len(tokens) > 0 {
		end := statementEnd(tokens)
		if statement, ok := parseStatement(tokens[:end]); ok {
			statements = append(statements, statement)
		}
		if end < len(tokens) {
			end++
		}
		tokens = tokens[end:]
	}
	return statements
}

// statementEnd finds the ';' that ends the first statement. A CREATE
// TRIGGER body has ';' inside BEGIN ... END.
func statementEnd(tokens []sqlToken) int {
	depth := 0
	trigger := len(tokens) > 2 && tokens[0].text == "create" && (tokens[1].text == "trigger" || (len(tokens) > 3 && (tokens[1].text == "temp" || tokens[1].text == "temporary") && tokens[2].text == "trigger"))
	for i, token := range tokens {
		switch {
		case token.word && !token.quoted && token.text == "begin" && trigger:
			depth++
		case token.word && !token.quoted && token.text == "end" && trigger && depth > 0:
			depth--
		case token.text == ";" && depth == 0:
			return i
		}
	}
	return len(tokens)
}

func isKeyword(token sqlToken, words ...string) bool {
	if !token.word || token.quoted {
		return false
	}
	for _, word := range words {
		if token.text == word {
			return true
		}
	}
	return false
}

func parseStatement(tokens []sqlToken) (sqlStatement, bool) {
	if len(tokens) == 0 {
		return sqlStatement{}, false
	}
	if isKeyword(tokens[0], "create") {
		for i := 1; i < len(tokens) && i < 4; i++ {
			if isKeyword(tokens[i], "trigger") {
				return parseTrigger(tokens[i+1:])
			}
		}
		return sqlStatement{}, false
	}
	statement := sqlStatement{}
	ctes := map[string]bool{}
	scanAccesses(tokens, ctes, &statement)
	statement.Count = countOnly(tokens)
	return statement, len(statement.Accesses) > 0
}

// parseTrigger records a trigger's name and the accesses of its body.
func parseTrigger(tokens []sqlToken) (sqlStatement, bool) {
	i := 0
	if i+2 < len(tokens) && isKeyword(tokens[i], "if") && isKeyword(tokens[i+1], "not") && isKeyword(tokens[i+2], "exists") {
		i += 3
	}
	if i >= len(tokens) {
		return sqlStatement{}, false
	}
	name := tokens[i].text
	if i+2 < len(tokens) && tokens[i+1].text == "." {
		name = tokens[i+2].text
	}
	for j := i; j < len(tokens); j++ {
		if isKeyword(tokens[j], "begin") {
			statement := sqlStatement{Trigger: name}
			body := tokens[j+1:]
			for len(body) > 0 {
				end := 0
				for end < len(body) && body[end].text != ";" && !isKeyword(body[end], "end") {
					end++
				}
				inner := sqlStatement{}
				scanAccesses(body[:end], map[string]bool{}, &inner)
				statement.Accesses = append(statement.Accesses, inner.Accesses...)
				if end >= len(body) || isKeyword(body[end], "end") {
					break
				}
				body = body[end+1:]
			}
			return statement, true
		}
	}
	return sqlStatement{}, false
}

// scanAccesses walks one statement (and its subqueries) and records every
// table it writes or reads. CTE names are not tables.
func scanAccesses(tokens []sqlToken, ctes map[string]bool, statement *sqlStatement) {
	add := func(table, op string) {
		if table == "" || ctes[table] {
			return
		}
		statement.Accesses = append(statement.Accesses, sqlAccess{Table: table, Op: op})
	}
	// CTE names: WITH [RECURSIVE] name [(cols)] AS (
	for i := 0; i < len(tokens); i++ {
		if !isKeyword(tokens[i], "with") {
			continue
		}
		j := i + 1
		if j < len(tokens) && isKeyword(tokens[j], "recursive") {
			j++
		}
		for j < len(tokens) && tokens[j].word {
			ctes[tokens[j].text] = true
			j++
			if j < len(tokens) && tokens[j].text == "(" {
				j = skipParens(tokens, j)
			}
			if j < len(tokens) && isKeyword(tokens[j], "as") {
				j++
			}
			for j < len(tokens) && isKeyword(tokens[j], "not", "materialized") {
				j++
			}
			if j < len(tokens) && tokens[j].text == "(" {
				j = skipParens(tokens, j)
			}
			if j < len(tokens) && tokens[j].text == "," {
				j++
				continue
			}
			break
		}
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case isKeyword(token, "insert", "replace"):
			// INSERT OR REPLACE: the REPLACE is the conflict clause.
			if i > 0 && isKeyword(tokens[i-1], "or") {
				continue
			}
			j := i + 1
			if j+1 < len(tokens) && isKeyword(tokens[j], "or") {
				j += 2
			}
			if j < len(tokens) && isKeyword(tokens[j], "into") {
				add(tableName(tokens, j+1), "insert")
			}
		case isKeyword(token, "update"):
			// UPDATE [OR x] table SET, but not ON CONFLICT ... DO UPDATE SET.
			if i > 0 && isKeyword(tokens[i-1], "do") {
				continue
			}
			j := i + 1
			if j+1 < len(tokens) && isKeyword(tokens[j], "or") {
				j += 2
			}
			add(tableName(tokens, j), "update")
		case isKeyword(token, "delete"):
			if i+1 < len(tokens) && isKeyword(tokens[i+1], "from") {
				add(tableName(tokens, i+2), "delete")
				i++
			}
		case isKeyword(token, "from", "join"):
			// FROM a [AS] x, b [AS] y: every comma-joined table is read;
			// a table-valued function such as json_each(...) is not a table.
			for j := i + 1; j < len(tokens) && tokens[j].word; {
				next := j + 1
				if next+1 < len(tokens) && tokens[next].text == "." {
					next += 2
				}
				if next < len(tokens) && tokens[next].text == "(" {
					break
				}
				add(tableName(tokens, j), "read")
				if next < len(tokens) && isKeyword(tokens[next], "as") {
					next++
				}
				if next < len(tokens) && tokens[next].word && !isSQLClause(tokens[next]) {
					next++
				}
				if next < len(tokens) && tokens[next].text == "," {
					j = next + 1
					continue
				}
				break
			}
		}
	}
}

// tableName reads [schema.]table at position i.
func tableName(tokens []sqlToken, i int) string {
	if i >= len(tokens) || !tokens[i].word {
		return ""
	}
	name := tokens[i].text
	if i+2 < len(tokens) && tokens[i+1].text == "." && tokens[i+2].word {
		name = tokens[i+2].text
	}
	return name
}

func skipParens(tokens []sqlToken, i int) int {
	depth := 0
	for ; i < len(tokens); i++ {
		switch tokens[i].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return i
}

// countOnly reports whether a SELECT's result list is exactly one COUNT
// aggregate: SELECT [DISTINCT] COUNT(...) [AS name] FROM ...
func countOnly(tokens []sqlToken) bool {
	for i := 0; i < len(tokens); i++ {
		if !isKeyword(tokens[i], "select") {
			continue
		}
		j := i + 1
		if j < len(tokens) && isKeyword(tokens[j], "distinct", "all") {
			j++
		}
		if j+1 >= len(tokens) || !isKeyword(tokens[j], "count") || tokens[j+1].text != "(" {
			return false
		}
		j = skipParens(tokens, j+1)
		if j < len(tokens) && isKeyword(tokens[j], "as") {
			j += 2
		} else if j < len(tokens) && tokens[j].word && !isKeyword(tokens[j], "from") {
			j++
		}
		return j >= len(tokens) || isKeyword(tokens[j], "from")
	}
	return false
}

// isSQLClause reports keywords that can follow a table reference, so they
// are not mistaken for an alias.
func isSQLClause(token sqlToken) bool {
	return isKeyword(token, "where", "group", "order", "limit", "join", "left", "right", "inner", "outer",
		"cross", "natural", "full", "on", "using", "union", "intersect", "except", "having", "window",
		"set", "values", "returning", "indexed", "not", "and", "or", "when", "then", "else", "end",
		"select", "default", "do", "offset")
}
