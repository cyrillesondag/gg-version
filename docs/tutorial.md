# Tutoriel : votre première version automatique

Dans ce tutoriel, vous allez configurer `gg-version` sur un dépôt Git réel et obtenir votre premier numéro de version calculé automatiquement. Aucune connaissance préalable n'est requise.

**Ce que vous obtiendrez à la fin :** une commande utilisable dans votre pipeline CI qui affiche `v1.2.3` (ou la version appropriée) à chaque build.

---

## Prérequis

- `gg-version` installé (`go install` ou binaire téléchargé)
- Un dépôt Git avec au moins un commit

---

## Étape 1 — Vérifier que gg-version voit votre dépôt

Depuis la racine de votre dépôt :

```bash
gg-version current
```

Résultat attendu si vous n'avez aucun tag :

```
0.1.0
```

C'est la version initiale par défaut. L'outil fonctionne déjà sans aucune configuration.

---

## Étape 2 — Créer votre premier tag de version

`gg-version` lit les tags Git existants. Créez votre point de départ :

```bash
git tag v1.0.0
```

Vérifiez que l'outil le reconnaît :

```bash
gg-version current
```

```
v1.0.0
```

`gg-version` confirme que HEAD pointe sur un commit tagué — la version courante est exactement ce tag.

---

## Étape 3 — Ajouter des commits et observer l'évolution

Faites quelques commits après le tag. Utilisez le format Conventional Commits :

```bash
echo "change" >> README.md
git add README.md
git commit -m "fix: correct typo in README"

echo "feature" >> feature.txt
git add feature.txt
git commit -m "feat: add feature.txt"
```

Relancez :

```bash
gg-version current
```

```
v1.1.0
```

`gg-version` a analysé les deux commits :
- `fix:` → bump de patch
- `feat:` → bump de minor (écrase le patch)

Le résultat est `v1.1.0`, le prochain tag qui devrait être posé.

---

## Étape 4 — Créer un fichier de configuration

Sans configuration, le préfixe de tag est vide. La plupart des projets utilisent `v`. Créez `.gg-version.yml` à la racine :

```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
```

Vérifiez la configuration chargée :

```bash
gg-version config
```

```yaml
# config from: .gg-version.yml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: main
      release: true
      format: ""
    - pattern: .*
      release: false
      format: '{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}'
  ...
```

---

## Étape 5 — Observer le comportement sur une branche de feature

Créez une branche :

```bash
git checkout -b feat/my-feature
echo "work" >> work.txt
git add work.txt
git commit -m "feat: add work"
```

```bash
gg-version current
```

```
1.2.0-feat/my-feature.1
```

Sur une branche qui n'est pas `main`, `gg-version` génère un identifiant de pré-release avec le nom de branche et le nombre de commits.

---

## Étape 6 — Utiliser la version dans un script

```bash
VERSION=$(gg-version current)
echo "Building version $VERSION"
docker build -t myapp:$VERSION .
```

Sur `main` :
```
Building version v1.2.0
```

Sur une feature branch :
```
Building version 1.2.0-feat/my-feature.1
```

---

## Récapitulatif

Vous avez appris à :

1. Obtenir une version sans configuration
2. Ancrer une version avec un tag Git
3. Observer comment les Conventional Commits font évoluer la version
4. Créer un fichier de configuration minimal
5. Observer la différence entre une branche de release et une branche de feature
6. Capturer la version dans un script CI

**Prochaine étape :** consultez les [guides pratiques](how-to.md) pour des scénarios spécifiques, ou la [référence](reference.md) pour l'exhaustivité des options.
