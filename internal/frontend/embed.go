// Package frontend embebe los assets compilados de la SPA React y expone
// un http.Handler que los sirve con fallback a index.html para rutas cliente.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Handler devuelve un http.Handler que sirve la SPA embebida.
// Si el build de Vite no se ha ejecutado aún, devuelve 503 con instrucciones.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Nunca debería ocurrir: "dist" siempre existe (al menos el .gitkeep).
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend no construido: ejecuta `make web-build`", http.StatusServiceUnavailable)
		})
	}
	return NewSPAHandler(sub)
}

// NewSPAHandler crea un handler que sirve ficheros estáticos desde fsys y
// hace fallback a index.html para cualquier ruta no encontrada (SPA routing).
//
// Si index.html NO existe en fsys (estado placeholder sin build), devuelve
// 503 con un mensaje claro para el desarrollador.
func NewSPAHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Limpiamos la ruta de posibles dobles barras, etc.
		path := r.URL.Path
		if path == "" || path == "/" {
			path = "index.html"
		} else {
			// Elimina la barra inicial para buscar dentro del FS.
			if len(path) > 0 && path[0] == '/' {
				path = path[1:]
			}
		}

		// Intentamos abrir el fichero pedido.
		f, err := fsys.Open(path)
		if err == nil {
			// El fichero existe: lo servimos con http.FileServer.
			// Ignoramos el error de Close: el descriptor se libera al
			// terminar la respuesta y no afecta a la lógica del handler.
			_ = f.Close()
			fileServer := http.FileServer(http.FS(fsys))
			fileServer.ServeHTTP(w, r)
			return
		}

		// El fichero no existe: fallback a index.html (SPA routing).
		index, err := fsys.Open("index.html")
		if err != nil {
			// Sin index.html: build no ejecutado.
			http.Error(w, "frontend no construido: ejecuta `make web-build`", http.StatusServiceUnavailable)
			return
		}
		// Ignoramos el error de Close: el descriptor se libera al servir
		// el fichero y no afecta a la respuesta del handler.
		_ = index.Close()

		// Servimos index.html manteniendo el status 200.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(http.FS(fsys)).ServeHTTP(w, r2)
	})
}
