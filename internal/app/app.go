// Package app conecta configuración, servidor HTTP y dependencias del binario.
//
// En la fase 1.1 Run irá creciendo para arrancar el servidor HTTP (Chi), la
// conexión a base de datos y el frontend embebido. De momento es un esqueleto
// que compila y permite establecer la estructura del proyecto.
package app

import "fmt"

// Run es el punto de entrada de la aplicación. Devuelve un error en lugar de
// terminar el proceso para que el caller (main) decida cómo reportarlo.
func Run() error {
	fmt.Println("ghamusinos: esqueleto fase 1.1 — servidor pendiente")
	return nil
}
