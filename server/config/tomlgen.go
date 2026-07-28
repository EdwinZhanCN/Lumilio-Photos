package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// manifestSchemaJSON is the generated schema, embedded so that manifests
// written at runtime by `server config init` carry the same per-key prose as
// the checked-in examples. Embedding also makes the schema load-bearing rather
// than a write-only artifact: delete it and the build fails.
//
//go:embed schema/lumilio-server.schema.json
var manifestSchemaJSON []byte

// schemaNode is the slice of JSON Schema the TOML emitter needs: the prose and
// the closed value set for one key, plus its children.
type schemaNode struct {
	Description string                 `json:"description"`
	Enum        []any                  `json:"enum"`
	Properties  map[string]*schemaNode `json:"properties"`
}

func (n *schemaNode) child(key string) *schemaNode {
	if n == nil || n.Properties == nil {
		return nil
	}
	return n.Properties[key]
}

func loadEmbeddedSchema() (*schemaNode, error) {
	root := new(schemaNode)
	if err := json.Unmarshal(manifestSchemaJSON, root); err != nil {
		return nil, fmt.Errorf("decode embedded manifest schema: %w", err)
	}
	return root, nil
}

// EncodeProfile renders one profile as a complete, commented TOML manifest.
//
// schemaRef becomes the file's `#:schema` directive. Checked-in examples pass a
// path relative to the file so the schema resolves offline; manifests written
// to an arbitrary operator-chosen location pass SchemaID instead.
func EncodeProfile(profile Profile, inputs ProfileInputs, schemaRef string) ([]byte, error) {
	raw, err := profile.Build(inputs)
	if err != nil {
		return nil, err
	}
	schema, err := loadEmbeddedSchema()
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	writeProfileHeader(&out, profile, schemaRef)
	if err := writeSection(&out, "", reflect.ValueOf(raw), schema); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeProfileHeader(out *bytes.Buffer, profile Profile, schemaRef string) {
	if schemaRef != "" {
		fmt.Fprintf(out, "#:schema %s\n", schemaRef)
	}
	writeComment(out, "", "Lumilio Photos runtime manifest, schema v"+strconv.Itoa(SchemaVersion)+".")
	writeComment(out, "", profile.Summary)
	writeComment(out, "", "")
	writeComment(out, "", "Scenario: "+profile.Scenario)
	if len(profile.Notes) != 0 {
		writeComment(out, "", "")
		for _, note := range profile.Notes {
			writeComment(out, "", note)
		}
	}
	writeComment(out, "", "")
	writeComment(out, "", "Every key below is required: the loader has no defaults, performs no file")
	writeComment(out, "", "search, and accepts no environment overrides. Whether a manifest is legal")
	writeComment(out, "", "also depends on combinations across keys, which no per-key comment can")
	writeComment(out, "", "state; `server config validate --config <file>` is the authority.")
	writeComment(out, "", "")
	writeComment(out, "", "Generated from server/config/profiles.go by `make config-examples`.")
	writeComment(out, "", "Edit the profile table, not this file.")
	out.WriteByte('\n')
}

// writeSection emits one TOML table: scalars first, then nested tables, so the
// output is valid regardless of how the Go struct orders its fields.
func writeSection(out *bytes.Buffer, path string, value reflect.Value, schema *schemaNode) error {
	structType := value.Type()

	type pending struct {
		path   string
		value  reflect.Value
		schema *schemaNode
	}
	var subTables []pending
	first := true

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		key := strings.Split(field.Tag.Get("toml"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		fieldValue := value.Field(i)
		if fieldValue.IsNil() {
			return fmt.Errorf("manifest field %s%s is nil; profiles must be complete", path, key)
		}
		child := schema.child(key)
		elem := fieldValue.Elem()

		if elem.Kind() == reflect.Struct {
			subTables = append(subTables, pending{path: joinKey(path, key), value: elem, schema: child})
			continue
		}

		literal, err := tomlLiteral(elem)
		if err != nil {
			return fmt.Errorf("%s%s: %w", path, key, err)
		}
		if !first {
			out.WriteByte('\n')
		}
		first = false
		writeKeyDoc(out, child)
		fmt.Fprintf(out, "%s = %s\n", key, literal)
	}

	for _, table := range subTables {
		out.WriteByte('\n')
		if table.schema != nil && table.schema.Description != "" {
			writeComment(out, "", table.schema.Description)
		}
		fmt.Fprintf(out, "[%s]\n", table.path)
		if err := writeSection(out, table.path+".", table.value, table.schema); err != nil {
			return err
		}
	}
	return nil
}

// writeKeyDoc emits the field's prose followed by its closed value set. The
// enum line is generated from the same list runtime validation uses, so a
// comment can never advertise a value the server rejects.
func writeKeyDoc(out *bytes.Buffer, node *schemaNode) {
	if node == nil {
		return
	}
	for _, line := range strings.Split(node.Description, "\n") {
		writeComment(out, "", line)
	}
	if len(node.Enum) != 0 {
		values := make([]string, 0, len(node.Enum))
		for _, item := range node.Enum {
			values = append(values, fmt.Sprint(item))
		}
		writeComment(out, "", "One of: "+strings.Join(values, ", "))
	}
}

func writeComment(out *bytes.Buffer, indent, text string) {
	text = strings.TrimRight(text, " \t")
	if text == "" {
		out.WriteString(indent + "#\n")
		return
	}
	out.WriteString(indent + "# " + text + "\n")
}

func tomlLiteral(value reflect.Value) (string, error) {
	switch value.Kind() {
	case reflect.String:
		return strconv.Quote(value.String()), nil
	case reflect.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), nil
	case reflect.Slice:
		if value.Len() == 0 {
			return "[]", nil
		}
		items := make([]string, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			item, err := tomlLiteral(value.Index(i))
			if err != nil {
				return "", err
			}
			items = append(items, item)
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	default:
		return "", fmt.Errorf("unsupported manifest value kind %s", value.Kind())
	}
}

func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + key
}
