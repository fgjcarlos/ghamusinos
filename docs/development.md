# Desarrollo local

Guía para levantar Ghamusinos en local. Stack: Go (binario único) + React/Vite embebido + PostgreSQL.

## Requisitos

| Herramienta | Versión | Notas |
|---|---|---|
| Go | 1.22 | El proyecto fija `go 1.22`. Usa `GOTOOLCHAIN=local` para evitar descargas de toolchain |
| Node | 20+ | Frontend con Vite |
| pnpm | 10+ | Gestor de paquetes del frontend |
| Docker | — | PostgreSQL local vía `docker-compose` |
| sqlc | — | Se ejecuta vía Docker (`make generate`), no requiere instalación |

## Quick path

```bash
cp .env.example .env           # edita POSTGRES_PASSWORD (obligatorio) y DATABASE_URL
make db-up                     # PostgreSQL + TimescaleDB en Docker
make migrate                   # aplica migraciones
make web-build                 # compila el frontend a internal/frontend/dist
make build                     # compila el binario (embebe el frontend)
./bin/ghamusinos               # arranca: API + SPA en http://localhost:8080
```

> **POSTGRES_PASSWORD es obligatorio.** Sin él `docker compose up` falla con error explícito.
> Elige cualquier contraseña para desarrollo local; en staging/prod usa `openssl rand -base64 32`.

Healthcheck: `curl http://localhost:8080/healthz` → `{"status":"ok"}`.

> Si el puerto 8080 está ocupado, arranca con `PORT=8099 ./bin/ghamusinos`.

## Comandos (Makefile)

| Comando | Qué hace |
|---|---|
| `make help` | Lista los targets disponibles |
| `make run` | Ejecuta la app (`go run`) |
| `make build` | Compila el binario (depende de `web-build`) |
| `make test` | Tests Go (`go test ./...`) |
| `make check` | `fmt` + `vet` + `test` |
| `make web-install` | Instala dependencias del frontend |
| `make web-build` | Compila el frontend a `internal/frontend/dist` |
| `make generate` | Regenera el código SQLC (vía Docker) |
| `make db-up` / `make db-down` | Levanta / apaga PostgreSQL |
| `make migrate` / `make migrate-status` | Aplica / muestra estado de migraciones |

## Notas de toolchain

- **Go 1.22**: usa siempre `GOTOOLCHAIN=local`. Las dependencias están fijadas a versiones compatibles (chi `v5.0.14`, pgx `v5.6.0`, goose `v3.21.1`).
- **sqlc** se ejecuta como contenedor Docker (`sqlc/sqlc:1.31.1`), no por `go install`. La versión está fijada para que el código generado sea reproducible.
- **goose** se usa como librería dentro de `cmd/migrate` (no el CLI); las migraciones se embeben con `embed.FS`.
- El frontend se compila a `internal/frontend/dist` (no a `web/dist`) porque `go:embed` no admite rutas con `..`.

## Variables de entorno

Ver `.env.example`. Las principales:

| Variable | Descripción |
|---|---|
| `ENV` | `development` / `production` |
| `PORT` | Puerto HTTP (default 8080) |
| `DATABASE_URL` | Cadena de conexión PostgreSQL (obligatoria) |
| `POSTGRES_USER` | Usuario de la DB (default: `ghamusinos`) |
| `POSTGRES_PASSWORD` | Contraseña de la DB — **sin valor por defecto, obligatoria** |
| `POSTGRES_DB` | Nombre de la DB (default: `ghamusinos`) |

> En staging/prod genera contraseñas aleatorias: `openssl rand -base64 32`.
> El puerto 5432 está vinculado a `127.0.0.1` para evitar exposición en redes externas.

## CI

El workflow `.github/workflows/ci.yml` corre en cada push a `main` y en cada PR:

- **backend**: `gofmt`, `go vet`, `go test`, `go build` y un smoke de migraciones contra un PostgreSQL de servicio.
- **frontend**: `pnpm install` + `pnpm build`.
