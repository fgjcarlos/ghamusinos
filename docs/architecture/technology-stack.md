# Stack tecnológico de Ghamusinos

Este documento define el stack tecnológico recomendado para Ghamusinos V1 y su evolución hacia V2. La decisión principal es construir una aplicación Go-first con frontend React/Vite embebido en el binario, PostgreSQL/TimescaleDB como base de datos central, jobs sobre River e integración deportiva fuerte con Strava.

El objetivo no es elegir tecnologías de moda. El objetivo es reducir superficie operativa, mantener un backend sólido para ingesta y análisis de datos deportivos, y dejar espacio para evolucionar hacia creación y gestión de entrenos en V2.

## 1. Resumen ejecutivo

Ghamusinos debe construirse como un producto de análisis deportivo con backend robusto, datos temporales, jobs fiables y una UI rica para mapas, gráficas y dashboards.

| Área | Decisión |
|---|---|
| Backend | Go |
| Router HTTP | Chi |
| Frontend | React + Vite |
| Distribución frontend | Embebido en binario Go con `embed.FS` |
| Base de datos | PostgreSQL + TimescaleDB |
| Queries | SQLC |
| Migraciones | Goose |
| Jobs/background | River sobre PostgreSQL |
| Auth | Clerk inicialmente |
| Integración deportiva | Strava OAuth + webhooks + backfill + deduplicación |
| IA | Multi-proveedor opcional (OpenAI → Claude → OpenRouter) |
| Mapas | MapLibre |
| Gráficas | ECharts |
| Deploy | Binario único + PostgreSQL/TimescaleDB |

La razón central: Ghamusinos V1 depende más de ingesta, procesamiento, consistencia de datos, cálculos y jobs que de renderizado web tradicional. Go encaja mejor que una arquitectura NestJS + Astro separada porque reduce complejidad operativa, genera binarios simples, ofrece buen rendimiento para procesamiento GPX y mantiene el sistema fácil de desplegar.

## 2. Principios técnicos

1. Simplicidad operativa antes que arquitectura distribuida.
2. Un solo backend dueño de datos, jobs, API y frontend embebido.
3. SQL explícito para queries críticas.
4. PostgreSQL como núcleo transaccional y de background jobs.
5. TimescaleDB solo donde aporte valor temporal real.
6. Frontend interactivo donde importa: mapas, gráficas, dashboards y análisis.
7. Integraciones externas tratadas como sistemas no fiables.
8. Procesamiento idempotente para GPX, Strava, webhooks y backfills.
9. IA como mejora opcional, no como dependencia del core.
10. Evolución hacia V2 sin rediseñar toda la base.

## 3. Arquitectura de alto nivel

```text
┌──────────────────────────────────────────────┐
│                 Browser                      │
│        React + Vite + MapLibre + ECharts      │
└───────────────────────┬──────────────────────┘
                        │ HTTPS
                        ▼
┌──────────────────────────────────────────────┐
│                Go Binary                     │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ embed.FS                               │  │
│  │ Static React/Vite assets               │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ Chi HTTP Router                        │  │
│  │ API, auth middleware, webhooks         │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ Application Services                   │  │
│  │ GPX analysis, Strava sync, dashboards  │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ River Workers                          │  │
│  │ imports, backfills, dedup, AI jobs     │  │
│  └────────────────────────────────────────┘  │
└───────────────────────┬──────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────┐
│          PostgreSQL + TimescaleDB             │
│  users, activities, metrics, jobs, tokens      │
└──────────────────────────────────────────────┘

External services:
- Clerk: authentication and identity
- Strava: OAuth, activities, webhooks
- Claude API: optional AI insights
```

## 4. Por qué Go encaja mejor que NestJS + Astro separado

Ghamusinos no es principalmente una web de contenido. Es un producto de datos deportivos.

V1 necesita:

- Subida y análisis de GPX.
- Ingesta desde Strava.
- Webhooks fiables.
- Backfills.
- Deduplicación.
- Cálculos de rendimiento, salud y fatiga.
- Jobs en background.
- API para dashboards.
- Deploy sencillo.

Go encaja especialmente bien aquí porque:

| Necesidad | Por qué Go encaja |
|---|---|
| Procesar GPX | Buen rendimiento, bajo consumo, parsing eficiente |
| Jobs concurrentes | Goroutines, contexto, cancelación y workers simples |
| Deploy | Binario único, fácil de empaquetar y operar |
| API estable | `net/http` + Chi son suficientes sin framework grande |
| SQL explícito | SQLC genera tipos Go desde SQL real |
| Menor complejidad | Una app sirve API, assets y workers |

NestJS sería razonable si el equipo necesitara un ecosistema Node empresarial, decorators, DI pesada o una organización muy TypeScript-first. Para Ghamusinos añade capas que no resuelven el problema principal.

Astro separado sería razonable si el producto fuera principalmente marketing, contenido estático o SSR público. Pero Ghamusinos requiere una SPA autenticada, dashboards interactivos, mapas y gráficas. Separar Astro de un backend Go aumentaría despliegue y coordinación sin aportar mucho a V1.

## 5. Estructura de repositorio propuesta

```text
ghamusinos/
├── cmd/
│   └── ghamusinos/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── server.go
│   │   └── config.go
│   ├── auth/
│   │   └── clerk.go
│   ├── http/
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── handlers/
│   ├── strava/
│   │   ├── oauth.go
│   │   ├── webhooks.go
│   │   ├── client.go
│   │   └── sync.go
│   ├── gpx/
│   │   ├── parser.go
│   │   └── analysis.go
│   ├── activities/
│   │   ├── service.go
│   │   └── dedup.go
│   ├── metrics/
│   │   ├── fatigue.go
│   │   ├── performance.go
│   │   └── health.go
│   ├── ai/
│   │   └── claude.go
│   ├── jobs/
│   │   ├── river.go
│   │   ├── import_strava.go
│   │   ├── process_gpx.go
│   │   └── generate_insights.go
│   ├── db/
│   │   ├── queries/
│   │   ├── sqlc/
│   │   └── migrations/
│   └── frontend/
│       └── embed.go
├── web/
│   ├── src/
│   │   ├── app/
│   │   ├── pages/
│   │   ├── features/
│   │   ├── components/
│   │   ├── maps/
│   │   ├── charts/
│   │   └── api/
│   ├── index.html
│   ├── vite.config.ts
│   └── package.json
├── sqlc.yaml
├── goose.yaml
├── go.mod
├── Makefile
└── docs/
    └── architecture/
        └── technology-stack.md
```

## 6. Flujo de build

```text
1. Compilar frontend
   web/ React + Vite → web/dist/

2. Embeber assets
   Go embed.FS incluye web/dist/

3. Compilar backend
   Go genera binario único

4. Ejecutar migraciones
   Goose aplica cambios sobre PostgreSQL/TimescaleDB

5. Arrancar proceso
   El binario sirve:
   - API HTTP
   - SPA estática
   - Strava webhooks
   - workers River
```

Comandos esperados:

```bash
make web-build
make generate
make build
make migrate
./bin/ghamusinos
```

## 7. Decisiones por capa

### Backend

Go será el lenguaje principal del backend.

Responsabilidades:

- API HTTP.
- Autenticación y autorización.
- Ingesta GPX.
- Integración Strava.
- Procesamiento de métricas.
- Jobs River.
- Servir frontend embebido.
- Integración opcional con Claude API.

Chi será el router HTTP porque es pequeño, composable y no intenta imponer una arquitectura completa.

Evitar:

- Frameworks pesados innecesarios.
- ORMs mágicos.
- Separar workers en otro runtime durante V1.
- Microservicios prematuros.

### Frontend

React + Vite será la capa de UI.

Responsabilidades:

- Dashboard de rendimiento.
- Visualización de actividad.
- Mapas de rutas.
- Gráficas de series temporales.
- Estados de sincronización Strava.
- Experiencia de análisis GPX.
- V2: creación y gestión de entrenos.

Vite genera assets estáticos que Go sirve desde `embed.FS`.

Una SPA embebida no significa acoplar mal el frontend. Significa simplificar distribución. La separación lógica debe mantenerse en `web/`.

### Base de datos

PostgreSQL será la base de datos principal.

TimescaleDB se usará para datos temporales donde tenga sentido:

- Series de puntos de actividad.
- Métricas por tiempo.
- Cargas de entrenamiento.
- Agregados de rendimiento.
- Datos de salud/fatiga si tienen granularidad temporal.

No todo debe ser hypertable. Tablas de usuarios, tokens, actividades, permisos, jobs y relaciones normales deben seguir siendo tablas PostgreSQL convencionales.

### Queries

SQLC generará código Go tipado desde SQL.

Ventajas:

- SQL real.
- Tipos generados.
- Menos runtime magic.
- Queries revisables.
- Mejor control sobre rendimiento.

Esto obliga a pensar el modelo de datos. Eso es bueno. En productos analíticos, esconder SQL demasiado pronto suele salir caro.

### Migraciones

Goose gestionará migraciones SQL.

Reglas recomendadas:

- Una migración por cambio lógico.
- Migraciones pequeñas.
- SQL explícito.
- No depender de migraciones generadas automáticamente por un ORM.
- Separar cambios destructivos en pasos seguros si hay datos reales.

### Jobs y background processing

River ejecutará jobs usando PostgreSQL.

Casos de uso:

- Importar actividades desde Strava.
- Ejecutar backfill.
- Procesar GPX.
- Deduplicar actividades.
- Recalcular métricas.
- Generar insights con Claude.
- Reintentar llamadas externas fallidas.

Ventaja central: no se añade Redis, RabbitMQ ni otro sistema en V1.

River comparte base de datos con el producto. Hay que controlar:

- Tamaño de colas.
- Reintentos.
- Jobs idempotentes.
- Transacciones.
- Índices.
- Retención de jobs históricos.
- Observabilidad.

### Autenticación

Clerk será la solución inicial de auth.

Responsabilidades delegadas:

- Login.
- Registro.
- Sesiones.
- Gestión básica de identidad.
- Proveedores externos si aplica.

Advertencias realistas con Go:

- Clerk está muy orientado a ecosistema frontend/Node.
- En Go habrá que validar JWTs correctamente.
- Hay que mapear identidad Clerk a usuarios internos.
- El `user_id` interno no debería depender ciegamente de detalles cambiantes del proveedor.
- Webhooks de Clerk pueden ser necesarios para sincronizar usuarios.
- Hay que diseñar salida futura si Clerk deja de encajar.

Modelo recomendado:

```text
Clerk user ID → identity externa
Ghamusinos user ID → identidad interna del dominio
```

### Strava

Strava será la integración deportiva principal para V1.

Componentes necesarios:

- OAuth.
- Almacenamiento seguro de tokens.
- Refresh tokens.
- Webhooks.
- Backfill inicial.
- Sincronización incremental.
- Deduplicación.
- Rate limit handling.
- Reintentos.

La ingesta Strava debe ser idempotente. El sistema debe poder recibir el mismo evento varias veces sin duplicar actividades ni corromper métricas.

Deduplicación recomendada:

- `strava_activity_id` cuando exista.
- Hash de GPX o fingerprint de actividad para imports manuales.
- Comparación por tiempo, distancia, duración y usuario cuando no haya ID externo fiable.

### GPX

El almacenamiento GPX debe decidirse con cuidado.

| Opción | Ventaja | Riesgo |
|---|---|---|
| Guardar GPX completo en DB | Simplicidad, backups únicos | Puede inflar PostgreSQL rápidamente |
| Guardar GPX en object storage | Escalable para ficheros grandes | Añade infraestructura |
| Guardar solo datos normalizados | Consultas rápidas | Se pierde fuente original si no se conserva |
| Modelo híbrido | Mejor equilibrio | Más diseño inicial |

Para V1, una opción pragmática es:

- Guardar metadata y datos normalizados en PostgreSQL.
- Guardar el GPX original de forma controlada.
- Si el volumen crece, mover originales a object storage.
- Calcular y guardar hash del fichero para deduplicación.

### IA

La IA será **opcional y multi-proveedor**, tras una interfaz común en `internal/ai`. Orden de implementación: OpenAI → Claude → OpenRouter (ver `docs/decisions/0002-ai-provider.md`).

Casos de uso:

- Resúmenes de actividad.
- Insights de rendimiento.
- Explicación de fatiga.
- Sugerencias no prescriptivas.
- Preparación futura para entrenos en V2.

La IA no debe ser fuente de verdad. Debe consumir métricas calculadas por el sistema y producir texto o recomendaciones auxiliares.

Reglas:

- El dominio no conoce el proveedor; se elige por configuración (`AI_PROVIDER`, `AI_MODEL`).
- No bloquear flujos críticos si el proveedor de IA falla.
- Ejecutar generación en jobs.
- Schema de salida validado e idéntico para todos los proveedores.
- Guardar prompts/versiones si afectan resultados visibles.
- Permitir desactivar IA por configuración (flag global + opt-in por usuario).

### Mapas

MapLibre será la librería de mapas.

Motivos:

- Open source.
- Flexible.
- No ata el producto a un proveedor único.
- Buen encaje con rutas GPX y visualización deportiva.

Hay que decidir proveedor de tiles. MapLibre es la librería, no el proveedor de mapas.

### Gráficas

ECharts será la librería de visualización.

Motivos:

- Soporta series temporales complejas.
- Buen rendimiento para dashboards.
- Flexible para overlays, zoom, tooltip y comparativas.
- Adecuada para fatiga, carga, ritmo, frecuencia cardiaca y evolución.

Hay que evitar que las gráficas se conviertan en lógica de dominio. La UI visualiza; el backend calcula.

## 8. Deploy

Deploy recomendado para V1:

```text
1 binario Go
1 PostgreSQL con TimescaleDB
Variables de entorno
TLS vía reverse proxy o plataforma
```

Ventajas:

- Menos piezas.
- Menos fallos operativos.
- Más fácil de mover entre entornos.
- Mejor para una V1 que todavía valida producto.

No se recomienda separar frontend, backend y workers en servicios distintos al inicio salvo que haya una necesidad real.

## 9. Tradeoffs

| Decisión | Beneficio | Coste |
|---|---|---|
| Go en vez de NestJS | Simplicidad, rendimiento, binario único | Menos ergonomía full-stack TypeScript |
| Chi en vez de framework grande | Control y bajo acoplamiento | Hay que definir estructura propia |
| React SPA embebida | Deploy simple | Hay que cuidar rutas, cache y fallback |
| PostgreSQL + TimescaleDB | Relacional + temporal | Operación algo más exigente que Postgres puro |
| SQLC | SQL tipado y explícito | Menos velocidad inicial que ORM |
| Goose | Migraciones simples | Disciplina manual |
| River | Jobs sin infraestructura extra | Carga adicional sobre PostgreSQL |
| Clerk | Auth rápida | Dependencia externa y adaptación en Go |
| Claude opcional | Valor añadido | Coste, latencia y control de calidad |
| MapLibre | Flexibilidad | Hay que resolver tiles |
| ECharts | Gráficas potentes | Bundle y complejidad visual |

## 10. Alternativas descartadas

### NestJS

Descartado como backend principal para V1.

No es una mala tecnología. Simplemente no encaja tan bien con el centro de gravedad del producto.

Razones:

- Añade runtime Node separado.
- Favorece una arquitectura más pesada.
- No aporta ventaja clara para procesamiento GPX.
- Complica el modelo de binario único.
- SQLC, River y `embed.FS` encajan mejor en Go.

NestJS sería más competitivo si Ghamusinos fuera una plataforma TypeScript-first con equipo grande, muchas integraciones web empresariales y necesidad fuerte de DI/decorators.

### Astro como frontend separado

Descartado como pieza principal separada.

Razones:

- Ghamusinos requiere SPA autenticada e interactiva.
- Mapas, dashboards y gráficas viven mejor en React directo.
- Astro aporta más en contenido, marketing, documentación o SSR.
- Separarlo aumenta complejidad de deploy.

Astro podría usarse en el futuro para una web pública de marketing o documentación, pero no como núcleo de la app V1.

### ORM tradicional

Descartado en favor de SQLC.

Razones:

- El producto depende de queries analíticas.
- SQL explícito será importante.
- TimescaleDB requiere control.
- Evitar abstracciones que escondan rendimiento.

### Cola externa tipo Redis/RabbitMQ

Descartada inicialmente en favor de River.

Razones:

- Añade otra infraestructura.
- River cubre bien jobs persistentes para V1.
- PostgreSQL ya es dependencia obligatoria.

Podría reconsiderarse si hay alto volumen, necesidades de streaming, fanout complejo o separación fuerte de workloads.

## 11. Riesgos

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Clerk no encaja perfectamente con Go | Integración más manual | Validar JWT bien, aislar auth en `internal/auth`, mantener usuario interno |
| SPA embebida mal cacheada | Bugs tras deploy | Versionar assets, configurar cache headers y fallback correctamente |
| River sobrecarga PostgreSQL | Latencia o bloqueo | Índices, límites de concurrencia, colas separadas y observabilidad |
| TimescaleDB usado de más | Complejidad innecesaria | Usar hypertables solo para series temporales reales |
| GPX infla la base de datos | Coste y backups grandes | Hash, normalización y posible object storage |
| Strava rate limits | Sync incompleto | Backoff, jobs reintentables y backfill controlado |
| Webhooks duplicados o perdidos | Datos inconsistentes | Idempotencia, backfill periódico y deduplicación |
| Claude genera respuestas pobres | Mala confianza de usuario | IA opcional, explicabilidad y métricas determinísticas como fuente |
| Map tiles tienen coste/límites | Mapas fallan o salen caros | Elegir proveedor temprano y abstraer configuración |
| Bundle frontend crece demasiado | Carga lenta | Code splitting y revisar dependencias pesadas |

## 12. Decisiones abiertas

| Tema | Decisión pendiente |
|---|---|
| Hosting | Plataforma concreta para binario y Postgres/TimescaleDB |
| Object storage | Decidir si GPX original vive en DB o storage externo |
| Proveedor de tiles | Elegir proveedor compatible con MapLibre |
| Modelo de usuario | Definir relación exacta entre Clerk user y usuario interno |
| Retención de jobs | Política de limpieza de jobs River |
| Observabilidad | Logs, métricas, trazas y alertas mínimas |
| Backfill Strava | Límites, frecuencia y estrategia por usuario |
| IA | Qué features usan Claude en V1 y cuáles esperan a V2 |
| Timescale schema | Qué tablas serán hypertables |
| V2 entrenos | Modelo inicial para workouts, bloques, planificación y cumplimiento |

## 13. Checklist de implementación

### Base del proyecto

- [ ] Crear módulo Go.
- [ ] Crear estructura `cmd/ghamusinos`.
- [ ] Crear configuración central.
- [ ] Crear servidor HTTP con Chi.
- [ ] Crear healthcheck.
- [ ] Crear manejo de shutdown graceful.

### Frontend

- [ ] Crear app React + Vite en `web/`.
- [ ] Configurar build a `web/dist/`.
- [ ] Embeber `web/dist/` con `embed.FS`.
- [ ] Servir assets desde Go.
- [ ] Configurar fallback SPA.
- [ ] Definir cliente API frontend.

### Base de datos

- [ ] Configurar PostgreSQL.
- [ ] Activar TimescaleDB.
- [ ] Configurar Goose.
- [ ] Crear migraciones iniciales.
- [ ] Configurar SQLC.
- [ ] Crear queries iniciales.
- [ ] Definir convenciones de transacciones.

### Auth

- [ ] Configurar Clerk.
- [ ] Validar JWT en Go.
- [ ] Crear tabla de usuarios internos.
- [ ] Mapear Clerk user ID a usuario interno.
- [ ] Proteger rutas privadas.
- [ ] Decidir manejo de webhooks Clerk.

### Strava

- [ ] Implementar OAuth.
- [ ] Guardar tokens cifrados o protegidos.
- [ ] Implementar refresh token.
- [ ] Crear endpoint de webhooks.
- [ ] Crear job de importación.
- [ ] Crear job de backfill.
- [ ] Implementar deduplicación.
- [ ] Gestionar rate limits.

### GPX

- [ ] Implementar parser GPX.
- [ ] Extraer metadata.
- [ ] Normalizar puntos relevantes.
- [ ] Calcular hash de fichero.
- [ ] Definir almacenamiento del original.
- [ ] Crear job de procesamiento.
- [ ] Crear estrategia de errores.

### Métricas y dashboard

- [ ] Definir métricas V1.
- [ ] Crear queries de actividad.
- [ ] Crear agregados temporales.
- [ ] Implementar dashboard base.
- [ ] Integrar ECharts.
- [ ] Integrar MapLibre.
- [ ] Separar cálculo backend de visualización frontend.

### Jobs

- [ ] Configurar River.
- [ ] Definir colas.
- [ ] Definir concurrencia.
- [ ] Implementar retries.
- [ ] Garantizar idempotencia.
- [ ] Añadir limpieza de jobs antiguos.
- [ ] Añadir logs por job.

### IA opcional

- [ ] Crear cliente Claude.
- [ ] Definir feature flags.
- [ ] Ejecutar generación vía River.
- [ ] Guardar resultado de insights.
- [ ] Manejar fallos sin romper flujos principales.

### Deploy

- [ ] Crear `Makefile`.
- [ ] Crear build frontend.
- [ ] Crear build Go.
- [ ] Crear comando de migraciones.
- [ ] Definir variables de entorno.
- [ ] Documentar arranque local.
- [ ] Documentar despliegue V1.
- [ ] Añadir backup/restore de PostgreSQL.

## 14. Recomendación final

La arquitectura recomendada para Ghamusinos V1 es:

```text
Go + Chi
React + Vite embebido con embed.FS
PostgreSQL + TimescaleDB
SQLC + Goose
River para jobs
Clerk para auth inicial
Strava como integración deportiva principal (app global, OAuth de un clic)
IA opcional multi-proveedor (OpenAI → Claude → OpenRouter)
MapLibre + ECharts para experiencia visual
Deploy como binario único + base de datos
```

Esta decisión es coherente con el producto: primero datos deportivos, ingesta fiable, análisis GPX, dashboards y salud/fatiga; después, en V2, creación y gestión de entrenos.

La clave es no sobrediseñar. Ghamusinos necesita cimientos sólidos, no una urbanización entera antes de tener la primera casa habitable.
