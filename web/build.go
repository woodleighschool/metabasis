package webdist

import (
	"embed"
	"io/fs"
)

var (
	// Dist embeds the built Vite bundle.
	//go:embed all:dist
	Dist embed.FS

	// DistDirFS exposes the built Vite bundle rooted at dist.
	DistDirFS = mustSub(Dist, "dist")
)

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
