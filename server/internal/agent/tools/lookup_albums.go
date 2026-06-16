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

// LookupAlbumsInput resolves album titles to album ids.
type LookupAlbumsInput struct {
	TitleQuery string `json:"title_query,omitempty" jsonschema:"description=Partial or full album title; empty lists the largest albums"`
}

// LookupAlbum is one resolved album row.
type LookupAlbum struct {
	AlbumID    int    `json:"album_id"`
	Title      string `json:"title"`
	AssetCount int    `json:"asset_count"`
}

// LookupAlbumsOutput lists matching albums.
type LookupAlbumsOutput struct {
	Albums []LookupAlbum `json:"albums,omitempty"`
	Error  *ref.Error    `json:"error,omitempty"`
}

// RegisterLookupAlbums registers the album entity resolver.
func RegisterLookupAlbums() {
	info := &schema.ToolInfo{
		Name: "lookup_albums",
		Desc: "Resolve an album title to album ids for filter_assets(album_id) or add_to_album. " +
			"Returns up to 20 albums with their photo counts.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *LookupAlbumsInput) (*LookupAlbumsOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Looking up albums...", input)

			var titleQuery *string
			if input.TitleQuery != "" {
				titleQuery = &input.TitleQuery
			}
			rows, err := deps.Queries.AgentLookupAlbums(ctx, repo.AgentLookupAlbumsParams{
				UserID:     deps.UserID,
				TitleQuery: titleQuery,
				Limit:      maxLookupResults,
			})
			if err != nil {
				refErr := ref.Internal("album lookup")
				sendError(deps, info.Name, execID, start, refErr)
				return &LookupAlbumsOutput{Error: refErr}, nil
			}

			albums := make([]LookupAlbum, 0, len(rows))
			for _, row := range rows {
				title := ref.SanitizeUserText(row.Title, ref.MaxFacetValueLen)
				if title == "" {
					continue
				}
				albums = append(albums, LookupAlbum{
					AlbumID:    int(row.AlbumID),
					Title:      title,
					AssetCount: int(row.AssetCount),
				})
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Found %d albums", len(albums)), nil)
			return &LookupAlbumsOutput{Albums: albums}, nil
		})
	})
}
