package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"gover/config"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

var (
	configPath    string
	repoPath      string
	componentFlag string
	rootFlag      bool
)

func Run() error {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       ".gg-version.yml",
				Destination: &configPath,
				Usage:       "path to the configuration file",
			},
			&cli.StringFlag{
				Name:        "repo",
				Value:       ".",
				Destination: &repoPath,
				Usage:       "path to the git repository",
			},
			&cli.StringFlag{
				Name:        "component",
				Destination: &componentFlag,
				Usage:       "filter output to a single component (use with components defined in config)",
			},
			&cli.BoolFlag{
				Name:        "root",
				Destination: &rootFlag,
				Usage:       "show only the root version, ignoring components",
			},
			&cli.StringSliceFlag{
				Name:  "var",
				Usage: "extra template variable as name=value (repeatable)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "current",
				Usage:  "print the current version at HEAD",
				Action: currentCmd,
			},
			{
				Name:   "last",
				Usage:  "print the last valid semver tag reachable from HEAD",
				Action: lastCmd,
			},
			{
				Name:  "env",
				Usage: "print all template variables available for version formatting",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Value: "plain",
						Usage: "output format: plain or json",
					},
				},
				Action: envCmd,
			},
			{
				Name:  "config",
				Usage: "print the effective configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Value: "yaml",
						Usage: "output format: yaml or json",
					},
				},
				Action: configCmd,
			},
			{
				Name:  "components",
				Usage: "list components defined in the configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Value: "plain",
						Usage: "output format: plain or json",
					},
				},
				Action: componentsCmd,
			},
		},
	}

	return cmd.Run(context.Background(), os.Args)
}

func currentCmd(ctx context.Context, cmd *cli.Command) error {
	if componentFlag != "" && rootFlag {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	extra := parseVarFlags(cmd.Root().StringSlice("var"))
	strategy := semverstrategy.NewStrategy(cfg.Semver)

	results, err := strategy.AllCurrent(project, extra, cfg)
	if err != nil {
		return fmt.Errorf("computing current version: %w", err)
	}
	return printComponentResults(results)
}

func lastCmd(ctx context.Context, cmd *cli.Command) error {
	if componentFlag != "" && rootFlag {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	strategy := semverstrategy.NewStrategy(cfg.Semver)

	results, err := strategy.AllLast(project, cfg)
	if err != nil {
		return fmt.Errorf("computing last version: %w", err)
	}
	return printComponentResults(results)
}

func envCmd(ctx context.Context, cmd *cli.Command) error {
	if componentFlag != "" && rootFlag {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	extra := parseVarFlags(cmd.Root().StringSlice("var"))
	format := cmd.String("format")

	project, err := gitpkg.NewProject(repoPath, "")
	cfg, cfgErr := config.Load(configPath)

	if err != nil || cfgErr != nil {
		// Not a git repo or no config: populate only var namespace
		varVars := map[string]interface{}{}
		for k, v := range extra {
			varVars[k] = v
		}
		vars := map[string]interface{}{
			"semver": map[string]interface{}{},
			"git":    map[string]interface{}{},
			"regex":  map[string]interface{}{},
			"var":    varVars,
		}
		return printVars(vars, format)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	allResults, err := strategy.AllVars(project, extra, cfg)
	if err != nil {
		return fmt.Errorf("computing vars: %w", err)
	}

	filtered := filterVarsResults(allResults)

	if componentFlag != "" && len(filtered) == 0 {
		return fmt.Errorf("component %q not found in config", componentFlag)
	}

	// Single unnamed result (no components): plain vars output
	if len(filtered) == 1 && filtered[0].Name == "" {
		return printVars(filtered[0].Vars, format)
	}

	if format == "json" {
		out := map[string]interface{}{}
		for _, r := range filtered {
			out[r.Name] = r.Vars
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling vars to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// plain multi-component: prefix each key with component name
	for _, r := range filtered {
		prefix := r.Name + "."
		if r.Name == "" {
			prefix = ""
		}
		for _, ns := range []string{"semver", "git", "regex", "var"} {
			nsVars, ok := r.Vars[ns].(map[string]interface{})
			if !ok {
				continue
			}
			keys := make([]string, 0, len(nsVars))
			for k := range nsVars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s%s.%s=%v\n", prefix, ns, k, nsVars[k])
			}
		}
	}
	return nil
}

func configCmd(ctx context.Context, cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "yaml" && format != "json" {
		return fmt.Errorf("unknown format %q: must be yaml or json", format)
	}

	source := configPath
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		source = "default"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if format == "json" {
		out := struct {
			Source     string                            `json:"_source"`
			Semver     config.SemverConfig               `json:"semver"`
			Components map[string]config.ComponentConfig `json:"components,omitempty"`
		}{
			Source:     source,
			Semver:     cfg.Semver,
			Components: cfg.Components,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling config to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}
	var comment string
	if source == "default" {
		comment = "# default config\n"
	} else {
		comment = fmt.Sprintf("# config from: %s\n", source)
	}
	fmt.Print(comment + string(b))
	return nil
}

func componentsCmd(ctx context.Context, cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "plain" && format != "json" {
		return fmt.Errorf("unknown format %q: must be plain or json", format)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Components) == 0 {
		fmt.Println("(no components defined)")
		return nil
	}

	type componentInfo struct {
		Path       string `json:"path"`
		TagScope   string `json:"tag_scope"`
		TagPattern string `json:"tag_pattern"`
	}
	infoMap := map[string]componentInfo{}
	names := make([]string, 0, len(cfg.Components))
	for name := range cfg.Components {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		comp := cfg.Components[name]
		scope := comp.TagScope
		if scope == "" {
			scope = name
		}
		pattern := scope + "/" + cfg.Semver.TagPrefix + "*"
		infoMap[name] = componentInfo{
			Path:       comp.Path,
			TagScope:   scope,
			TagPattern: pattern,
		}
	}

	if format == "json" {
		b, err := json.MarshalIndent(infoMap, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	for _, name := range names {
		info := infoMap[name]
		fmt.Printf("%-12s path=%-30s tag=%s\n", name, info.Path, info.TagPattern)
	}
	return nil
}

// filterComponentResults filters results based on --component and --root flags.
func filterComponentResults(results []semverstrategy.ComponentResult) []semverstrategy.ComponentResult {
	if rootFlag {
		for _, r := range results {
			if r.Name == "@root" || r.Name == "" {
				return []semverstrategy.ComponentResult{r}
			}
		}
		return results[:1]
	}
	if componentFlag != "" {
		for _, r := range results {
			if r.Name == componentFlag {
				return []semverstrategy.ComponentResult{r}
			}
		}
		return nil
	}
	return results
}

func filterVarsResults(results []semverstrategy.ComponentVarsResult) []semverstrategy.ComponentVarsResult {
	if rootFlag {
		for _, r := range results {
			if r.Name == "@root" || r.Name == "" {
				return []semverstrategy.ComponentVarsResult{r}
			}
		}
		return results[:1]
	}
	if componentFlag != "" {
		for _, r := range results {
			if r.Name == componentFlag {
				return []semverstrategy.ComponentVarsResult{r}
			}
		}
		return nil
	}
	return results
}

// printComponentResults prints version results to stdout.
// Single unnamed result (no components): prints version only.
// Single result filtered by --component or --root: prints version only.
// Multiple or named results: prints "name    version" per line.
func printComponentResults(results []semverstrategy.ComponentResult) error {
	filtered := filterComponentResults(results)

	if componentFlag != "" && len(filtered) == 0 {
		return fmt.Errorf("component %q not found in config", componentFlag)
	}

	if len(filtered) == 1 && filtered[0].Name == "" {
		fmt.Println(filtered[0].Version)
		return nil
	}

	if len(filtered) == 1 && (componentFlag != "" || rootFlag) {
		fmt.Println(filtered[0].Version)
		return nil
	}

	for _, r := range filtered {
		fmt.Printf("%-12s %s\n", r.Name, r.Version)
	}
	return nil
}

// parseVarFlags parses a slice of "name=value" strings into a map.
func parseVarFlags(rawVars []string) map[string]string {
	extra := map[string]string{}
	for _, v := range rawVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			extra[parts[0]] = parts[1]
		}
	}
	return extra
}

// printVars prints vars to stdout in the requested format (plain or json).
func printVars(vars map[string]interface{}, format string) error {
	if format != "plain" && format != "json" {
		return fmt.Errorf("unknown format %q: must be plain or json", format)
	}
	if format == "json" {
		out, err := json.MarshalIndent(vars, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling vars to JSON: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}
	for _, ns := range []string{"semver", "git", "regex", "var"} {
		nsVars, ok := vars[ns].(map[string]interface{})
		if !ok {
			continue
		}
		keys := make([]string, 0, len(nsVars))
		for k := range nsVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s.%s=%v\n", ns, k, nsVars[k])
		}
	}
	return nil
}
