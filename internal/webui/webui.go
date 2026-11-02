// Package webui embeds the built frontend into the binary.
//
// The dist directory is produced by `npm run build` in ../../frontend. It is
// not committed, apart from a placeholder so this package always compiles.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend, with index.html at its root.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time, so this cannot happen
	}
	return sub
}
