package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionFile string

func (a *App) GetVersion() string {
	return strings.TrimSpace(versionFile)
}
