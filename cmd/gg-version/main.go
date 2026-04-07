package main

import (
	"log"
)

// Version is injected at build time via -ldflags "-X main.Version=x.y.z".
// Defaults to "dev" for local builds without ldflags.
var Version = "dev"

func main() {
	if err := Run(Version); err != nil {
		log.Fatal(err)
	}
}
