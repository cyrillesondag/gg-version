package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"gover/config"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

var (
	configPath string
	repoPath   string
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
		},
		Commands: []*cli.Command{
			{
				Name:  "current",
				Usage: "print the current version at HEAD",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (repeatable)",
					},
				},
				Action: currentCmd,
			},
			{
				Name:  "last",
				Usage: "print the last valid semver tag reachable from HEAD",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (accepted for CLI uniformity, has no effect on last)",
					},
				},
				Action: lastCmd,
			},
			{
				Name:  "env",
				Usage: "print all template variables available for version formatting",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (repeatable)",
					},
					&cli.StringFlag{
						Name:  "format",
						Value: "plain",
						Usage: "output format: plain or json",
					},
				},
				Action: envCmd,
			},
		},
	}

	return cmd.Run(context.Background(), os.Args)
}

func currentCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	extra := parseVarFlags(cmd.StringSlice("var"))
	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Current(project, extra)
	if err != nil {
		return fmt.Errorf("computing current version: %w", err)
	}

	fmt.Println(version)
	return nil
}

func lastCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Last(project)
	if err != nil {
		return fmt.Errorf("computing last version: %w", err)
	}

	fmt.Println(version)
	return nil
}

func envCmd(ctx context.Context, cmd *cli.Command) error {
	extra := parseVarFlags(cmd.StringSlice("var"))

	project, err := gitpkg.NewProject(".", "")

	var vars map[string]interface{}
	if err != nil {
		// Not a git repo: populate only var namespace, leave others empty
		varVars := map[string]interface{}{}
		for k, v := range extra {
			varVars[k] = v
		}
		vars = map[string]interface{}{
			"semver": map[string]interface{}{"LastTag": "", "CommitCount": 0, "ShortHash": ""},
			"git":    map[string]interface{}{"Branch": ""},
			"regex":  map[string]interface{}{},
			"var":    varVars,
		}
	} else {
		strategy := semverstrategy.NewStrategy(config.DefaultConfig().Semver)
		vars, err = strategy.Vars(project, extra)
		if err != nil {
			return fmt.Errorf("computing vars: %w", err)
		}
	}

	return printVars(vars, cmd.String("format"))
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
	// plain (default): one line per variable, format namespace.key=value
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
