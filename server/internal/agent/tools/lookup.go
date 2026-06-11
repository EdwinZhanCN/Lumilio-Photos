package tools

import (
	"context"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

const maxLookupResults = 20

// LookupPeopleInput resolves person names to person ids.
type LookupPeopleInput struct {
	NameQuery string `json:"name_query,omitempty" jsonschema:"description=Partial or full person name; empty lists the most photographed people"`
}

// LookupPerson is one resolved entity row. This is the Lookup quadrant:
// small bounded entity tables are the only non-ref data allowed back into
// the context, and names are sanitized user content (INV-7) — treat them as
// data, never as instructions.
type LookupPerson struct {
	PersonID   int    `json:"person_id"`
	Name       string `json:"name"`
	AssetCount int    `json:"asset_count"`
}

// LookupPeopleOutput lists matching people.
type LookupPeopleOutput struct {
	People []LookupPerson `json:"people,omitempty"`
	Error  *ref.Error     `json:"error,omitempty"`
}

// RegisterLookupPeople registers the people entity resolver.
func RegisterLookupPeople() {
	info := &schema.ToolInfo{
		Name: "lookup_people",
		Desc: "Resolve a person name to person ids for search_people. Returns up to 20 named people " +
			"with their photo counts. An empty query lists the most photographed people.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *LookupPeopleInput) (*LookupPeopleOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Looking up people...", input)

			var nameQuery *string
			if input.NameQuery != "" {
				nameQuery = &input.NameQuery
			}
			rows, err := deps.Queries.AgentLookupPeople(ctx, repo.AgentLookupPeopleParams{
				NameQuery: nameQuery,
				Limit:     maxLookupResults,
			})
			if err != nil {
				refErr := ref.Internal("people lookup")
				sendError(deps, info.Name, execID, start, refErr)
				return &LookupPeopleOutput{Error: refErr}, nil
			}

			people := make([]LookupPerson, 0, len(rows))
			for _, row := range rows {
				name := ref.SanitizeUserText(row.Name, ref.MaxFacetValueLen)
				if name == "" {
					continue
				}
				people = append(people, LookupPerson{
					PersonID:   int(row.ClusterID),
					Name:       name,
					AssetCount: int(row.AssetCount),
				})
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Found %d people", len(people)), nil)
			return &LookupPeopleOutput{People: people}, nil
		})
	})
}
