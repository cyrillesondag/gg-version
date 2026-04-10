# Shell Completion — Design Spec

**Goal:** Activer la completion shell native de `urfave/cli/v3` pour bash, zsh, fish et PowerShell.

**Architecture:** Un seul champ ajouté au root `cli.Command` dans `commands.go`. La lib génère automatiquement une sous-commande `completion <shell>` qui imprime le script d'initialisation à sourcer.

**Tech Stack:** Go 1.24, urfave/cli/v3 v3.8.0 (support natif).

---

## Problème

`gg-version` n'offre pas de completion shell. Les utilisateurs doivent taper les noms de commandes et de flags en entier. `urfave/cli/v3` supporte la completion nativement — elle n'est pas activée.

---

## Design

### Activation dans `commands.go`

Dans la fonction `Run()`, ajouter deux champs au root `cli.Command` :

```go
cmd := &cli.Command{
    Name:                       "gg-version",
    Version:                    version,
    EnableShellCompletion:      true,
    ShellCompletionCommandName: "completion",
    // ... reste inchangé
}
```

`EnableShellCompletion: true` enregistre automatiquement la sous-commande `completion`. `ShellCompletionCommandName` est optionnel (valeur par défaut : `"completion"`), mais le rendre explicite évite toute surprise si la valeur par défaut change dans une future version.

### Sous-commande générée automatiquement

```
gg-version completion <shell>
```

Shells supportés par `urfave/cli/v3` : `bash`, `zsh`, `fish`, `powershell`.

### Usage pour l'utilisateur final

```bash
# bash
echo 'source <(gg-version completion bash)' >> ~/.bashrc

# zsh
echo 'source <(gg-version completion zsh)' >> ~/.zshrc

# fish
gg-version completion fish > ~/.config/fish/completions/gg-version.fish

# PowerShell
gg-version completion powershell >> $PROFILE
```

---

## Fichiers

| Fichier | Modification |
|---|---|
| `cmd/gg-version/commands.go` | Ajouter `EnableShellCompletion: true` et `ShellCompletionCommandName: "completion"` au root Command |
| `docs/reference.md` | Ajouter une section "Shell completion" avec les instructions par shell |

---

## Tests

Pas de test unitaire à écrire — la logique de génération est dans `urfave/cli/v3`. Vérification manuelle : `go run ./cmd/gg-version completion bash` doit produire un script non vide.

---

## Breaking changes

Aucun. La sous-commande `completion` est additionnelle.
