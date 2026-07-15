package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeConfigArchitectureHasNoFallbackOrAmbientEnvReaders(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	serverRoot := filepath.Dir(filepath.Dir(thisFile))
	for _, rel := range []string{"config", "app", "internal"} {
		root := filepath.Join(serverRoot, rel)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			source := string(data)
			if strings.Contains(source, "os.Getenv(") || strings.Contains(source, "os.LookupEnv(") {
				t.Errorf("runtime package reads ambient environment directly: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	configSource, err := os.ReadFile(filepath.Join(serverRoot, "config", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"LoadOptions", "ProcessEnv", "defaultAppConfigForEnvironment", "applyEnvOverrides"} {
		if strings.Contains(string(configSource), forbidden) {
			t.Errorf("config fallback API was reintroduced: %s", forbidden)
		}
	}
	lumenSource, err := os.ReadFile(filepath.Join(serverRoot, "internal", "service", "lumen_service.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"lumenconfig.DefaultConfig(", ".LoadFromEnv("} {
		if strings.Contains(string(lumenSource), forbidden) {
			t.Errorf("Lumen ambient/default config was reintroduced: %s", forbidden)
		}
	}

	moduleFile, err := os.ReadFile(filepath.Join(serverRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"github.com/joho/godotenv",
		"github.com/cloudwego/eino-ext/components/model/deepseek",
	} {
		if strings.Contains(string(moduleFile), forbidden) {
			t.Errorf("dependency can reintroduce ambient .env configuration: %s", forbidden)
		}
	}
}
