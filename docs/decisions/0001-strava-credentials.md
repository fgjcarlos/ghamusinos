# ADR 0001 — Modelo de credenciales de Strava

- **Estado:** aceptado
- **Fecha:** 2026-06-01
- **Contexto:** V1, integración Strava (fase 1.2)

## Decisión

**Una sola app Strava global del proyecto.** El usuario nunca ve ni gestiona `client_id`, `client_secret` ni tokens. El flujo es el estándar de la industria (como Intervals.icu o Connect): el usuario pulsa **"Conectar con Strava"**, Strava muestra su pantalla de consentimiento, autoriza a Ghamusinos a leer sus actividades, y vuelve. Nada más.

## Por qué

El modelo del código legacy era "bring your own credentials": cada usuario debía crear su propia app en Strava e introducir `client_id`/`client_secret` cifrados en la DB. Eso resolvía el aislamiento de rate limit, pero a un coste de UX inaceptable:

> El usuario medio no tiene ni idea de qué es un token o una app OAuth. En cuanto ve esos campos, abandona el proceso.

Ghamusinos es por invitación y de uso acotado: la cuota global de Strava es más que suficiente y el onboarding debe ser de un clic.

## Opciones consideradas

| Opción | UX | Rate limit | Veredicto |
|---|---|---|---|
| **A. App global** | Un clic, sin configuración | Compartido (suficiente para el volumen previsto) | **Elegida** |
| B. Por usuario (legacy) | Alta fricción, requiere crear app en Strava | Aislado por usuario | Descartada por UX |

## Consecuencias

- `STRAVA_CLIENT_ID` / `STRAVA_CLIENT_SECRET` viven en config del servidor, nunca en la UI.
- El usuario solo ve un botón "Conectar con Strava" y la pantalla de consentimiento oficial.
- Un único webhook de suscripción para toda la app.
- Los tokens **del usuario** (access/refresh) sí se guardan cifrados (AES-256-GCM) tras el consentimiento, pero son invisibles para él.
- Si algún día se alcanza el límite de rate de Strava, reevaluar hacia la opción B sin rediseño (el modelo de datos lo permite).
