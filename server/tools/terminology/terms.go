package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	retiredRepositoryTerms  = regexp.MustCompile(`(?i)\b(?:storage|repository) root\b|\blibrary\b|图库|媒体库|存储根目录|存储根|资源库根|仓库`)
	retiredCapabilityLabels = regexp.MustCompile(`["'` + "`" + `](Semantic Search|Face Recognition|OCR|Species Recognition|语义搜索|人脸识别|物种识别)["'` + "`" + `]`)
)

func checkUserFacingTerminology(root string) error {
	for _, locale := range []string{"en", "zh"} {
		catalog, err := readJSONCatalog(filepath.Join(root, "web/src/locales", locale, "translation.json"))
		if err != nil {
			return err
		}
		for _, term := range []struct{ key, en, zh string }{
			{"assets.lifecycle.missing", "Missing", "缺失"},
			{"assets.trash.title", "Trash", "回收站"},
			{"assets.lifecycle.deletePermanently", "Delete permanently", "永久删除"},
			{"assets.lifecycle.removeMissing", "Remove missing items", "移除缺失项"},
		} {
			var value any = catalog
			for _, part := range strings.Split(term.key, ".") {
				object, ok := value.(map[string]any)
				if !ok {
					value = nil
					break
				}
				value = object[part]
			}
			expected := term.en
			if locale == "zh" {
				expected = term.zh
			}
			if value != expected {
				return fmt.Errorf("canonical lifecycle term %s (%s): got %v, want %s", term.key, locale, value, expected)
			}
		}
	}

	paths := []string{
		"README.md", "README.en.md",
		"web/src", "desktop/internal/locales", "desktop/internal/panel", "desktop/internal/host",
		"site/docs/en", "site/docs/zh-cn",
		"server/internal/api/dto", "server/internal/api/handler", "server/docs",
	}
	violations, err := scanTextPaths(root, paths, func(relative, line string) bool {
		return userFacingTerminologyViolation(relative, line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"User-facing terminology check found retired Repository/Storage Location or Lumen capability labels:\n%s\nUse Repository/资源库, Storage Location/存储位置, and the canonical capability labels from AGENTS.md.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func readJSONCatalog(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read translation catalog %s: %w", path, err)
	}
	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse translation catalog %s: %w", path, err)
	}
	return catalog, nil
}

func userFacingTerminologyViolation(relative, line string) bool {
	if strings.Contains(relative, ".test.") || strings.HasSuffix(relative, "_test.go") {
		return false
	}
	if retiredRepositoryTerms.MatchString(line) && !allowedRepositoryTermContext(relative, line) {
		return true
	}
	if filepath.Ext(relative) != ".md" && retiredCapabilityLabels.MatchString(withoutCanonicalCapabilityLabels(line)) {
		return true
	}
	return filepath.Ext(relative) == ".md" && retiredMarkdownCapabilityLabel(relative, line)
}

// allowedRepositoryTermContext keeps platform and source-code vocabulary from
// being confused with the Repository product entity. These are deliberately
// narrow: adding another exception requires naming the technical context.
func allowedRepositoryTermContext(relative, line string) bool {
	lower := strings.ToLower(line)
	trimmed := strings.TrimSpace(line)
	// The Music browse label is scoped to the listening domain. The Web
	// terminology test additionally verifies its exact translation key.
	if strings.HasPrefix(relative, "web/src/features/music/") && strings.Contains(line, `t("music.browse.title", "Library")`) {
		return true
	}
	if relative == "web/src/locales/en/translation.json" && strings.TrimSuffix(trimmed, ",") == `"title": "Library"` {
		return true
	}

	if (strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, ".ts") || strings.HasSuffix(relative, ".tsx")) &&
		(strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")) &&
		!strings.HasPrefix(trimmed, "// @") && !strings.Contains(relative, "schema.d.ts") &&
		!strings.HasSuffix(relative, "/doc.ts") {
		return true
	}
	if strings.HasSuffix(relative, ".go") && !strings.Contains(line, `"`) && !strings.HasPrefix(trimmed, "// @") {
		return true
	}
	if strings.Contains(lower, "/flows/library/") {
		return true
	}
	for _, allowed := range []string{
		"~/library/", "/library/application support", "library/application support",
		"library.sqlite", "library-manifest", "library manifest", "library identity",
		"source repository", "code repository", "project repository", "git repository",
		"repository source", "repository checkout", "repository convention", "repository map",
		"component library", "icon library", "media library api", "photo library api",
		"代码仓库", "源码仓库", "项目仓库", "git 仓库", "仓库根目录", "跨仓库",
		"复制仓库中的", "公开仓库", "仓库声明的", "主仓库通过", "仓库约定",
		"仓库与组件地图", "仓库中的 `", "相邻仓库", "上游仓库", "仓库权威指南",
	} {
		if strings.Contains(lower, allowed) {
			return true
		}
	}
	return false
}

func retiredMarkdownCapabilityLabel(relative, line string) bool {
	trimmed := strings.TrimSpace(line)
	retiredText := withoutCanonicalCapabilityLabels(trimmed)
	labels := []string{
		"Semantic Search", "Face Recognition", "OCR", "Species Recognition",
		"语义搜索", "人脸识别", "物种识别",
	}
	for _, label := range labels {
		if strings.EqualFold(strings.Trim(retiredText, "#*|-：: `\t"), label) {
			return true
		}
		if strings.Contains(retiredText, "| "+label+" |") || strings.Contains(retiredText, "**"+label+"**") {
			return true
		}
	}
	return false
}

func withoutCanonicalCapabilityLabels(line string) string {
	for _, label := range []string{
		"Image Semantic Analysis", "Person Recognition", "OCR Text Recognition", "BioCLIP Species Recognition",
		"图像语义分析", "人物识别", "OCR文字识别", "BioCLIP物种识别",
	} {
		line = strings.ReplaceAll(line, label, "")
	}
	return line
}

func scanTextPaths(root string, paths []string, match func(relative, line string) bool) ([]string, error) {
	allowedExtensions := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".json": true,
		".yaml": true, ".yml": true, ".md": true, ".html": true,
	}
	var matches []string
	for _, relativeRoot := range paths {
		pathRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.WalkDir(pathRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" || entry.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !allowedExtensions[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(file)
			lineNumber := 0
			for scanner.Scan() {
				lineNumber++
				if match(relative, scanner.Text()) {
					matches = append(matches, fmt.Sprintf("%s:%d:%s", relative, lineNumber, scanner.Text()))
				}
			}
			scanErr := scanner.Err()
			closeErr := file.Close()
			if scanErr != nil {
				return scanErr
			}
			return closeErr
		})
		if err != nil {
			return nil, fmt.Errorf("scan user-facing terminology in %s: %w", relativeRoot, err)
		}
	}
	return matches, nil
}
