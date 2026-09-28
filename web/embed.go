// Package web bettet die gebaute Oberfläche (npm run build → dist/) in die Server-Binary ein.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func FS() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}
