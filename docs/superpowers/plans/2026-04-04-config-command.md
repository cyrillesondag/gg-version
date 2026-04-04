# config Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ajouter la sous-commande `config` qui affiche la configuration effective chargée par `config.Load()`, avec indicateur de source.

**Architecture:** Une seule modification dans `command/commands.go` — ajout de `configCmd`, import de `gopkg.in/yaml.v3`, enregistrement de la sous-commande. Pas de changement à `config/config.go`.

**Tech Stack:** Go, `urfave/cli/v3`, `gopkg.in/yaml.v3`, `encoding/json`

---

### Task 1: config subcommand

**Goal:** Ajouter `gg-version [--config path] config [--format yaml|json]` qui affiche la configuration effective avec commentaire de source.

**Files:**
- Modify: `command/commands.go`

**Acceptance Criteria:**
- [ ] `gg-version config` affiche du YAML valide avec `# default config` en tête
- [ ] `gg-version config --format json` affiche du JSON avec `"_source": "default"`
- [ ] `gg-version --config .gg-version.yml config` (si le fichier existe) affiche `# config from: .gg-version.yml`
- [ ] `gg-version --config .gg-version.yml config --format json` affiche `"_source": ".gg-version.yml"`
- [ ] `gg-version config --format xml` retourne une erreur `unknown format "xml"`
- [ ] `/usr/local/go/bin/go build ./...` → succès
- [ ] `/usr/local/go/bin/go test ./...` → tous PASS

**Verify:** `/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...` → PASS

**Steps:**

- [ ] **Step 1 : Ajouter l'import `gopkg.in/yaml.v3` dans `command/commands.go`**

Modifier le bloc d'imports existant :

```go
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
```

- [ ] **Step 2 : Enregistrer la sous-commande `config` dans `Run()`**

Ajouter après la sous-commande `env` dans le slice `Commands` :

```go
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
```

- [ ] **Step 3 : Implémenter `configCmd`**

Ajouter après `envCmd` :

```go
func configCmd(ctx context.Context, cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "yaml" && format != "json" {
		return fmt.Errorf("unknown format %q: must be yaml or json", format)
	}

	// Detect source: does the config file exist on disk?
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
			Source string             `json:"_source"`
			Semver config.SemverConfig `json:"semver"`
		}{
			Source: source,
			Semver: cfg.Semver,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling config to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// YAML (default)
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
```

- [ ] **Step 4 : Vérifier le build et les tests**

```bash
/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...
```

Résultat attendu : build succès, tous les tests PASS.

- [ ] **Step 5 : Vérifier manuellement**

```bash
# Config par défaut — YAML
/usr/local/go/bin/go run . config
# Résultat attendu (première ligne) : # default config

# Config par défaut — JSON
/usr/local/go/bin/go run . config --format json
# Résultat attendu : JSON avec "_source": "default"

# Format inconnu
/usr/local/go/bin/go run . config --format xml
# Résultat attendu : erreur "unknown format \"xml\": must be yaml or json"
```

- [ ] **Step 6 : Stager et commiter**

```bash
git add command/commands.go
git commit -m "feat: add config command to display effective configuration"
```

```json:metadata
{"files": ["command/commands.go"], "verifyCommand": "/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...", "acceptanceCriteria": ["config affiche YAML avec commentaire source", "config --format json avec _source", "format inconnu retourne erreur", "build et tests passent"]}
```
