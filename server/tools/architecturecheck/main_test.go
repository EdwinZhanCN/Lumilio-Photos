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
