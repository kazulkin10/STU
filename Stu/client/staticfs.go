package client

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/public admin/public
var staticFS embed.FS

func WebHandler() http.Handler {
	fsys, err := fs.Sub(staticFS, "web/public")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fsys))
}

func AdminHandler() http.Handler {
	fsys, err := fs.Sub(staticFS, "admin/public")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/admin", http.FileServer(http.FS(fsys)))
}
