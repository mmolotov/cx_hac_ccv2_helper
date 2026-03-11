package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	ConfigDir      = ".config"
	projectsSubDir = "projects"
)

// ConfigDirPath returns the path to the hac-helper config directory.
// Defaults to the relative path ConfigDir (.config) so that config
// lives next to the working directory where the binary is run.
// Set HAC_HELPER_CONFIG_DIR to override (used in tests).
func ConfigDirPath() (string, error) {
	if override := os.Getenv("HAC_HELPER_CONFIG_DIR"); override != "" {
		return override, nil
	}
	return ConfigDir, nil
}

// ProjectsDir returns the directory that contains per-project YAML files.
func ProjectsDir() (string, error) {
	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, projectsSubDir), nil
}

// LoadProject reads the ProjectConfig for a named project from projects/{name}.yaml.
// Returns (nil, nil) if the project file does not exist.
func LoadProject(name string) (*ProjectConfig, error) {
	dir, err := ProjectsDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name+".yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading project config %q: %w", name, err)
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config %q: %w", name, err)
	}
	if cfg.Subscriptions == nil {
		cfg.Subscriptions = make(map[string]Subscription)
	}
	return &cfg, nil
}

// SaveProject writes a ProjectConfig to projects/{name}.yaml.
func SaveProject(name string, cfg ProjectConfig) error {
	dir, err := ProjectsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating projects dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling project config: %w", err)
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing project config %q: %w", name, err)
	}
	return nil
}

// ListProjects returns the names of all projects that have a per-project config file.
func ListProjects() ([]string, error) {
	dir, err := ProjectsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return names, nil
}

// Load returns an aggregate Config over all per-project files.
func Load() (*Config, error) {
	names, err := ListProjects()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Projects: make(map[string]ProjectConfig)}
	for _, name := range names {
		proj, err := LoadProject(name)
		if err != nil {
			return nil, err
		}
		if proj != nil {
			cfg.Projects[name] = *proj
		}
	}
	return cfg, nil
}

// Save writes all projects in cfg to individual per-project files.
// Prefer SaveProject for single-project operations.
func Save(cfg *Config) error {
	for name, proj := range cfg.Projects {
		if err := SaveProject(name, proj); err != nil {
			return err
		}
	}
	return nil
}
