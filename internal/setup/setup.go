// Package setup enthält die Einrichtungs- und Einstellungsseite. Absichtlich schlichtes HTML ohne Build-Schritt,
// getrennt von der Wiedergabe-UI: Sie wird auf PC oder Handy bedient, nie auf dem TV.
package setup

import (
	"embed"
	"io/fs"
)

//go:embed *.html *.css
var pages embed.FS

func FS() fs.FS { return pages }
