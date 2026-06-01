# ADR 0001 — Modelo de credenciales de Strava

- **Estado:** propuesto (pendiente de decisión)
- **Fecha:** 2026-06-01
- **Contexto:** V1, integración Strava (fase 1.2)

## Decisión a tomar

Cómo se obtienen las credenciales de la aplicación Strava (`client_id` / `client_secret`) que el sistema usa para OAuth, webhooks y backfill.

## Opciones

| Opción | Cómo funciona | Pro | Contra |
|---|---|---|---|
| **A. App global** | Una sola app Strava del proyecto; credenciales en variables de entorno del servidor | Onboarding sin fricción; el usuario solo pulsa "Conectar" | Cuota de rate limit compartida entre todos los usuarios; un único webhook |
| **B. Por usuario (legacy)** | Cada usuario registra su propia app Strava; credenciales cifradas en DB (AES-256-GCM) | Rate limit aislado por usuario; multi-tenant real | Fricción alta de onboarding; el usuario debe crear una app en Strava |

## Recomendación

**Opción A (app global)** para V1. Ghamusinos es por invitación y de uso personal/acotado: la cuota global de Strava es suficiente y el onboarding debe ser de un clic. El modelo por usuario del código legacy resolvía un problema de escala que V1 no tiene.

Mantener la columna cifrada de credenciales en el modelo de datos permite migrar a la opción B más adelante sin rediseño.

## Consecuencias

- `STRAVA_CLIENT_ID` / `STRAVA_CLIENT_SECRET` viven en config del servidor.
- Un único webhook de suscripción para toda la app.
- Si se alcanza el límite de rate de Strava, reevaluar hacia la opción B.
