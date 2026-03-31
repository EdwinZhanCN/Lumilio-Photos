package mocktools

type Definition struct {
	Name               string   `json:"name"`
	Category           string   `json:"category"`
	Description        string   `json:"description"`
	OutputKind         string   `json:"output_kind"`
	Tags               []string `json:"tags,omitempty"`
	FailureModes       []string `json:"failure_modes,omitempty"`
	SuggestedNextTools []string `json:"suggested_next_tools,omitempty"`
}

var catalog = []Definition{
	{
		Name:        "mock_filter_assets",
		Category:    "selection",
		Description: "Filter a synthetic media library using natural-language constraints such as time, camera, rating, liked, or album.",
		OutputKind:  "asset_selection",
		Tags:        []string{"media", "filter", "selection"},
		SuggestedNextTools: []string{
			"mock_group_assets",
			"mock_bulk_like_assets",
			"mock_bulk_archive_assets",
			"mock_create_album",
		},
	},
	{
		Name:        "mock_group_assets",
		Category:    "organization",
		Description: "Group a synthetic asset selection by date, type, location, or album for browsing and curation.",
		OutputKind:  "asset_groups",
		Tags:        []string{"media", "group", "organization"},
		SuggestedNextTools: []string{
			"mock_summarize_selection",
			"mock_add_assets_to_album",
		},
	},
	{
		Name:        "mock_inspect_asset_metadata",
		Category:    "inspection",
		Description: "Inspect synthetic asset metadata such as EXIF, camera model, timestamp, geolocation, and tags.",
		OutputKind:  "asset_metadata_report",
		Tags:        []string{"media", "metadata", "inspection"},
		SuggestedNextTools: []string{
			"mock_filter_assets",
			"mock_find_duplicate_assets",
		},
	},
	{
		Name:        "mock_find_duplicate_assets",
		Category:    "cleanup",
		Description: "Find synthetic duplicate or near-duplicate assets and propose keep/archive actions.",
		OutputKind:  "duplicate_report",
		Tags:        []string{"media", "duplicates", "cleanup"},
		FailureModes: []string{
			"false_positive_duplicate",
			"insufficient_similarity_threshold",
		},
		SuggestedNextTools: []string{
			"mock_inspect_asset_metadata",
			"mock_bulk_archive_assets",
		},
	},
	{
		Name:        "mock_bulk_like_assets",
		Category:    "mutation",
		Description: "Apply a synthetic bulk like or unlike action to the currently selected assets.",
		OutputKind:  "bulk_like_update",
		Tags:        []string{"media", "like", "bulk-action"},
		FailureModes: []string{
			"selection_empty",
			"permission_denied",
		},
		SuggestedNextTools: []string{
			"mock_summarize_selection",
		},
	},
	{
		Name:        "mock_bulk_archive_assets",
		Category:    "mutation",
		Description: "Apply a synthetic archive action to selected assets, commonly after duplicate review or curation.",
		OutputKind:  "bulk_archive_update",
		Tags:        []string{"media", "archive", "bulk-action"},
		FailureModes: []string{
			"selection_empty",
			"archive_conflict",
		},
		SuggestedNextTools: []string{
			"mock_summarize_selection",
		},
	},
	{
		Name:        "mock_create_album",
		Category:    "organization",
		Description: "Create a synthetic album for a selected theme, trip, or event.",
		OutputKind:  "album_record",
		Tags:        []string{"media", "album", "organization"},
		FailureModes: []string{
			"duplicate_album_name",
		},
		SuggestedNextTools: []string{
			"mock_add_assets_to_album",
			"mock_summarize_selection",
		},
	},
	{
		Name:        "mock_add_assets_to_album",
		Category:    "organization",
		Description: "Add a synthetic asset selection into an existing or newly created album.",
		OutputKind:  "album_membership_update",
		Tags:        []string{"media", "album", "membership"},
		FailureModes: []string{
			"album_not_found",
			"empty_selection",
		},
		SuggestedNextTools: []string{
			"mock_summarize_selection",
		},
	},
	{
		Name:        "mock_summarize_selection",
		Category:    "summary",
		Description: "Summarize a synthetic selection or action result for the user, highlighting counts, time ranges, and key metadata.",
		OutputKind:  "selection_summary",
		Tags:        []string{"media", "summary", "results"},
	},
}

func Catalog() []Definition {
	out := make([]Definition, len(catalog))
	copy(out, catalog)
	return out
}
