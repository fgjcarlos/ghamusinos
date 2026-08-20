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

`backend` job corre, en este orden: `gofmt`, `golangci-lint`, `govulncheck`, `go test -race -coverprofile`, `go build`, smoke de migraciones contra un servicio Postgres. `frontend` job corre `typecheck` + `build`. Todas las acciones externas (`actions/checkout`, `actions/setup-go`, `actions/setup-node`, `pnpm/action-setup`, `actions/upload-artifact`) están pineadas por SHA con comentario de versión. Dependabot cubre `gomod`, `npm` y `github-actions` semanalmente.

`govulncheck` (issue #23) corre con la base de datos oficial de vulnerabilidades de Go (`vuln.go.dev`); rompe la CI si alguna dep transitiva tiene un CVE conocido con fix disponible.

## Flujo de trabajo con ramas y PRs

El repo sigue una disciplina estricta para evitar acumulación de ramas muertas:

1. **Toda cambio mergeable va por PR.** No se pushea directo a `main`.
2. **Cada rama tiene un propósito y un nombre descriptivo** (`feat/...`, `fix/...`, `chore/...`, `docs/...`). Para más detalle sobre la convención de títulos y fases, ver `docs/roadmap/issue-backlog.md`.
3. **El cuerpo del commit sigue conventional commits** (`feat:`, `fix:`, `chore:`, etc.) y referencia la issue que cierra (`Closes #N`).
4. **Auto-delete de ramas está activado** en la configuración del repo (Settings → General → Pull Requests → "Automatically delete head branches"). Cuando una PR se mergea, su rama se borra automáticamente del remoto.
5. **Ramas abandonadas** (trabajo que nunca llegó a PR) se archivan en local con el prefijo `archive/<nombre-rama>` antes de borrar la remota. Si querés resurrectir trabajo viejo, las tags `archive/stale-*` mantienen los SHAs disponibles mientras el reflog no los recolecte.

> Si abrís una rama nueva y la dejás sin abrir PR durante más de 2 semanas, es razonable cerrarla y archivar el trabajo. Mantener el remoto limpio es responsabilidad de quien abrió la rama.
