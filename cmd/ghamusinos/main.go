// Command ghamusinos es el entrypoint del binario único: sirve la API HTTP,
// la SPA embebida y, en fases posteriores, los workers de background.
package main

import (
	"log"

	"github.com/fgjcarlos/ghamusinos/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("ghamusinos: %v", err)
	}
}
