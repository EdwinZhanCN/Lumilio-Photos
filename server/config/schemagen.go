package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/invopop/jsonschema"
)

// SchemaID is the published identity of the manifest schema. Generated example
// manifests reference it through a `#:schema` directive so a TOML-aware editor
// finds it without any per-project configuration.
const SchemaID = "https://lumilio.org/schema/lumilio-server-v3.schema.json"

// GenerateJSONSchema reflects the manifest struct into a JSON Schema.
//
// The schema is a derived, deliberately lossy artifact. It carries presence,
// types, and the closed value sets, which is what an editor can act on while
// you type. It cannot carry the conditional rules that decide whether a
// manifest is actually legal — that http_listen and hostname are non-empty
// only under tls.mode = acme, that ACME listeners must not collide, and that
// backups_path must fall outside storage.path. Those live in
// resolveManifest, which stays the sole authority, and are documented by the
// per-scenario example manifests rather than by any single key.
//
// Descriptions come from the Go doc comments on the manifest fields, so the
// prose has exactly one home next to the field and its validation.
func GenerateJSONSchema() ([]byte, error) {
	sourceDir, err := packageSourceDir()
	if err != nil {
		return nil, err
	}
	comments, err := manifestDocComments(sourceDir)
	if err != nil {
		return nil, err
	}

	reflector := &jsonschema.Reflector{
		// Every manifest field is a pointer with no omitempty, so invopop marks
		// all of them required — which is exactly the "no code defaults, the
		// file must be complete" rule the loader enforces.
		DoNotReference: true,
		ExpandedStruct: true,
		CommentMap:     comments,
	}

	schema := reflector.Reflect(&manifest{})
	schema.ID = SchemaID
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.Title = "Lumilio Photos runtime manifest (schema v3)"
	schema.Description = "Complete runtime configuration. There are no code defaults, " +
		"no configuration file search, and no environment-variable overrides: every " +
		"key below must be present in the file."

	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest schema: %w", err)
	}
	return append(bytes.TrimRight(encoded, "\n"), '\n'), nil
}

// packageSourceDir locates this package's directory so schema generation reads
// the same source tree it was compiled from.
func packageSourceDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate config package source")
	}
	return filepath.Dir(thisFile), nil
}

// manifestDocComments extracts field doc comments keyed the way invopop looks
// them up: "<pkgpath>.<type>.<field>".
//
// invopop's own AddGoComments walks the same AST but drops unexported types,
// and every manifest struct here is unexported on purpose — the profile table
// is the only supported way to build one. So we collect the comments ourselves
// and hand them over through CommentMap, which has no such restriction.
func manifestDocComments(sourceDir string) (map[string]string, error) {
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, sourceDir, func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse config package source: %w", err)
	}

	comments := make(map[string]string)
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			collectFieldComments(file, comments)
		}
	}
	return comments, nil
}

func collectFieldComments(file *ast.File, into map[string]string) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok || structType.Fields == nil {
				continue
			}
			for _, field := range structType.Fields.List {
				text := strings.TrimSpace(field.Doc.Text())
				if text == "" {
					text = strings.TrimSpace(field.Comment.Text())
				}
				if text == "" {
					continue
				}
				for _, name := range field.Names {
					key := manifestPackagePath + "." + typeSpec.Name.Name + "." + name.Name
					into[key] = text
				}
			}
		}
	}
}

// manifestPackagePath must match reflect.Type.PkgPath for the manifest structs.
const manifestPackagePath = "server/config"
