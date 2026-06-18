// Package anatomyweb embeds the built Anatomy frontend (Vite output under
// dist/). `just anatomy-web` regenerates dist/ before the Go build embeds it.
package anatomyweb

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend rooted at dist/.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
