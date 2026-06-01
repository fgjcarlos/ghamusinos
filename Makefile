BINARY := bin/ghamusinos

.PHONY: help build run test tidy fmt vet check web-install web-build

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

web-install: ## Instala dependencias del frontend (pnpm)
	pnpm -C web install

web-build: ## Compila el frontend y deposita assets en internal/frontend/dist
	pnpm -C web build

build: web-build ## Compila el binario en bin/ghamusinos (incluye build del frontend)
	go build -o $(BINARY) ./cmd/ghamusinos

run: ## Ejecuta la aplicación
	go run ./cmd/ghamusinos

test: ## Ejecuta los tests
	go test ./...

tidy: ## Ordena y limpia go.mod
	go mod tidy

fmt: ## Formatea el código
	go fmt ./...

vet: ## Análisis estático
	go vet ./...

check: fmt vet test ## fmt + vet + test
