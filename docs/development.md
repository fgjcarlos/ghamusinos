# Desarrollo local

Guía para levantar Ghamusinos en local. Stack: Go (binario único) + React/Vite embebido + PostgreSQL.

## Requisitos

| Herramienta | Versión | Notas |
|---|---|---|
| Go | 1.22 | El proyecto fija `go 1.22`. Usa `GOTOOLCHAIN=local` para evitar descargas de toolchain |
| Node | 22+ | Frontend con Vite |
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
| `make dev` | Backend con hot reload (requiere `air`: `go install github.com/air-verse/air@latest`) |
| `make run` | Ejecuta la app (`go run`) |
| `make build` | Compila el binario (depende de `web-build`) |
| `make test` | Tests Go (`go test -race`) |
| `make check` | `fmt` + `vet` + `test` |
| `make lint` | `golangci-lint run` con la config del repo (`.golangci.yml`) |
| `make coverage` | Genera `coverage.out` (artefacto de CI) |
| `make fmt` / `make vet` / `make tidy` | Formato / análisis estático / `go mod tidy` |
| `make web-install` | Instala dependencias del frontend (pnpm) |
| `make web-build` | Compila el frontend a `internal/frontend/dist` |
| `make web-dev` | Servidor de desarrollo de Vite con HMR (puerto 5173) |
| `make web-lint` | Linter y typecheck del frontend |
| `make db-up` / `make db-down` | Levanta / apaga PostgreSQL |
| `make migrate` | Aplica migraciones pendientes (`up`) |
| `make migrate-down` | Revierte la última migración (requiere `--allow-destructive`, ver #27) |
| `make migrate-status` | Muestra el estado de las migraciones |
| `make generate` | Regenera código SQLC (vía Docker) |

## Notas de toolchain

- **Go 1.22**: usa siempre `GOTOOLCHAIN=local`. Las dependencias están fijadas a versiones compatibles (chi `v5.0.14`, pgx `v5.6.0`, goose `v3.21.1`).
- **sqlc** se ejecuta como contenedor Docker (`sqlc/sqlc:1.31.1`), no por `go install`. La versión está fijada para que el código generado sea reproducible.
- **goose** se usa como librería dentro de `cmd/migrate` (no el CLI); las migraciones se embeben con `embed.FS`.
- El frontend se compila a `internal/frontend/dist` (no a `web/dist`) porque `go:embed` no admite rutas con `..`.

## TimescaleDB

La imagen base (`timescale/timescaledb:2.27.2-pg16`) trae la extensión pre-instalada pero **no la activa por defecto**. La migración `00007_timescaledb_extension.sql` la activa con `CREATE EXTENSION IF NOT EXISTS timescaledb` en el primer arranque. Esto es requisito para futuras hypertables (fase 1.4+ — `training_load_daily`). **Issue #28.**

Paridad CI ↔ local: tanto `docker-compose.yml` como el servicio Postgres de CI usan la misma imagen y tag fijo (no `latest`). Si actualizás la versión de TimescaleDB, mantené ambos en sincronía.

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
- **frontend**: `pnpm --dir web install --frozen-lockfile` + `pnpm --dir web build`.

## Flujo de trabajo con ramas y PRs

El repo sigue una disciplina estricta para evitar acumulación de ramas muertas:

1. **Toda cambio mergeable va por PR.** No se pushea directo a `main`.
2. **Cada rama tiene un propósito y un nombre descriptivo** (`feat/...`, `fix/...`, `chore/...`, `docs/...`). Para más detalle sobre la convención de títulos y fases, ver `docs/roadmap/issue-backlog.md`.
3. **El cuerpo del commit sigue conventional commits** (`feat:`, `fix:`, `chore:`, etc.) y referencia la issue que cierra (`Closes #N`).
4. **Auto-delete de ramas está activado** en la configuración del repo (Settings → General → Pull Requests → "Automatically delete head branches"). Cuando una PR se mergea, su rama se borra automáticamente del remoto.
5. **Ramas abandonadas** (trabajo que nunca llegó a PR) se archivan en local con el prefijo `archive/<nombre-rama>` antes de borrar la remota. Si querés resurrectir trabajo viejo, las tags `archive/stale-*` mantienen los SHAs disponibles mientras el reflog no los recolecte.

> Si abrís una rama nueva y la dejás sin abrir PR durante más de 2 semanas, es razonable cerrarla y archivar el trabajo. Mantener el remoto limpio es responsabilidad de quien abrió la rama.
