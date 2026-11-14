package problem_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/api/problem"
)

// Every registered Problem type has its generated public page, exact
// OpenAPI HTTP and Reference schemas, and an exhaustive entry in the Web
// Problem catalog, which is written by hand.
func TestEveryProblemTypeHasItsArtifacts(t *testing.T) {
	root := repositoryRoot
	swagger, err := os.ReadFile(filepath.Join(root, "server/docs/swagger.yaml"))
	if err != nil {
		t.Fatalf("read generated OpenAPI Problem contract: %v", err)
	}
	webSource, err := os.ReadFile(filepath.Join(root, "web/src/lib/http-commons/problem.ts"))
	if err != nil {
		t.Fatalf("read Web Problem catalog: %v", err)
	}
	for _, descriptor := range problem.Registered() {
		path := strings.TrimPrefix(descriptor.Type, problem.Namespace)
		docPath := filepath.Join(root, "site/docs/public/problems", filepath.FromSlash(path), "index.html")
		doc, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("Problem type %s has no generated public page at %s: %v", descriptor.Type, docPath, err)
		}
		if !strings.Contains(string(doc), descriptor.Type) || !strings.Contains(string(doc), descriptor.Definition) {
			t.Fatalf("generated Problem page for %s does not match its registry descriptor", descriptor.Type)
		}
		if strings.Count(string(swagger), "const: "+descriptor.Type) < 2 {
			t.Fatalf("generated OpenAPI lacks exact HTTP and Reference schemas for Problem type %s", descriptor.Type)
		}
		knownLiteral := fmt.Sprintf("%q: true", descriptor.Type)
		caseLiteral := fmt.Sprintf("case %q:", descriptor.Type)
		if !strings.Contains(string(webSource), knownLiteral) || !strings.Contains(string(webSource), caseLiteral) {
			t.Fatalf("Web Problem catalog is not exhaustive for generated type %s", descriptor.Type)
		}
	}
}
