package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Semver SemverConfig `yaml:"semver"`
}

type SemverConfig struct {
	TagPrefix string         `yaml:"tag_prefix"`
	Initial   string         `yaml:"initial"`
	Branches  []BranchConfig `yaml:"branches"`
}

type BranchConfig struct {
	Pattern string `yaml:"pattern"`
	Release bool   `yaml:"release"`
	Format  string `yaml:"format"` // ignored when Release is true
}

// DefaultConfig returns a Config with sensible defaults.
// Used when no .gg-version.yaml file is found.
func DefaultConfig() Config {
	return Config{
		Semver: SemverConfig{
			TagPrefix: "",
			Initial:   "0.1.0",
			Branches: []BranchConfig{
				{
					Pattern: ".*",
					Release: false,
					Format:  "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}",
				},
			},
		},
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
	return cfg, nil
}
