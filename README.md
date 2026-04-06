# gg-version

> Calcule automatiquement la version de votre projet à partir de l'historique Git et des Conventional Commits — sans jamais écrire dans le dépôt.

---

## Installation

```bash
go install github.com/yourorg/gg-version@latest
```

Ou depuis les sources :

```bash
git clone https://github.com/yourorg/gg-version.git
cd gg-version
go build -o gg-version .
```

---

## Démarrage rapide

```bash
# Version actuelle au HEAD
gg-version current
# → v1.4.2

# Dernière version taguée
gg-version last
# → v1.4.1

# Toutes les variables disponibles pour le formatage
gg-version env
```

Sans fichier de configuration, `gg-version` fonctionne avec des valeurs par défaut raisonnables.

---

## Documentation

| Document | Quand le lire |
|---|---|
| [Tutoriel](docs/tutorial.md) | Vous débutez — suivez un exemple pas à pas |
| [Guides pratiques](docs/how-to.md) | Vous avez un objectif précis (monorepo, CI, format personnalisé…) |
| [Référence](docs/reference.md) | Vous cherchez un flag, une option de config ou une variable de template |
| [Concepts](docs/explanation.md) | Vous voulez comprendre le fonctionnement interne |

---

## Fonctionnement en une phrase

`gg-version` remonte l'historique Git depuis HEAD, trouve le dernier tag semver atteignable, analyse les commits intermédiaires avec les Conventional Commits, et en déduit la version courante — sans jamais créer de tag ni modifier le dépôt.
