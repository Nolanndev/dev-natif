package httpapi

import (
	"embed"
	"io/fs"
)

// webFS holds the embedded single-page console (static assets, no build step).
//
//go:embed web
var webFS embed.FS

// webRoot returns the embedded assets rooted at the web/ directory.
func webRoot() fs.FS {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		// web/ is embedded at compile time; this can only fail on a packaging bug.
		panic(err)
	}
	return sub
}
