// Package webapp embeds the Angular application that `task build` compiles
// into dist.
package webapp

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Files returns the compiled application.
func Files() fs.FS {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}

	return files
}
