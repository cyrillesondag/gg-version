# Semver Vars Enrichment Design

**Date:** 2026-04-05

## Goal

Enrichir le map de variables de `Strategy.Vars()` avec les composants semver parsés (`Major`, `Minor`, `Patch`, `PreRelease`) et la date du jour (`git.Date`), disponibles à la fois dans les templates de format et dans la sortie de la commande `env`.

---

## Context

Ceci est la **feature A** d'un plan en deux parties :
- **A** (ce doc) : enrichissement des vars avec composants semver + date
- **B** (futur) : calcul de version par Conventional Commits

---

## Section 1 : Changements dans `strategy/semver/semver.go`

### Vars() enrichi

`Vars()` est la source unique de vérité pour les variables de template. Deux ajouts :

#### Composants semver dans `semver.*`

Parsés depuis `LastTag` via `gosemver` (dépendance déjà présente) :

```go
"semver": map[string]interface{}{
    "LastTag":     "1.2.3",
    "Major":       "1",
    "Minor":       "2",
    "Patch":       "3",
    "PreRelease":  "",        // "rc.1" si le tag contient un suffixe pre-release
    "CommitCount": 2,
    "ShortHash":   "abc1234",
},
```

Règles de parsing :
- Si `lastTag != "0.0.0"` → parser `lastTag`
- Si `lastTag == "0.0.0"` (pas de tag) → parser `cfg.Initial` (ex: `"0.1.0"`)
- Si le parse échoue → `Major`, `Minor`, `Patch`, `PreRelease` restent `""`
- Les valeurs sont des **strings** (pas des entiers) pour uniformité avec les autres vars

#### Date du jour dans `git.*`

```go
"git": map[string]interface{}{
    "Branch": "main",
    "Date":   "2026-04-05",   // time.Now().UTC().Format("2006-01-02")
},
```

---

## Section 2 : `env` et templates

Aucun changement à la commande `env` — elle affiche tout ce que `Vars()` retourne, y compris les nouveaux champs.

Les templates de format peuvent désormais utiliser :
```
{{ .semver.Major }}.{{ .semver.Minor }}.{{ .semver.Patch }}-dev.{{ .semver.CommitCount }}
```

`semver.PreRelease` est vide pour les tags de release (`1.2.3`), non-vide uniquement si le tag lui-même contient un suffixe (ex: `1.2.3-rc.1 → PreRelease="rc.1"`). L'enrichissement avec la version calculée par Conventional Commits viendra en feature B.

---

## Section 3 : Tests

Nouveaux tests dans `strategy/semver/semver_test.go` :

- `TestVars_semverMajorMinorPatch` : tag `"1.2.3"` → `Major="1"`, `Minor="2"`, `Patch="3"`, `PreRelease=""`
- `TestVars_semverPreRelease` : tag `"1.2.3-rc.1"` → `PreRelease="rc.1"`
- `TestVars_semverNoTag` : pas de tag → composants depuis `cfg.Initial` (`"0.1.0"` → `Major="0"`, `Minor="1"`, `Patch="0"`)
- `TestVars_gitDate` : `vars["git"]["Date"]` est une chaîne au format `YYYY-MM-DD`

---

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `strategy/semver/semver.go` | Enrichissement de `Vars()` |
| `strategy/semver/semver_test.go` | 4 nouveaux tests |
