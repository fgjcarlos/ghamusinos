# ADR 0003 — Preferencias de usuario en la fase 1.1

- **Estado:** aceptado
- **Fecha:** 2026-06-11
- **Contexto:** V1, fase 1.1 (base de producto y autenticación). Hallazgo 11 de
  la auditoría de coherencia: el PRD (§8) y el feature-inventory (§3) asignan a
  la fase 1.1 las preferencias iniciales del usuario (`hr_max`, `lthr`, `ftp`,
  nivel, `timezone`, IA on/off), pero la tabla `users` no tenía ninguno de esos
  campos. Docs y schema se contradecían.

## Decisión

Mantener las preferencias en la **fase 1.1** (opción A) e implementarlas ahora
como **columnas en la tabla `users`**, no en una tabla `user_preferences`
aparte.

Campos añadidos (migración `00003_user_preferences.sql`):

| Campo        | Tipo       | Nulo | Notas |
|--------------|------------|------|-------|
| `hr_max`     | SMALLINT   | sí   | FC máxima; `CHECK (>0 y <=260)` |
| `lthr`       | SMALLINT   | sí   | FC umbral láctico; `CHECK (>0 y <=260)` |
| `ftp`        | SMALLINT   | sí   | Functional Threshold Power; `CHECK (>0 y <=2000)` |
| `level`      | TEXT       | sí   | `CHECK IN (beginner, intermediate, advanced)` |
| `timezone`   | TEXT       | no   | IANA tz; `DEFAULT 'UTC'` |
| `ai_enabled` | BOOLEAN    | no   | opt-in/out de IA; `DEFAULT true` |

Lectura vía `GetUserByClerkID` (`SELECT *`); actualización vía la query SQLC
`UpdateUserPreferences`, separada de `UpdateUserProfile` (identidad vs
preferencias de entrenamiento/IA).

## Por qué

- **Coherencia docs↔schema:** el PRD y el feature-inventory ya prometen estos
  campos en 1.1; moverlos al 1.2/1.4 obligaría a reescribir ambos. Mantenerlos
  es el menor cambio y respeta el alcance ya acordado.
- **Disponibilidad temprana:** `hr_max`/`lthr`/`ftp` se necesitan como muy tarde
  en la fase 1.4 (TSS/CTL/ATL) y `ai_enabled` en la 1.5; tenerlos desde la base
  evita migraciones de datos sobre usuarios ya creados.
- **Columnas y no tabla aparte:** la relación es 1:1 y los campos son atributos
  opcionales del usuario. Columnas evitan un JOIN en cada lectura de perfil. Si
  el bloque de preferencias creciera, es extraíble a `user_preferences` sin
  romper la API (las queries ya están aisladas).

## Consecuencias

- Métricas opcionales (`NULL` = sin definir); `timezone`/`ai_enabled` siempre con
  valor por sus defaults, de modo que usuarios existentes quedan consistentes
  tras la migración.
- Los `CHECK` acotan rangos fisiológicos y el conjunto de `level`; cualquier
  taxonomía de nivel más rica se modela más adelante sin tocar identidad.
- `schema.sql` (fuente de SQLC) y las migraciones Goose se mantienen en sync
  manualmente: este cambio toca ambos.
