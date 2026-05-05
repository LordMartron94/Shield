package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateTransientMainSource_GlueOnlyRuntimeCall(t *testing.T) {
	source := generateTransientMainSource([]string{"shield/tests"})
	if !strings.Contains(source, "_ \"shield/tests\"") {
		t.Fatalf("expected discovery blank import in generated source")
	}
	if !strings.Contains(source, "\"shield/runner\"") {
		t.Fatalf("expected runner import in generated source")
	}
	if !strings.Contains(source, "runner.RunShieldEntrypoint") {
		t.Fatalf("expected generated source to delegate into runner entrypoint")
	}
}

func TestLoadShieldConfiguration_RejectsNonCanonicalDiscoveryModule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shield_config.toml")
	cfg := `
[environment]
name = "local"
db_path = ".shield/telemetry.sqlite"
color_mode = "none"

[discovery]
modules = ["force/tools/shield/tests"]

[runtime]
transient = false
`
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatalf("failed writing config fixture: %v", err)
	}

	_, err := loadShieldConfiguration(path)
	if err == nil {
		t.Fatalf("expected non-canonical discovery module validation error")
	}
	if !strings.Contains(err.Error(), "use 'shield/...' module paths") {
		t.Fatalf("unexpected error: %v", err)
	}
}
