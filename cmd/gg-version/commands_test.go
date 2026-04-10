package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"gover/config"
)

var commitCounter atomic.Int64

// newOnDiskRepo creates a go-git repo in t.TempDir() with an initial commit.
func newOnDiskRepo(t *testing.T) (string, *gogit.Repository) {
	t.Helper()
	dir := t.TempDir()
	r, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	f, err := w.Filesystem.Create("README.md")
	if err != nil {
		t.Fatalf("Create README: %v", err)
	}
	fmt.Fprintln(f, "init")
	f.Close()
	if _, err := w.Add("README.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := w.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "t@t.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return dir, r
}

// addCommit creates a commit (unique file whose content changes) in the worktree.
func addCommit(t *testing.T, r *gogit.Repository, msg string) plumbing.Hash {
	t.Helper()
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	f, err := w.Filesystem.Create("file.txt")
	if err != nil {
		t.Fatalf("Create file: %v", err)
	}
	fmt.Fprintf(f, "commit-%d", commitCounter.Add(1))
	f.Close()
	if _, err := w.Add("file.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hash, err := w.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "t@t.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("Commit %q: %v", msg, err)
	}
	return hash
}

// addTag creates a lightweight tag on the given commit.
func addTag(t *testing.T, r *gogit.Repository, hash plumbing.Hash, name string) {
	t.Helper()
	if _, err := r.CreateTag(name, hash, nil); err != nil {
		t.Fatalf("CreateTag %q: %v", name, err)
	}
}

// testCtx returns a context.Context with the globalFlags injected.
func testCtx(flags globalFlags) context.Context {
	return context.WithValue(context.Background(), contextKey{}, flags)
}

// testCmd returns a *cli.Command whose flags have been parsed from args.
// Use to get cmd.String("format"), cmd.Bool("dry-run"), etc.
func testCmd(t *testing.T, args ...string) *cli.Command {
	t.Helper()
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "plain"},
			&cli.BoolFlag{Name: "dry-run"},
			&cli.BoolFlag{Name: "push"},
			&cli.StringFlag{Name: "message"},
		},
		Action: func(ctx context.Context, c *cli.Command) error { return nil },
	}
	if err := cmd.Run(context.Background(), append([]string{"cmd"}, args...)); err != nil {
		t.Fatalf("testCmd.Run: %v", err)
	}
	return cmd
}

// captureOutput redirects os.Stdout during fn() and returns what was written.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old }) // restore even on panic/t.FailNow
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		buf.ReadFrom(r)
		close(done)
	}()
	fn()
	w.Close()
	<-done
	return buf.String()
}

// writeConfig writes cfg as YAML to a temp file and returns its path.
func writeConfig(t *testing.T, cfg config.Config) string {
	t.Helper()
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	f, err := os.CreateTemp(t.TempDir(), "*.yml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.Write(b); err != nil {
		t.Fatalf("Write config: %v", err)
	}
	f.Close()
	return f.Name()
}

// testConfig returns a minimal valid Config for tests.
// TagPrefix "" → tags named "1.0.0" (not "v1.0.0").
// Pattern ".*" Release:true → any branch counts as a release branch.
func testConfig() config.Config {
	return config.Config{
		Semver: config.SemverConfig{
			TagPrefix: "",
			Initial:   "0.1.0",
			Branches: []config.BranchConfig{
				{Pattern: ".*", Release: true},
			},
			ConventionalCommits: config.ConventionalCommitsConfig{
				Format: `^\w+(?:\(.+\))?!?:`,
				Major:  []string{`^\w+(?:\(.+\))?!:`, `BREAKING[- ]CHANGE:`},
				Minor:  []string{`^feat(?:\(.+\))?:`},
				Patch:  []string{`^fix(?:\(.+\))?:`},
			},
		},
	}
}

func TestNextCmd(t *testing.T) {
	t.Run("HeadAheadOfTag", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")
		addCommit(t, r, "feat: add something")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = nextCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("nextCmd error: %v", cmdErr)
		}
		if strings.TrimSpace(out) == "" {
			t.Error("expected non-empty output from nextCmd")
		}
	})

	t.Run("ComponentAndRoot", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{
			Config:    cfgPath,
			Repo:      dir,
			Component: "api",
			Root:      true,
		})
		cmd := testCmd(t)

		err := nextCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected error for --component + --root, got nil")
		}
	})

	t.Run("FormatJSON", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")
		addCommit(t, r, "fix: patch something")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t, "--format", "json")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = nextCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("nextCmd error: %v", cmdErr)
		}
		if !json.Valid([]byte(strings.TrimSpace(out))) {
			t.Errorf("expected valid JSON, got: %q", out)
		}
	})
}

func TestLastCmd(t *testing.T) {
	t.Run("WithTag", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "3.1.0")
		addCommit(t, r, "chore: bump")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = lastCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("lastCmd error: %v", cmdErr)
		}
		if !strings.Contains(out, "3.1.0") {
			t.Errorf("expected '3.1.0' in output, got: %q", out)
		}
	})

	t.Run("WithoutTag", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = lastCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("lastCmd error: %v", cmdErr)
		}
		// No tag → AllLast returns cfg.Initial ("0.1.0")
		if !strings.Contains(out, "0.1.0") {
			t.Errorf("expected '0.1.0' (cfg.Initial) when no tag, got: %q", out)
		}
	})

	t.Run("ComponentAndRoot", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{
			Config:    cfgPath,
			Repo:      dir,
			Component: "api",
			Root:      true,
		})
		cmd := testCmd(t)

		err := lastCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected error for --component + --root, got nil")
		}
	})
}

func TestEnvCmd(t *testing.T) {
	t.Run("Namespaces", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = envCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("envCmd error: %v", cmdErr)
		}
		// plain format only prints non-empty namespaces; semver and git are always present
		for _, ns := range []string{"semver", "git"} {
			if !strings.Contains(out, ns) {
				t.Errorf("expected namespace %q in output, got: %q", ns, out)
			}
		}
	})

	t.Run("FormatJSON", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t, "--format", "json")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = envCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("envCmd error: %v", cmdErr)
		}
		if !json.Valid([]byte(strings.TrimSpace(out))) {
			t.Errorf("expected valid JSON, got: %q", out)
		}
		for _, ns := range []string{"semver", "git", "regex", "var"} {
			if !strings.Contains(out, ns) {
				t.Errorf("expected namespace %q in JSON, got: %q", ns, out)
			}
		}
	})

	t.Run("UnknownComponent", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{
			Config:    cfgPath,
			Repo:      dir,
			Component: "nonexistent",
		})
		cmd := testCmd(t)

		err := envCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected error for unknown component, got nil")
		}
	})
}

func TestConfigCmd(t *testing.T) {
	t.Run("FormatYAML", func(t *testing.T) {
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t, "--format", "yaml")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = configCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("configCmd error: %v", cmdErr)
		}
		if !strings.Contains(out, "semver") {
			t.Errorf("expected 'semver' key in YAML output, got: %q", out)
		}
	})

	t.Run("FormatJSON", func(t *testing.T) {
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t, "--format", "json")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = configCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("configCmd error: %v", cmdErr)
		}
		if !json.Valid([]byte(strings.TrimSpace(out))) {
			t.Errorf("expected valid JSON, got: %q", out)
		}
		if !strings.Contains(out, "_source") {
			t.Errorf("expected '_source' key in JSON output, got: %q", out)
		}
	})

	t.Run("UnknownFormat", func(t *testing.T) {
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t, "--format", "toml")

		err := configCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected error for unknown format, got nil")
		}
	})
}

func TestComponentsCmd(t *testing.T) {
	t.Run("NoComponents", func(t *testing.T) {
		cfg := testConfig() // no components
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = componentsCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("componentsCmd error: %v", cmdErr)
		}
		if !strings.Contains(out, "no components") {
			t.Errorf("expected 'no components' in output, got: %q", out)
		}
	})

	t.Run("WithComponent", func(t *testing.T) {
		cfg := testConfig()
		cfg.Components = map[string]config.ComponentConfig{
			"api": {Path: "api/**"},
		}
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t)

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = componentsCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("componentsCmd error: %v", cmdErr)
		}
		if !strings.Contains(out, "api") {
			t.Errorf("expected component 'api' in output, got: %q", out)
		}
	})

	t.Run("FormatJSON", func(t *testing.T) {
		cfg := testConfig()
		cfg.Components = map[string]config.ComponentConfig{
			"web": {Path: "web/**"},
		}
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath})
		cmd := testCmd(t, "--format", "json")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = componentsCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("componentsCmd error: %v", cmdErr)
		}
		if !json.Valid([]byte(strings.TrimSpace(out))) {
			t.Errorf("expected valid JSON, got: %q", out)
		}
		if !strings.Contains(out, "web") {
			t.Errorf("expected component 'web' in JSON, got: %q", out)
		}
	})
}

func TestLintCmd(t *testing.T) {
	t.Run("HeadOnTag", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		err := lintCmd(ctx, cmd)
		if err != nil {
			t.Fatalf("lintCmd error on tagged HEAD: %v", err)
		}
	})

	t.Run("ValidCC", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")
		addCommit(t, r, "feat: add feature")
		addCommit(t, r, "fix: patch bug")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		err := lintCmd(ctx, cmd)
		if err != nil {
			t.Fatalf("lintCmd error with valid CC commits: %v", err)
		}
	})

	t.Run("InvalidCC", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")
		addCommit(t, r, "not a conventional commit message")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		err := lintCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected exit code 1 for non-CC commits, got nil")
		}
		var exitErr cli.ExitCoder
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected cli.ExitCoder, got %T: %v", err, err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	})
}

func TestTagCmd(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")
		addCommit(t, r, "feat: new feature")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t, "--dry-run")

		var cmdErr error
		out := captureOutput(t, func() {
			cmdErr = tagCmd(ctx, cmd)
		})
		if cmdErr != nil {
			t.Fatalf("tagCmd dry-run error: %v", cmdErr)
		}
		if !strings.Contains(out, "would") {
			t.Errorf("expected 'would' in dry-run output, got: %q", out)
		}
		// Verify no new tag was created
		tags, _ := r.Tags()
		count := 0
		tags.ForEach(func(_ *plumbing.Reference) error { count++; return nil })
		if count != 1 {
			t.Errorf("expected 1 tag after dry-run, got %d", count)
		}
	})

	t.Run("AlreadyTagged", func(t *testing.T) {
		dir, r := newOnDiskRepo(t)
		head, _ := r.Head()
		addTag(t, r, head.Hash(), "1.0.0")

		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{Config: cfgPath, Repo: dir})
		cmd := testCmd(t)

		// HEAD already tagged → "already tagged" message to stderr, no error
		err := tagCmd(ctx, cmd)
		if err != nil {
			t.Fatalf("tagCmd on already-tagged HEAD: %v", err)
		}
	})

	t.Run("ComponentAndRoot", func(t *testing.T) {
		dir, _ := newOnDiskRepo(t)
		cfg := testConfig()
		cfgPath := writeConfig(t, cfg)

		ctx := testCtx(globalFlags{
			Config:    cfgPath,
			Repo:      dir,
			Component: "api",
			Root:      true,
		})
		cmd := testCmd(t)

		err := tagCmd(ctx, cmd)
		if err == nil {
			t.Fatal("expected error for --component + --root, got nil")
		}
	})
}
