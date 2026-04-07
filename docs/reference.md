# Référence

---

## Commandes

### `current`

Affiche la version au HEAD.

```
gg-version [global flags] current
```

**Comportement :**
- HEAD est tagué → affiche exactement ce tag
- HEAD non tagué → affiche `""` (chaîne vide), exit 0
- Aucun tag trouvé → affiche `""` (chaîne vide)

> Pour obtenir la version calculée sur un HEAD non tagué, utilisez `next`.

**Exemples :**

```bash
# HEAD exactement sur un tag
gg-version current
# v1.4.2

# HEAD non tagué
gg-version current
# (chaîne vide)

# Avec variable de template
gg-version --var env=prod current
# (utilise {{ .var.env }} dans le template de format)

# En monorepo — @root tagué, api non tagué
gg-version current
# @root        v2.1.0
# api          

gg-version current --format json
# {
#   "@root": "v2.1.0",
#   "api": ""
# }

# Filtrer un composant
gg-version current --component api
# (vide si api non tagué)

gg-version current --root
# v2.1.0
```

**Flags :**

| Flag | Défaut | Description |
|---|---|---|
| `--format <plain\|json>` | `plain` | Format de sortie |

---

### `next`

Affiche la version calculée au HEAD — qu'elle existe comme tag ou non. Comportement identique à l'ancien `current`.

```
gg-version [global flags] next
```

**Comportement :**
- HEAD est tagué → affiche ce tag
- HEAD non tagué, branche de release → affiche le prochain tag calculé (`tag_prefix + semver`)
- HEAD non tagué, branche de pré-release → rend le template `format` de la branche
- Aucun tag trouvé → affiche `initial`

**Exemples :**

```bash
gg-version next
# v1.5.0  (version calculée même si HEAD non tagué)

gg-version next --format json
# "v1.5.0"

# En monorepo — tous les composants avec version calculée
gg-version next
# @root        v2.2.0
# api          v0.6.0

gg-version next --format json
# {
#   "@root": "v2.2.0",
#   "api": "v0.6.0"
# }
```

**Flags :**

| Flag | Défaut | Description |
|---|---|---|
| `--format <plain\|json>` | `plain` | Format de sortie |

> **Migration :** Si vos pipelines utilisaient `current` pour la version calculée sur un commit non tagué, migrez vers `next`.

---

### `last`

Affiche le dernier tag semver atteignable depuis HEAD.

```
gg-version [global flags] last
```

**Comportement :**
- Remonte tous les ancêtres de HEAD, filtre les tags valides selon `tag_prefix`, retourne le plus proche topologiquement.
- Aucun tag trouvé → affiche `initial`.
- Contrairement à `current`, `last` n'analyse pas les commits : il retourne le tag tel quel, sans calculer de bump.

**Exemples :**

```bash
gg-version last
# v1.4.1

# En monorepo
gg-version last
# @root        v2.0.0
# api          v0.4.0
# frontend     v2.9.0

gg-version last --component frontend
# v2.9.0
```

**Flags :**

| Flag | Défaut | Description |
|---|---|---|
| `--format <plain\|json>` | `plain` | Format de sortie |

---

### `env`

Affiche toutes les variables de template disponibles.

```
gg-version [global flags] env [--format plain|json]
```

**Comportement :**
- En mode monorepo, affiche les variables pour chaque composant préfixées par son nom.
- Si le dépôt ou la config est inaccessible, affiche uniquement les variables `var.*`.

**Exemples :**

```bash
# Format par défaut (plain)
gg-version env
# git.AuthorDate=2026-04-06
# git.Branch=main
# git.CommitCount=5
# git.CommitterDate=2026-04-06
# git.Hash=abc1234def5678901234567890abcdef12345678
# git.LastTag=v1.2.0
# git.ShortHash=abc1234
# semver.HasNonConventionalCommits=false
# semver.IsBreakingChange=false
# semver.IsPreRelease=false
# semver.LastMajor=1
# semver.LastMinor=2
# semver.LastPatch=0
# semver.LastPreRelease=
# semver.LastVersion=1.2.0
# semver.Major=1
# semver.Minor=3
# semver.Patch=0
# semver.PreRelease=
# semver.Semver=1.3.0

# Format JSON
gg-version env --format json

# Avec variable personnalisée
gg-version --var buildno=42 env
# var.buildno=42

# En monorepo (plain)
gg-version env
# @root.git.Branch=main
# @root.semver.Semver=2.1.0
# api.git.LastTag=api/v0.5.0
# api.semver.Semver=0.5.1
# ...

# En monorepo (JSON)
gg-version env --format json
# {
#   "@root": { "git": {...}, "semver": {...} },
#   "api": { "git": {...}, "semver": {...} }
# }

gg-version env --component api --format json
```

---

### `config`

Affiche la configuration effective (valeurs par défaut + fichier `.gg-version.yml` mergé).

```
gg-version [global flags] config [--format yaml|json]
```

**Exemples :**

```bash
gg-version config
# config from: .gg-version.yml
# semver:
#   tag_prefix: v
#   initial: 0.1.0
#   branches: ...

gg-version config --format json
# {
#   "_source": ".gg-version.yml",
#   "semver": { ... },
#   "components": { ... }
# }

# Quand aucun fichier de config n'existe
gg-version config
# # default config
# semver:
#   tag_prefix: ""
#   ...
```

---

### `components`

Liste les composants définis dans la configuration (monorepo).

```
gg-version [global flags] components [--format plain|json]
```

**Exemples :**

```bash
gg-version components
# api          path=api/**                      tag=api/v*
# frontend     path=frontend/**                 tag=frontend/v*

gg-version components --format json
# {
#   "api": {
#     "path": "api/**",
#     "tag_scope": "api",
#     "tag_pattern": "api/v*"
#   },
#   "frontend": {
#     "path": "frontend/**",
#     "tag_scope": "frontend",
#     "tag_pattern": "frontend/v*"
#   }
# }

# Sans composants définis
gg-version components
# (no components defined)
```

---

## Flags globaux

Ces flags s'appliquent à toutes les commandes et se placent avant le nom de la commande.

| Flag | Défaut | Description |
|---|---|---|
| `--config <path>` | `.gg-version.yml` | Chemin vers le fichier de configuration |
| `--repo <path>` | `.` | Chemin vers le dépôt Git |
| `--component <name>` | _(aucun)_ | Filtre la sortie sur un seul composant (monorepo) |
| `--root` | `false` | Affiche uniquement le composant `@root` (monorepo) |
| `--var <name=value>` | _(aucun)_ | Variable de template supplémentaire (répétable) |

`--component` et `--root` sont mutuellement exclusifs.

```bash
gg-version --repo /path/to/project --config /path/to/.gg-version.yml current
gg-version --component api current
gg-version --root last
```

---

## Fichier de configuration

Emplacement par défaut : `.gg-version.yml` à la racine du dépôt. Si le fichier n'existe pas, les valeurs par défaut s'appliquent.

### Schéma complet

```yaml
semver:
  # Préfixe attendu sur les tags Git. Exemple : "v" pour des tags v1.2.3.
  # Défaut : "" (pas de préfixe)
  tag_prefix: "v"

  # Version retournée quand aucun tag n'est trouvé.
  # Défaut : "0.1.0"
  initial: "0.1.0"

  # Règles par branche. Évaluées dans l'ordre — la première qui correspond est utilisée.
  branches:
    - pattern: "main"        # Expression régulière Go
      release: true          # true → version de release (pas de template)
      # format est ignoré quand release: true

    - pattern: "release/(?P<major>[0-9]+)\\.x"
      release: true

    - pattern: ".*"
      release: false
      # Template Go. Variables disponibles : {{ .semver.* }}, {{ .git.* }},
      # {{ .regex.* }}, {{ .var.* }}
      format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"

  # Règles de détection des Conventional Commits (expressions régulières Go).
  conventional_commits:
    # Format général d'un commit CC. Un commit qui correspond mais n'est dans
    # aucune liste major/minor/patch → aucun bump (BumpNone).
    format: '^\w+(?:\(.+\))?!?:'

    # Patterns qui déclenchent un bump MAJOR.
    major:
      - '^\w+(?:\(.+\))?!:'      # feat!: ou fix!:
      - 'BREAKING[- ]CHANGE:'    # footer BREAKING CHANGE:

    # Patterns qui déclenchent un bump MINOR.
    minor:
      - '^feat(?:\(.+\))?:'

    # Patterns qui déclenchent un bump PATCH.
    patch:
      - '^fix(?:\(.+\))?:'

  # Chemins à exclure du calcul de version (globs doublestar).
  # Un commit est ignoré si TOUS ses fichiers modifiés correspondent à au moins
  # un pattern. Un commit mixte (docs + code) n'est pas ignoré.
  # Défaut : []
  ignore_paths:
    - "docs/**"
    - "*.md"
    - ".github/**"

  # SHAs de commits à ignorer (préfixes acceptés).
  # Défaut : []
  ignore_commits:
    - "abc1234"
    - "deadbeef"

# Composants pour les monorepos. Optionnel.
components:
  api:
    # Glob des fichiers appartenant à ce composant.
    path: "api/**"
    # Scope du tag. Défaut : le nom de la clé ("api").
    # Les tags de ce composant seront api/v1.2.3.
    tag_scope: "api"

  frontend:
    path: "frontend/**"
    # Sans tag_scope, les tags seront frontend/v1.2.3.
```

---

## Variables de template

Disponibles dans le champ `format` des branches et via `gg-version env`.

### Namespace `semver`

| Variable | Type | Description |
|---|---|---|
| `semver.Semver` | string | Version calculée par CC (ex : `1.3.0`) |
| `semver.Major` | string | Composant major de la version calculée |
| `semver.Minor` | string | Composant minor de la version calculée |
| `semver.Patch` | string | Composant patch de la version calculée |
| `semver.PreRelease` | string | Pré-release de la version calculée (souvent vide) |
| `semver.LastVersion` | string | Version du dernier tag (sans préfixe, ex : `1.2.0`) |
| `semver.LastMajor` | string | Major du dernier tag |
| `semver.LastMinor` | string | Minor du dernier tag |
| `semver.LastPatch` | string | Patch du dernier tag |
| `semver.LastPreRelease` | string | Pré-release du dernier tag |
| `semver.IsBreakingChange` | bool | `true` si au moins un commit MAJOR depuis le dernier tag |
| `semver.IsPreRelease` | bool | `true` si la branche courante n'est pas de release |
| `semver.HasNonConventionalCommits` | bool | `true` si au moins un commit ne respecte pas le format CC |

### Namespace `git`

| Variable | Type | Description |
|---|---|---|
| `git.Branch` | string | Nom de la branche courante |
| `git.AuthorDate` | string | Date de l'auteur du commit HEAD (format `2006-01-02`) |
| `git.CommitterDate` | string | Date du committer du commit HEAD (format `2006-01-02`) |
| `git.LastTag` | string | Dernier tag trouvé (avec préfixe, ex : `v1.2.0`), vide si aucun |
| `git.Hash` | string | Hash complet du commit HEAD |
| `git.ShortHash` | string | 7 premiers caractères du hash |
| `git.CommitCount` | int | Nombre de commits depuis le dernier tag |
| `git.IsShallow`  | bool | `true` si le dépôt est un clone superficiel (`git clone --depth=N`) |
| `git.Truncated`  | bool | `true` si l'historique a été tronqué avant d'atteindre le tag de référence |

### Namespace `regex`

Captures nommées extraites du pattern de branche qui correspond. Exemple avec `(?P<ticket>[A-Z]+-[0-9]+)` :

| Variable | Description |
|---|---|
| `regex.ticket` | Valeur capturée du groupe nommé `ticket` |

### Namespace `var`

Variables injectées via `--var name=value` sur la ligne de commande :

```bash
gg-version --var env=staging --var buildno=42 current
```

Accessibles comme `{{ .var.env }}` et `{{ .var.buildno }}`.

---

## Tags en monorepo

Quand des composants sont définis, chaque composant a son propre espace de tags :

| Composant | `tag_scope` | `tag_prefix` | Format du tag |
|---|---|---|---|
| `api` | `api` (défaut) | `v` | `api/v1.2.3` |
| `frontend` | `frontend` (défaut) | `v` | `frontend/v1.2.3` |
| `shared` | `libs` (explicite) | `v` | `libs/v0.9.0` |

`@root` utilise le `tag_prefix` global sans scope :

| Composant | Format du tag |
|---|---|
| `@root` | `v2.1.0` |

---

## Règles des Conventional Commits

Priorités de bump pour chaque commit :

| Critère | Bump |
|---|---|
| Sujet ou footer correspond à un pattern `major` | MAJOR |
| Sujet correspond à un pattern `minor` | MINOR |
| Sujet correspond à un pattern `patch` | PATCH |
| Sujet correspond au `format` CC mais aucun pattern | Aucun |
| Sujet ne correspond pas au `format` CC | PATCH (+ `HasNonConventionalCommits=true`) |

Le bump le plus élevé parmi tous les commits depuis le dernier tag détermine la version calculée.

---

## Codes de sortie

| Code | Signification |
|---|---|
| `0` | Succès |
| `1` | Erreur (dépôt introuvable, config invalide, flag inconnu…) |
