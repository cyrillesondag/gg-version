# Pistes d'amélioration

Analyse DevOps du fonctionnement et de l'architecture de `gg-version`.

---

## Problèmes bloquants en production

### Shallow clones non supportés

Quasiment toutes les pipelines CI modernes utilisent `git clone --depth=N`. Si le tag de référence est au-delà de la profondeur du clone, `CommitSinceTag` retourne une liste tronquée ou échoue silencieusement — pouvant produire `0.1.0` (initial) alors que le vrai dernier tag est `v3.2.1`. Aucune détection ni avertissement n'existe.

Piste : détecter que le dépôt est un clone superficiel et retourner une erreur explicite, ou supporter `--unshallow` automatique.

### `git.Date` retourne la date du build, pas celle du commit

```go
"Date": time.Now().UTC().Format("2006-01-02"),
```

Deux builds sur le même commit à deux jours d'intervalle produisent des versions différentes. C'est une violation de la reproductibilité et une surprise pour quiconque utilise `{{ .git.Date }}` en pensant dater le commit.

Piste : utiliser `commit.Author.When` ou `commit.Committer.When`.

---

## Ergonomie critique

### `current` est sémantiquement ambigu

Sur un commit non tagué sur `main`, `gg-version current` retourne `v1.3.0` — une version qui n'existe pas encore comme tag. C'est en réalité le *next* version. La sémantique trompe les utilisateurs : "quelle est la version courante ?" → `v1.3.0` → "mais ce tag n'existe pas dans le dépôt".

Piste : ajouter une commande `next` explicite ou documenter clairement la distinction, et/ou ajouter un flag `--tagged-only` qui retourne une erreur si HEAD n'est pas tagué.

### Pas de `--format json` sur `current` et `last`

Les commandes les plus utilisées en CI n'ont pas de sortie structurée. La sortie multi-composants (monorepo) n'est pas parseable proprement sans `awk`.

```bash
# Impossible aujourd'hui
gg-version current --format json
# { "@root": "v2.1.0", "api": "v0.5.1", "frontend": "v3.0.0" }
```

### `--var` est local à la sous-commande, pas global

`--component` et `--root` sont des flags globaux. `--var` est un flag de sous-commande. Un utilisateur qui tente `gg-version --var env=prod current` obtient une erreur silencieuse. L'asymétrie est surprenante.

### Sortie tabulaire multi-composants cassée au-delà de 12 caractères

```go
fmt.Printf("%-12s %s\n", r.Name, r.Version)
```

Un composant nommé `my-super-long-service` désaligne toute la sortie. Aucune alternative machine-readable pour `current` en mode monorepo.

### `@root` est risqué en contexte shell

`@` est un caractère spécial dans certains contextes (tags Docker, git remote syntax). `gg-version current --component @root` peut être ambigu selon le shell. Un nom plus neutre (`root`, `_root`) serait plus sûr.

### Pas de shell completion

`urfave/cli/v3` supporte nativement bash/zsh/fish completion. Non activée.

---

## Fonctionnalités manquantes

### Pas de création de tag

L'outil calcule la version mais ne peut pas créer le tag correspondant. Oblige à du boilerplate répété dans chaque pipeline. Une commande `gg-version tag [--push] [--dry-run]` serait la complétion naturelle du workflow.

### Pas de `gg-version --version`

Un outil de versioning qui ne peut pas afficher sa propre version est difficile à déboguer en prod. Indispensable pour les rapports de bug et les logs CI.

### Pas de codes de sortie sémantiques

Actuellement : `0` = succès, `1` = erreur. Des codes exploitables en CI permettraient :

| Code | Signification |
|---|---|
| `0` | HEAD tagué — version stable |
| `2` | HEAD non tagué — version calculée, tag non posé |
| `3` | Branche de pré-release |

### Pas de `lint` / validation des commits

```bash
gg-version lint
# ✗ 3 commits ne respectent pas les Conventional Commits :
#   a1b2c3d "WIP fix auth"
#   def4567 "temp"
#   890abcd "Merge pull request #42"
```

Indispensable pour faire adopter CC dans une équipe et donner du feedback aux développeurs avant que les commits ne polluent le calcul de version.

### Pas de validation de la configuration au chargement

Un pattern de branche invalide en Go regex, un `initial` qui n'est pas un semver valide, un chemin de composant vide — tout est accepté sans erreur au `Load()` et explose plus tard avec un message cryptique. Une passe de validation au démarrage éviterait des puzzles de debug en CI.

### `@root` non configurable en monorepo

`@root` utilise toujours le `tag_prefix` global sans scope. Impossible de lui associer un scope comme `core/v1.0.0`. La config de `@root` devrait être exposée comme celle de n'importe quel composant.

### Pas de changelog automatique

Fonctionnalité complémentaire classique : générer un CHANGELOG.md à partir des commits CC depuis le dernier tag. Naturel à implémenter avec l'infrastructure d'analyse existante.

---

## Fiabilité et tests

### Pas de tests sur `command/`

```
?   gover/command   [no test files]
```

C'est la couche d'intégration entre la CLI et la logique métier. Les bugs de parsing de flags, d'exclusion mutuelle `--component/--root`, de filtrage et de formatage de sortie ne sont testés nulle part. Surface de régression classique.

### Variables globales pour les flags — commandes non testables

```go
var (
    configPath    string
    repoPath      string
    componentFlag string
    rootFlag      bool
)
```

Ces variables mutées par `urfave/cli` rendent le code non-réentrant et les commandes impossibles à tester sans contournements. Piste : struct de configuration passée via le contexte.

### Non-CC commits provoquent un bump de patch implicite

Un message `WIP`, `tmp`, ou le message de merge automatique GitHub (`Merge pull request #42`) déclenche un bump de patch. Aucun moyen de dire "échoue si un commit non-CC est trouvé" ou "ignore les commits non-CC" sans les lister manuellement dans `ignore_commits`.

---

## Architecture et performance

### `varsCore` appelé N+1 fois pour N composants

`AllCurrent` fait un `CommitSinceTag` complet par composant. Sur un dépôt avec 10 000 commits et 5 composants, c'est 6 parcours complets d'historique. Le parcours devrait être factorisé : récupérer une fois tous les commits depuis le tag root le plus ancien, puis filtrer par composant en mémoire.

### `LastTag` a une complexité quadratique latente

```go
candidateIsNewer, _ := isAncestor(tagCommit, lastTagCommit)
```

Pour chaque paire de tags candidats, `isAncestor` remonte tout l'historique. Sur K tags valides et N commits : O(K × N). Sur un monorepo chargé en tags (`api/v0.1.0` … `api/v5.3.0`), cela devient mesurable.

### `CommitFiles` utilise `c.Stats()` — calcul de diff inutile

`Stats()` calcule la taille des diffs ligne par ligne. Pour le filtrage par chemin, seule la liste des fichiers modifiés est nécessaire. La comparaison d'arbres (`commit.Tree()` vs `parent.Tree()`) serait significativement plus rapide sur les gros commits.

### `Compare` défini mais jamais utilisé comme tiebreaker dans `LastTag`

`SemverFormat.Compare` satisfait l'interface mais `LastTag` ne l'utilise jamais. Quand deux tags sont topologiquement équidistants (branche mergée), le résultat dépend de l'ordre d'itération non garanti par `go-git`. `Compare` devrait servir de tiebreaker déterministe.

### `Strategy` n'est pas une interface

`Strategy` est une struct. `GitProject` est correctement une interface mockable. `Strategy` ne l'est pas, ce qui empêche de la substituer dans les tests ou de fournir des implémentations alternatives. L'asymétrie est incohérente.

### Module `gover`, binaire `gg-version`

Sans `go build -o gg-version`, `go install` produit un binaire nommé `gover`. Cette divergence est un piège pour les contributeurs et rend l'installation standard incorrecte sans instruction explicite.

---

## Synthèse par priorité

| Priorité | Problème |
|---|---|
| **Bloquant prod** | Shallow clones non supportés |
| **Bloquant prod** | `git.Date` = date de build, pas du commit |
| **Ergonomie critique** | Pas de `--format json` sur `current` |
| **Ergonomie critique** | `current` sémantiquement ambigu (`next` en réalité) |
| **Fiabilité** | Pas de tests sur `command/` |
| **Fiabilité** | Variables globales — commandes non testables |
| **Performance** | `varsCore` × N appels non factorisés |
| **Performance** | `LastTag` O(K×N) sur les repos chargés en tags |
| **Fonctionnel** | Pas de création de tag (même avec `--dry-run`) |
| **Fonctionnel** | Pas de `gg-version --version` |
| **Fonctionnel** | Pas de `lint` / validation des commits |
| **Fonctionnel** | Pas de codes de sortie sémantiques |
| **Fonctionnel** | Non-CC commits → patch implicite non configurable |
| **Architecture** | `Strategy` n'est pas une interface |
| **Architecture** | Module `gover` ≠ binaire `gg-version` |
