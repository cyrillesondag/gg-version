package command

import (
	"context"
	"fmt"
	"os"

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
				Name:   "current",
				Usage:  "print the current version at HEAD",
				Action: currentCmd,
			},
			{
				Name:   "last",
				Usage:  "print the last valid semver tag reachable from HEAD",
				Action: lastCmd,
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

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Current(project)
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
