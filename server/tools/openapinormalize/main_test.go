package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"server/internal/api/problem"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func loadGeneratedSpec(t *testing.T) *yaml.Node {
	t.Helper()
	data, err := os.ReadFile("../../docs/swagger.yaml")
	require.NoError(t, err)
	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))
	return document.Content[0]
}

func TestGeneratedAPIErrorsUseOnlyProblemJSON(t *testing.T) {
	root := loadGeneratedSpec(t)
	paths := mappingValue(root, "paths")
	checked := 0
	for pathIndex := 0; pathIndex+1 < len(paths.Content); pathIndex += 2 {
		path := paths.Content[pathIndex].Value
		if !strings.HasPrefix(path, "/api/v1") {
			continue
		}
		pathItem := paths.Content[pathIndex+1]
		for operationIndex := 0; operationIndex+1 < len(pathItem.Content); operationIndex += 2 {
			if !isHTTPMethod(pathItem.Content[operationIndex].Value) {
				continue
			}
			responses := mappingValueOrNil(pathItem.Content[operationIndex+1], "responses")
			if responses == nil {
				continue
			}
			for responseIndex := 0; responseIndex+1 < len(responses.Content); responseIndex += 2 {
				status, err := strconv.Atoi(responses.Content[responseIndex].Value)
				if err != nil || status < 400 || status > 599 {
					continue
				}
				content := mappingValue(responses.Content[responseIndex+1], "content")
				require.Lenf(t, content.Content, 2, "%s response %d has multiple media types", path, status)
				require.Equal(t, problem.MediaType, content.Content[0].Value)
				ref := mappingValue(mappingValue(content.Content[1], "schema"), "$ref").Value
				require.Contains(t, []string{
					"#/components/schemas/api.ProblemResponse",
					"#/components/schemas/api.RateLimitedProblemResponse",
					"#/components/schemas/api.RepositoryConflictProblemResponse",
				}, ref)
				checked++
			}
		}
	}
	require.Greater(t, checked, 0)
}

func TestGeneratedProblemUnionsAreExactAndCoverRegistry(t *testing.T) {
	root := loadGeneratedSpec(t)
	schemas := mappingValue(mappingValue(root, "components"), "schemas")
	httpUnion := mappingValue(schemas, "api.ProblemResponse")
	referenceUnion := mappingValue(schemas, "api.ProblemReference")
	require.Len(t, mappingValue(httpUnion, "oneOf").Content, len(problem.Registered())+1)
	require.Len(t, mappingValue(referenceUnion, "oneOf").Content, len(problem.Registered()))

	for _, descriptor := range problem.Registered() {
		requireProblemSchema(t, schemas, "api."+schemaName(descriptor.Type)+"Problem", descriptor.Type, true)
		requireProblemSchema(t, schemas, "api."+schemaName(descriptor.Type)+"ProblemReference", descriptor.Type, false)
	}
	requireProblemSchema(t, schemas, "api.AboutBlankProblem", "about:blank", true)
}

func requireProblemSchema(t *testing.T, schemas *yaml.Node, name, problemType string, httpProblem bool) {
	t.Helper()
	schema := mappingValue(schemas, name)
	require.Equal(t, "false", mappingValue(schema, "additionalProperties").Value)
	properties := mappingValue(schema, "properties")
	require.Equal(t, problemType, mappingValue(mappingValue(properties, "type"), "const").Value)
	for _, forbidden := range []string{"title", "detail", "code", "error_code", "message", "error"} {
		require.Nilf(t, mappingValueOrNil(properties, forbidden), "%s contains forbidden member %s", name, forbidden)
	}
	if httpProblem {
		require.NotNil(t, mappingValueOrNil(properties, "status"))
	} else {
		require.Nil(t, mappingValueOrNil(properties, "status"))
	}
}
