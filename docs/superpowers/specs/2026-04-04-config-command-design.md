# config Command Design

**Date:** 2026-04-04

## Goal

Ajouter une commande `config` qui affiche la configuration effective telle que chargée par `config.Load()`, avec indication de la source (fichier ou défauts).

---

## Section 1 : Architecture et CLI

### Commande

```
gg-version [--config path] config [--format yaml|json]
```

- `--config` : flag global racine existant (`configPath`), déjà partagé par `current`, `last`
- `--format yaml|json` : flag local sur `config` uniquement, défaut `yaml`
- Implémenté dans `command/commands.go` uniquement — aucune modification de `config/config.go`

### Détection de la source

Le sous-commande vérifie si le fichier `configPath` existe via `os.Stat` avant d'appeler `config.Load()` :

- Fichier absent → source `"default"`
- Fichier présent → source = valeur de `configPath` (ex: `".gg-version.yml"`)

### Sortie YAML (défaut)

```yaml
# default config
semver:
  tag_prefix: ""
  initial: 0.1.0
  branches:
    - pattern: .*
      release: false
      format: '{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}'
```

Quand chargé depuis un fichier :
```yaml
# config from: .gg-version.yml
semver:
  tag_prefix: "v"
  ...
```

### Sortie JSON

Ajoute un champ `"_source"` au niveau racine (JSON ne supporte pas les commentaires) :

```json
{
  "_source": "default",
  "semver": {
    "tag_prefix": "",
    "initial": "0.1.0",
    "branches": [...]
  }
}
```

Quand chargé depuis un fichier, `"_source"` vaut le chemin du fichier (ex: `".gg-version.yml"`).

---

## Section 2 : Tests

Pas de nouveau fichier de test — `command/` n'a pas de suite de tests, et les cas d'erreur de parsing YAML sont déjà couverts par `config/config_test.go`.

Vérification manuelle :

```bash
# Config par défaut
gg-version config                              # YAML avec commentaire # default config
gg-version config --format json               # JSON avec "_source": "default"

# Config depuis fichier
gg-version --config .gg-version.yml config    # YAML avec commentaire # config from: ...
gg-version --config .gg-version.yml config --format json

# Validation du commentaire/source
gg-version config | head -1                   # → # default config
gg-version config --format json | grep _source  # → "default"
```

---

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `command/commands.go` | Ajout sous-commande `config` avec `--format` flag et helpers `printConfig*` |
