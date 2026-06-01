# Plan V1 — Fase 1.1: Base de producto, arquitectura inicial y autenticación

Esta fase crea los cimientos técnicos de Ghamusinos. El objetivo no es implementar todavía Strava, GPX ni métricas avanzadas, sino dejar una aplicación mínima, ejecutable y extensible sobre la que construir el resto de V1.

## 1. Objetivo

Construir la base fullstack de Ghamusinos con:

- Backend Go con Chi.
- Frontend React + Vite embebido en el binario Go.
- PostgreSQL preparado.
- Migraciones con Goose.
- Queries tipadas con SQLC.
- Autenticación con Clerk.
- Usuario interno de dominio.
- Acceso por invitación.
- DX mínima para desarrollo local.

La fase termina cuando un usuario autenticado y autorizado puede entrar a una pantalla privada básica servida desde el binario Go.

## 2. Por qué esta fase va primero

Antes de construir análisis GPX, Strava o dashboards hace falta una base estable.

Sin esta fase, cualquier funcionalidad posterior arrastraría deuda en:

- estructura del repositorio,
- configuración,
- autenticación,
- migraciones,
- generación SQL,
- separación backend/frontend,
- modelo de usuario,
- control de acceso.

Esta fase es la losa de hormigón. No se ve tanto como la fachada, pero si está mal, todo lo demás se mueve.

## 3. Alcance

### Incluido

| Área | Entregable |
|---|---|
| Proyecto | Estructura base del repositorio |
| Backend | Servidor Go con Chi |
| Frontend | React + Vite mínimo |
| Embebido | Assets del frontend servidos desde Go con `embed.FS` |
| Config | Configuración por variables de entorno |
| DB | Conexión a PostgreSQL |
| Migraciones | Goose configurado |
| Queries | SQLC configurado |
| Auth | Validación de sesión/JWT de Clerk |
| Usuario | Tabla interna `users` |
| Invitaciones | Modelo inicial de invitaciones |
| Seguridad | Rutas públicas y privadas separadas |
| DX | Makefile o comandos equivalentes |
| Docs | README básico de arranque local |

### Fuera de alcance

| No incluido | Motivo |
|---|---|
| OAuth Strava | Fase 1.2 |
| Webhooks Strava | Fase 1.2 |
| Backfill de actividades | Fase 1.2 |
| Parsing GPX | Fase 1.3 |
| Métricas de ruta | Fase 1.3 |
| Dashboard real | Fase 1.4 |
| Claude/IA | Fase 1.5 |
| Entrenos/calendario | V2 |

## 4. Entregables técnicos

### 4.1 Estructura inicial

Estructura esperada:

```text
ghamusinos/
├── cmd/
│   └── ghamusinos/
│       └── main.go
├── internal/
│   ├── app/
│   ├── auth/
│   ├── config/
│   ├── db/
│   ├── frontend/
│   └── http/
├── web/
│   ├── src/
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── docs/
├── sqlc.yaml
├── goose.yaml
├── go.mod
└── Makefile
```

### 4.2 Backend Go

Debe incluir:

- `cmd/ghamusinos/main.go` como entrypoint.
- Configuración centralizada.
- Servidor HTTP con Chi.
- Middleware base.
- Graceful shutdown.
- Endpoint público `GET /healthz`.
- Grupo de rutas API bajo `/api`.
- Grupo de rutas privadas protegido por auth.

### 4.3 Frontend React + Vite embebido

Debe incluir:

- App React mínima.
- Build con Vite hacia `web/dist`.
- Embebido con `embed.FS`.
- Go sirviendo assets estáticos.
- Fallback para SPA.
- Pantalla pública básica.
- Pantalla privada básica.

### 4.4 Base de datos

Debe incluir:

- Conexión a PostgreSQL.
- Configuración por `DATABASE_URL`.
- Migraciones Goose.
- SQLC conectado a queries SQL.
- Healthcheck opcional de DB.

### 4.5 Modelo inicial de usuario

Tabla sugerida: `users`.

Campos mínimos:

| Campo | Descripción |
|---|---|
| `id` | ID interno de Ghamusinos |
| `clerk_user_id` | ID externo de Clerk |
| `email` | Email principal |
| `display_name` | Nombre visible opcional |
| `invite_status` | `pending`, `active`, `blocked` |
| `created_at` | Fecha de creación |
| `updated_at` | Fecha de actualización |

Regla importante:

> Clerk identifica al usuario externamente; Ghamusinos mantiene su propio usuario interno de dominio.

### 4.6 Modelo inicial de invitaciones

Tabla sugerida: `invites`.

Campos mínimos:

| Campo | Descripción |
|---|---|
| `id` | ID de invitación |
| `email` | Email invitado |
| `token_hash` | Hash del token de invitación |
| `status` | `pending`, `accepted`, `revoked`, `expired` |
| `expires_at` | Fecha de expiración |
| `accepted_at` | Fecha de aceptación |
| `created_at` | Fecha de creación |

Para esta fase no hace falta construir un sistema completo de administración de invitaciones. Basta con el modelo y el flujo mínimo para bloquear acceso sin invitación válida.

### 4.7 Autenticación con Clerk

Debe incluir:

- Validación de token/sesión de Clerk.
- Middleware de autenticación.
- Resolución o creación controlada de usuario interno.
- Bloqueo de usuario sin invitación activa.
- Separación entre rutas públicas y privadas.

Advertencia:

> No conviene contaminar el dominio con Clerk. Clerk debe vivir aislado en `internal/auth` o equivalente.

## 5. Criterios de aceptación

La fase se considera terminada cuando:

- [ ] El proyecto compila.
- [ ] El servidor Go arranca correctamente.
- [ ] `GET /healthz` responde correctamente.
- [ ] El frontend React carga desde el servidor Go.
- [ ] El build de frontend queda embebido en el binario.
- [ ] Existe conexión funcional a PostgreSQL.
- [ ] Goose ejecuta migraciones.
- [ ] SQLC genera código desde queries SQL.
- [ ] Existe tabla `users`.
- [ ] Existe tabla `invites`.
- [ ] Una ruta pública es accesible sin sesión.
- [ ] Una ruta privada exige autenticación.
- [ ] Un usuario autenticado con Clerk se mapea a usuario interno.
- [ ] Un usuario sin invitación activa no accede al área privada.
- [ ] Un usuario con invitación activa accede al área privada básica.
- [ ] Hay comandos documentados para desarrollo local.

## 6. Orden de implementación recomendado

1. Crear módulo Go y estructura base.
2. Crear servidor HTTP con Chi.
3. Añadir endpoint `GET /healthz`.
4. Crear app React + Vite mínima.
5. Configurar build del frontend.
6. Embeber `web/dist` con `embed.FS`.
7. Servir SPA desde Go.
8. Crear configuración central.
9. Conectar PostgreSQL.
10. Configurar Goose.
11. Crear migración inicial de `users` e `invites`.
12. Configurar SQLC.
13. Crear queries mínimas de usuario e invitación.
14. Integrar validación Clerk.
15. Crear middleware de rutas privadas.
16. Implementar resolución de usuario interno.
17. Implementar bloqueo por invitación.
18. Crear README de desarrollo local.

## 7. Riesgos de la fase

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Integración Clerk en Go más manual de lo esperado | Medio | Aislar en `internal/auth` y validar JWT de forma explícita |
| SPA embebida con rutas rotas | Medio | Configurar fallback correctamente |
| Configuración local pesada | Medio | Makefile y `.env.example` desde el inicio |
| Modelo de invitaciones sobrediseñado | Bajo | Implementar solo lo mínimo para V1 |
| SQLC ralentiza arranque inicial | Bajo | Empezar con pocas queries críticas |

## 8. Decisiones abiertas

| Tema | Decisión pendiente |
|---|---|
| Provider local de PostgreSQL | Docker Compose, instancia local o servicio externo |
| Estrategia de sesiones frontend | Clerk React SDK, token explícito o integración híbrida |
| Formato de IDs internos | UUID, ULID o BIGSERIAL |
| Gestión inicial de invitaciones | Seeds manuales, CLI o endpoint admin temporal |
| Estructura exacta de Makefile | Comandos definitivos de desarrollo |

## 9. Resultado esperado

Al terminar esta fase, Ghamusinos tendrá una base ejecutable:

```text
Usuario invitado → login con Clerk → usuario interno → área privada básica
```

Y una base técnica preparada para la siguiente fase:

```text
Fase 1.2 → Ingesta Strava: OAuth, tokens, webhooks, backfill y lista de actividades
```

## 10. Siguiente fase

La siguiente fase será:

> **Fase 1.2 — Ingesta Strava**

Objetivo:

- Conectar Strava por OAuth.
- Persistir tokens.
- Importar actividades recientes.
- Configurar webhooks.
- Implementar backfill acotado.
- Deduplicar actividades.
- Mostrar lista/carrusel de actividades.
