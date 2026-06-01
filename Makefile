BINARY := bin/ghamusinos

.PHONY: help build run test tidy fmt vet check

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Compila el binario en bin/ghamusinos
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
