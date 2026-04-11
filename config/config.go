package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ComponentConfig struct {
	Path     string `yaml:"path"`      // glob, ex: "api/**"
	TagScope string `yaml:"tag_scope"` // optionnel, défaut = nom de la clé
}

type Config struct {
	Semver     SemverConfig               `yaml:"semver"`
	Components map[string]ComponentConfig `yaml:"components"`
}

type ConventionalCommitsConfig struct {
	Format string   `yaml:"format"`
	Major  []string `yaml:"major"`
	Minor  []string `yaml:"minor"`
	Patch  []string `yaml:"patch"`
}

type SemverConfig struct {
	TagPrefix           string                    `yaml:"tag_prefix"`
	Initial             string                    `yaml:"initial"`
	Branches            []BranchConfig            `yaml:"branches"`
	ConventionalCommits ConventionalCommitsConfig `yaml:"conventional_commits"`
	IgnorePaths         []string                  `yaml:"ignore_paths"`
	IgnoreCommits       []string                  `yaml:"ignore_commits"`
}

type BranchConfig struct {
	Pattern       string `yaml:"pattern"`
	VersionFormat string `yaml:"version_format"` // empty = release branch; non-empty = pre-release suffix template
}

// DefaultConfig returns a Config with sensible defaults.
// Used when no .gg-version.yaml file is found.
func DefaultConfig() Config {
	return Config{
		Semver: SemverConfig{
			TagPrefix: "",
			Initial:   "0.1.0",
			Branches: []BranchConfig{
				{Pattern: "main"},
				{Pattern: "master"},
				{
					Pattern:       ".*",
					VersionFormat: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
				},
			},
			ConventionalCommits: ConventionalCommitsConfig{
				Format: `^\w+(?:\(.+\))?!?:`,
				Major: []string{
					`^\w+(?:\(.+\))?!:`,
					`BREAKING[- ]CHANGE:`,
				},
				Minor: []string{`^feat(?:\(.+\))?:`},
				Patch: []string{`^fix(?:\(.+\))?:`},
			},
		},
		Components: map[string]ComponentConfig{},
	}
}

// Load reads the YAML config file at path and returns a Config.
// If the file does not exist, DefaultConfig is returned without error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validating config file %s: %w", path, err)
	}
	return cfg, nil
}
