package backend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var rawFS embed.FS

// DistFS returns the filesystem rooted inside the "dist" directory.
func DistFS() (fs.FS, error) {
	return fs.Sub(rawFS, "dist")
}
