package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirPathOverride(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", "/tmp/custom")
	got, err := ConfigDirPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/custom" {
		t.Errorf("ConfigDirPath() = %q, want /tmp/custom", got)
	}
	if IsLegacyConfigDir() {
		t.Error("IsLegacyConfigDir() should be false when override is set")
	}
}

func TestConfigDirPathDefaultsToHome(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	got, err := ConfigDirPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "hac-helper")
	if got != want {
		t.Errorf("ConfigDirPath() = %q, want %q", got, want)
	}
}

func TestConfigDirPathLegacy(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", "")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(LegacyConfigDir, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := ConfigDirPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != LegacyConfigDir {
		t.Errorf("ConfigDirPath() = %q, want %q", got, LegacyConfigDir)
	}
}

func TestSaveLoadProjectRoundtrip(t *testing.T) {
	t.Setenv("HAC_HELPER_CONFIG_DIR", t.TempDir())
	in := ProjectConfig{Type: "direct", HACURL: "https://localhost:9002", HACUser: "admin"}
	if err := SaveProject("p1", in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadProject("p1")
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.HACURL != in.HACURL || out.HACUser != in.HACUser || out.Type != in.Type {
		t.Errorf("roundtrip mismatch: got %+v", out)
	}
	names, err := ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "p1" {
		t.Errorf("ListProjects() = %v, want [p1]", names)
	}
	missing, err := LoadProject("nope")
	if err != nil || missing != nil {
		t.Errorf("LoadProject(missing) = %v, %v; want nil, nil", missing, err)
	}
}
