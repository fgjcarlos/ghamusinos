// Package db expone el sistema de ficheros embebido con las migraciones SQL.
package db

import "embed"

// MigrationsFS contiene todos los ficheros .sql de la carpeta migrations.
// Es usado por cmd/migrate para ejecutar migraciones con goose como librería.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
