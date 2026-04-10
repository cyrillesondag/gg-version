# Largeur de colonnes dynamique — Design Spec

**Goal:** Remplacer les largeurs de colonnes fixes dans la sortie tabulaire plain par des largeurs calculées à partir des données réelles, pour éviter le désalignement avec les noms longs.

**Architecture:** Deux corrections chirurgicales dans `cmd/gg-version/commands.go`, même pattern : une passe de calcul du `maxLen`, puis une passe d'affichage avec `%-*s`.

**Tech Stack:** Go 1.24, stdlib uniquement.

---

## Problème

Deux `Printf` utilisent des largeurs fixes :

```go
fmt.Printf("%-12s %s\n", r.Name, r.Version)          // printComponentResults
fmt.Printf("%-12s path=%-30s tag=%s\n", name, ...)   // componentsCmd
```

Un composant nommé `my-super-long-service` (20 chars) dépasse les 12 chars réservés et désaligne toutes les colonnes suivantes.

---

## Design

### Pattern commun

Avant chaque bloc d'affichage tabulaire, calculer la largeur maximale de chaque colonne variable, puis utiliser `%-*s` (largeur dynamique via `*`) dans le format string.

```go
maxLen := 0
for _, item := range items {
    if len(item.Col) > maxLen {
        maxLen = len(item.Col)
    }
}
for _, item := range items {
    fmt.Printf("%-*s ...\n", maxLen, item.Col, ...)
}
```

### Correction 1 — `printComponentResults`

Colonne unique : `r.Name`.

```go
// avant
for _, r := range filtered {
    fmt.Printf("%-12s %s\n", r.Name, r.Version)
}

// après
maxLen := 0
for _, r := range filtered {
    if len(r.Name) > maxLen {
        maxLen = len(r.Name)
    }
}
for _, r := range filtered {
    fmt.Printf("%-*s %s\n", maxLen, r.Name, r.Version)
}
```

### Correction 2 — `componentsCmd` (format plain)

Deux colonnes variables : `name` et `info.Path`.

```go
// avant
for _, name := range names {
    info := infoMap[name]
    fmt.Printf("%-12s path=%-30s tag=%s\n", name, info.Path, info.TagPattern)
}

// après
maxNameLen, maxPathLen := 0, 0
for _, name := range names {
    if len(name) > maxNameLen {
        maxNameLen = len(name)
    }
    if len(infoMap[name].Path) > maxPathLen {
        maxPathLen = len(infoMap[name].Path)
    }
}
for _, name := range names {
    info := infoMap[name]
    fmt.Printf("%-*s path=%-*s tag=%s\n", maxNameLen, name, maxPathLen, info.Path, info.TagPattern)
}
```

---

## Périmètre

- Format JSON : non affecté.
- Format plain, résultat unique (non-monorepo ou filtré `--component`/`--root`) : non affecté (pas de tableau).
- Tests sur `cmd/gg-version/` : hors périmètre (problème distinct dans IMPROVEMENTS.md).

---

## Fichiers

| Fichier | Modification |
|---|---|
| `cmd/gg-version/commands.go` | Remplacer largeurs fixes dans `printComponentResults` et `componentsCmd` |

---

## Breaking changes

Aucun. La sortie reste identique pour les noms courts (largeur calculée = 12 ou moins → même rendu). Seuls les noms longs voient leur colonne élargie.
