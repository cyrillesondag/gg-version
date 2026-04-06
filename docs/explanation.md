# Concepts : comment gg-version calcule les versions

Ce document explique le raisonnement derrière le fonctionnement de `gg-version`. Lisez-le si vous voulez comprendre *pourquoi* l'outil se comporte d'une certaine façon, pas juste *comment* l'utiliser.

---

## Le principe fondamental : lire sans écrire

`gg-version` ne crée jamais de tag, ne fait jamais de commit, ne modifie aucun fichier. Il lit l'historique Git et calcule ce que *devrait* être la version — la décision de poser un tag reste entièrement à vous.

Cette séparation est délibérée. L'outil peut être exécuté à n'importe quel moment sans effets de bord, ce qui le rend idéal en CI : le même appel produit le même résultat que vous soyez en train de vérifier localement ou dans un pipeline.

---

## Comment la version est calculée

Le calcul se déroule en trois phases.

### Phase 1 : trouver le dernier tag

`gg-version` remonte tous les ancêtres de HEAD et identifie les tags valides (ceux qui commencent par le `tag_prefix` configuré et contiennent un semver valide). Il retient le tag **topologiquement le plus proche** — pas le plus récent dans le temps, mais celui qui est le plus proche dans le graphe de commits.

Si plusieurs tags se trouvent à égale distance, le plus élevé sémantiquement est retenu.

Si aucun tag n'est trouvé, la valeur `initial` est utilisée comme base (par défaut `0.1.0`).

### Phase 2 : analyser les commits intermédiaires

`gg-version` récupère tous les commits entre le dernier tag et HEAD (le tag lui-même exclu). Il analyse le message de chaque commit selon les règles des Conventional Commits :

- Le **sujet** (première ligne) est testé contre les patterns `major`, `minor`, `patch` dans cet ordre.
- Les **footers** (lignes après la première ligne blanche) sont également testés — c'est là que `BREAKING CHANGE:` est reconnu.
- Un commit qui ne respecte pas le format CC est traité comme un patch (et marque `HasNonConventionalCommits=true`).
- Un commit CC de type inconnu (ex : `docs:`, `chore:`) contribue `BumpNone` — il est reconnu mais n'augmente pas la version.

Le niveau de bump final est le maximum parmi tous les commits analysés.

### Phase 3 : produire la version

Le bump est appliqué à la version du dernier tag pour obtenir le semver calculé (ex : `1.3.0`). Ensuite :

**Si HEAD est exactement sur un tag** → la version est ce tag, sans calcul.

**Si la branche est de release** (`release: true`) → la version est `tag_prefix + semver` (ex : `v1.3.0`). C'est la version que vous devriez tagger.

**Si la branche est de pré-release** (`release: false`) → le template `format` est rendu avec toutes les variables disponibles. La version produite identifie le build sans prétendre être une release.

---

## Releases vs pré-releases

La distinction `release: true / false` est centrale.

Une **branche de release** (`main`, `master`, `release/x.y`…) produit des versions propres prêtes à être taguées : `v1.3.0`. Ces versions sont stables et signifient "ce code est prêt à être livré".

Une **branche de pré-release** (feature, hotfix, develop…) produit des identifiants de build : `1.3.0-feat/login.5`. Ces versions permettent de tracer un build précis sans polluer l'espace des versions stables.

Le template `format` n'est rendu que sur les branches de pré-release. Sur une branche de release, il est ignoré.

---

## Pourquoi `last` et `current` sont différents

`gg-version last` répond à : *"Quel est le dernier tag posé ?"*

`gg-version current` répond à : *"Quelle version ce code représente-t-il ?"*

Sur un commit non tagué sur `main` avec des `feat:` depuis `v1.2.0`, les réponses sont :

```bash
gg-version last     # v1.2.0  — le dernier tag existant
gg-version current  # v1.3.0  — la version que devrait avoir ce code
```

`last` est utile pour vérifier ce qui a été livré. `current` est utile pour nommer ce qui va être livré.

---

## Comment fonctionne le filtrage de chemins (monorepo)

Quand `ignore_paths` ou des composants sont configurés, les commits sont filtrés avant l'analyse CC.

La règle d'exclusion est intentionnellement stricte : un commit n'est ignoré que si **tous** ses fichiers modifiés correspondent à un pattern d'exclusion. Un commit qui touche à la fois `docs/README.md` et `src/api.go` n'est pas ignoré — il contribue à la version même si la documentation est exclue.

Cette règle évite les faux négatifs : mieux vaut sur-compter un commit que le manquer et produire une version qui sous-estime le changement réel.

Pour les composants, la logique est symétrique : un commit appartient à un composant si au moins un de ses fichiers correspond au `path` de ce composant (inclusion partielle).

---

## Isolation des composants en monorepo

Chaque composant vit dans son propre espace de tags (`{scope}/{prefix}{version}`) et son propre espace de commits (filtré par `path`).

`@root` est le composant implicite qui représente "tout ce qui ne touche pas un composant déclaré". Ses commits sont ceux qui n'appartiennent à aucun composant. C'est utile pour versionner la configuration globale, les scripts de déploiement, ou tout code partagé qui ne mérite pas son propre composant.

Conséquence importante : un commit qui touche deux composants (`api/` et `frontend/`) compte pour les deux. Ce n'est pas un bug — un changement partagé doit bien se refléter dans la version des deux composants.

---

## Les templates Go

Les formats de version utilisent la syntaxe standard des templates Go (`text/template`). Quelques rappels utiles :

```
{{ .semver.Major }}          → valeur brute
{{ printf "%02d" .git.CommitCount }}  → formatage numérique
```

Les templates ont accès à toutes les variables des quatre namespaces : `semver`, `git`, `regex`, `var`. Une variable absente produit une chaîne vide sans erreur.
