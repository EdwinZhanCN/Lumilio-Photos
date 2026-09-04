package main

import "testing"

func TestRepositoryTerminologyTechnicalContextAllowlistIsNarrow(t *testing.T) {
	for _, line := range []string{
		"Store state in ~/Library/Application Support/Lumilio.",
		"Clone the source repository before building.",
		"复制代码仓库中的 compose.yml。",
	} {
		if !allowedRepositoryTermContext("site/docs/en/example.md", line) {
			t.Fatalf("technical context rejected: %q", line)
		}
	}
	for _, line := range []string{
		"Open your Library",
		"The storage root is offline",
		"The Repository Root is offline",
		"打开图库",
		"资源库根不可用",
	} {
		if allowedRepositoryTermContext("site/docs/en/example.md", line) {
			t.Fatalf("product synonym unexpectedly allowed: %q", line)
		}
	}
}

func TestDeveloperMarkdownCannotBypassRepositoryTerminology(t *testing.T) {
	for _, line := range []string{"# Library", "打开图库"} {
		if !userFacingTerminologyViolation("site/docs/zh-cn/user-manual/developer/example.md", line) {
			t.Fatalf("developer Markdown bypassed product terminology: %q", line)
		}
	}
	for _, line := range []string{
		"Store state in ~/Library/Application Support/Lumilio.",
		"Clone the source repository before building.",
	} {
		if !allowedRepositoryTermContext("site/docs/en/user-manual/developer/example.md", line) {
			t.Fatalf("legitimate technical phrase rejected: %q", line)
		}
	}
}

func TestMarkdownCapabilityLabelGateDistinguishesFeatureProse(t *testing.T) {
	for _, line := range []string{
		"| Semantic Search | enabled |",
		"- **Face Recognition**: enabled",
		"- OCR",
		"| 物种识别 | 可用 |",
	} {
		if !retiredMarkdownCapabilityLabel("site/docs/en/example.md", line) {
			t.Fatalf("retired capability label accepted: %q", line)
		}
	}
	if !userFacingTerminologyViolation("site/docs/en/user-manual/use/semantic-search.md", "# Semantic Search") {
		t.Fatal("semantic-search feature filename bypassed a retired capability label")
	}
	if !userFacingTerminologyViolation("site/docs/en/user-manual/use/ocr-search.md", "- OCR") {
		t.Fatal("ocr-search feature filename bypassed a retired capability label")
	}
	for _, line := range []string{
		"Image Semantic Analysis enables semantic search.",
		"| OCR Text Recognition | enabled |",
		"BioCLIP Species Recognition identifies plants.",
	} {
		if retiredMarkdownCapabilityLabel("site/docs/en/example.md", line) {
			t.Fatalf("canonical label or feature prose rejected: %q", line)
		}
	}
}

func TestExecutionCouplingArchitecturePredicates(t *testing.T) {
	// 1. Resource literal scatter
	if !isResourceLiteralScatter("server/app/pipeline_runtime.go", "err := engine.Run(ctx, execution.Resources{CPU: 1}, work)") {
		t.Fatal("planted execution.Resources literal in pipeline_runtime was not detected")
	}
	if isResourceLiteralScatter("server/internal/execution/demand.go", "Resources{CPU: 1, DiskIO: 1}") {
		t.Fatal("Resources literal in demand.go unexpectedly flagged")
	}
	if isResourceLiteralScatter("server/app/pipeline_runtime_test.go", "res := execution.Resources{CPU: 1}") {
		t.Fatal("Resources literal in test file unexpectedly flagged")
	}

	// 2. Governor construction sites
	if !isGovernorConstructionViolation("server/app/app.go", "gov, err := execution.NewGovernor(capacity, 256)") {
		t.Fatal("planted execution.NewGovernor in app.go was not detected")
	}
	if isGovernorConstructionViolation("server/internal/execution/budget.go", "return execution.NewGovernor(capacity, maxWaiting)") {
		t.Fatal("NewGovernor call in budget.go unexpectedly flagged")
	}
	if isGovernorConstructionViolation("server/app/app_test.go", "gov, _ := execution.NewGovernor(capacity, 16)") {
		t.Fatal("NewGovernor call in test file unexpectedly flagged")
	}

	// 3. Processor naked flags
	if !isProcessorNakedFFmpegFlagViolation("server/internal/processors/video_helpers.go", `"-threads", "0"`) {
		t.Fatal("naked -threads flag in processors was not detected")
	}
	if !isProcessorNakedFFmpegFlagViolation("server/internal/processors/video_helpers.go", `"-preset", "medium"`) {
		t.Fatal("naked -preset flag in processors was not detected")
	}
	if isProcessorNakedFFmpegFlagViolation("server/internal/processors/video_helpers_test.go", `"-threads", "0"`) {
		t.Fatal("flag in processors test file unexpectedly flagged")
	}
	if isProcessorNakedFFmpegFlagViolation("server/internal/other/something.go", `"-threads", "0"`) {
		t.Fatal("flag in non-processors file unexpectedly flagged")
	}
}
