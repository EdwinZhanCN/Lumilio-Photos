package supervisor

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	serverconfig "server/config"

	"github.com/pelletier/go-toml/v2"
)

//go:embed server.template.toml
var desktopServerTemplate string

type serverManifestBindings struct {
	Port, BrowserOrigin, WebRoot, LogDir, StoragePath string
	DBHost, DBPort, DBUser, DBName                    string
	BootstrapPasswordFile, RotatedPasswordFile        string
	SecretKeyFile, PGBinDir                           string
	ExifToolPath, FFmpegPath, FFprobePath             string
	LumenStaticNode                                   string
}

func compileAndLoadServerManifest(path string, bindings serverManifestBindings) (serverconfig.AppConfig, error) {
	funcs := template.FuncMap{"toml": tomlLiteral}
	tmpl, err := template.New("desktop-server.toml").Option("missingkey=error").Funcs(funcs).Parse(desktopServerTemplate)
	if err != nil {
		return serverconfig.AppConfig{}, fmt.Errorf("parse desktop server manifest template: %w", err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, bindings); err != nil {
		return serverconfig.AppConfig{}, fmt.Errorf("render desktop server manifest: %w", err)
	}
	if err := writeAtomicPrivate(path, rendered.Bytes()); err != nil {
		return serverconfig.AppConfig{}, fmt.Errorf("write desktop server manifest: %w", err)
	}
	cfg, err := serverconfig.LoadAppConfig(path)
	if err != nil {
		return serverconfig.AppConfig{}, fmt.Errorf("reload generated desktop server manifest: %w", err)
	}
	return cfg, nil
}

func tomlLiteral(value any) (string, error) {
	data, err := toml.Marshal(map[string]any{"value": value})
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	literal, ok := strings.CutPrefix(line, "value = ")
	if !ok || strings.Contains(literal, "\n") {
		return "", fmt.Errorf("cannot encode TOML literal for %T", value)
	}
	return literal, nil
}

func writeAtomicPrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := applyPrivateDirectoryMode(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".server.toml-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := applyPrivateFileMode(tmpPath); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}
