BINARY := bin/ghamusinos

.PHONY: help build run test tidy fmt vet check \
        web-install web-build \
        db-up db-down migrate migrate-status generate

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ─── Frontend ────────────────────────────────────────────────────────────────

web-install: ## Instala dependencias del frontend (pnpm)
	pnpm -C web install

web-build: ## Compila el frontend y deposita assets en internal/frontend/dist
	pnpm -C web build

# ─── Build ───────────────────────────────────────────────────────────────────

build: web-build ## Compila el binario en bin/ghamusinos (incluye build del frontend)
	GOTOOLCHAIN=local go build -o $(BINARY) ./cmd/ghamusinos

# ─── Dev ─────────────────────────────────────────────────────────────────────

run: ## Ejecuta la aplicación
	GOTOOLCHAIN=local go run ./cmd/ghamusinos

# ─── Tests y calidad ─────────────────────────────────────────────────────────

test: ## Ejecuta los tests
	GOTOOLCHAIN=local go test ./...

tidy: ## Ordena y limpia go.mod
	GOTOOLCHAIN=local go mod tidy

fmt: ## Formatea el código
	GOTOOLCHAIN=local go fmt ./...

vet: ## Análisis estático
	GOTOOLCHAIN=local go vet ./...

check: fmt vet test ## fmt + vet + test

# ─── Base de datos ───────────────────────────────────────────────────────────

db-up: ## Levanta el contenedor de PostgreSQL/TimescaleDB
	docker compose up -d db

db-down: ## Para y elimina los contenedores (los volúmenes se conservan)
	docker compose down

migrate: ## Ejecuta las migraciones pendientes (up)
	GOTOOLCHAIN=local go run ./cmd/migrate up

migrate-status: ## Muestra el estado de las migraciones
	GOTOOLCHAIN=local go run ./cmd/migrate status

# ─── Generación de código ────────────────────────────────────────────────────

generate: ## Genera código Go con SQLC (requiere Docker)
	docker run --rm -v "$(PWD):/src" -w /src sqlc/sqlc generate
