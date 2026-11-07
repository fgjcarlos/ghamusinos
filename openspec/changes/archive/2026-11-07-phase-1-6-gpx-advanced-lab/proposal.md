# Proposal: phase-1-6-gpx-advanced-lab

> Cierre de la issue epic #18 ("epic: Fase 1.6 — Laboratorio GPX
> avanzado"). Objetivo de la issue, verbatim:
> **"Planificación de carrera y contexto ambiental (ya prototipado en
> old_ghamusinos)."**
>
> Entregables de la issue, verbatim:
> - "Race Day (tiempo, ritmo, nutrición, hitos KM)"
> - "Cutoff Calculator + Strategic Splits"
> - "Terrain Info (OSM/Overpass), Weather (Open-Meteo), Solar Exposure (SunCalc)"
> - "Gear Checklist + Post-Activity (plan vs real)"
>
> Criterio de cierre del roadmap, verbatim:
> **"el laboratorio ofrece preparación de carrera completa sobre un track."**
>
> Base: `main@2725d2e`. Rama integradora: `feat/phase-1-6-gpx-lab`.
> Entrega confirmada por el usuario: **cadena sobre rama de feature**
> (seis PRs hijas + PR tracker en borrador). El trabajo local suma
> 7.113 líneas añadidas y 4 eliminadas en 63 ficheros.

## Intent

El laboratorio GPX ya analiza un track: métricas, perfil de elevación,
climbs, muros, zonas de riesgo y comparación de rutas. Lo que faltaba era
**usar ese análisis para planificar una carrera**: estimar el tiempo, repartir
el esfuerzo, comprobar cortes horarios, preparar nutrición y equipo, y
contrastar el plan con lo que realmente ocurrió.

El prototipo `old_ghamusinos` ya tenía esos cálculos, pero concentrados en
un `utils.ts` de 1.669 líneas, con `any`, sin tests para la mayoría de los
modos, con `localStorage` y llamadas directas del navegador a Open-Meteo y
Overpass. Copiarlo no era aceptable: la SPA se sirve con CSP
`connect-src 'self'`, de modo que esas llamadas no funcionarían en
producción, y el prototipo usa una convención de coordenadas posicional
(`[lng, lat, ele]`) distinta de la del backend (`{lat, lon, ele}`).

La fase, por tanto, no es "portar el prototipo" sino **extraer un modelo puro
y tipado**, **mover las integraciones externas al backend autenticado** y
**montar una superficie de planificación sobre el track ya guardado**.

## What changes

### Backend

- La ingesta de Strava pide y persiste el stream `time` además de
  `heartrate`, `watts`, `cadence`, `altitude` y `latlng`.
- **Se corrige un fallo preexistente**: el worker `ImportStravaStreamsWorker`
  estaba registrado pero nada lo encolaba, así que `activity_streams` quedaba
  vacío en producción y con él se degradaban también las zonas de FC y el
  cardiac drift. Ahora se encola tras cada upsert de actividad, tanto en el
  backfill como en el webhook.
- `GET /api/v1/activities/{id}/streams` devuelve los streams almacenados de
  una actividad del usuario; `409` con los tipos que faltan cuando no hay
  `time` o `latlng`.
- `POST /api/v1/activities/{id}/streams/refresh` reencola la importación de
  una actividad concreta (`202`) para las actividades anteriores al cambio.
- `GET /api/v1/gpx/{id}/weather?at=RFC3339` devuelve la previsión de
  Open-Meteo en cuatro puntos estratégicos del track (salida, cima, punto
  medio, meta), con caché de 1 h y degradación explícita.
- `GET /api/v1/gpx/{id}/terrain` devuelve segmentos de superficie y
  tecnicidad 1–5 a partir de OSM/Overpass, con caché de 24 h y degradación
  explícita.
- Ambas integraciones son **track-scoped**: el servidor deriva las
  coordenadas del track guardado del usuario, nunca de una URL o coordenadas
  enviadas por el cliente. No hay superficie SSRF y el navegador no contacta
  con terceros.

### Web

- Modelo puro, tipado y testeado en `web/src/features/lab/RacePlanning/model/`:
  geometría y tiempo, splits y cortes, hitos y nutrición, exposición solar,
  equipo y comparación plan-vs-real.
- Cliente `web/src/lib/api/lab.ts` y hooks de carga bajo demanda.
- `RacePlanSection` con ocho pestañas, montada dentro de `TrackDetailPage`
  bajo el detalle de la ruta. Ninguna petición externa al cargar la página;
  un fallo de meteo o terreno degrada solo su pestaña.
- Post-Activity compara los splits planificados con los splits reales de una
  actividad de Strava, usando la distancia acumulada propia de la actividad
  (no una alineación punto a punto), y ofrece re-sincronizar los streams
  cuando faltan.

## Decisions

1. **Post-Activity incluye backend** (decisión del usuario). Sin el stream
   `time` no hay comparación plan-vs-real posible; se añade la ingesta, la
   lectura y el re-sync.
2. **Entrega en cadena sobre rama de feature** (decisión del usuario), con
   seis PRs hijas y un PR tracker en borrador.
3. **Las llamadas externas van al backend**, no se relaja la CSP. Mantiene
   `connect-src 'self'`, evita exponer la IP del usuario a terceros y permite
   cachear en servidor.
4. **El modelo es puro y usa campos nombrados** (`{lat, lon, ele}`), con un
   adaptador desde el wire. Nada de `any` ni de coordenadas posicionales.
5. **La comparación plan-vs-real usa la distancia propia de la actividad.**
   Alinear índices entre dos tracks distintos es frágil; comparar splits por
   kilómetro recorrido es correcto aunque el recorrido real se desvíe.
6. **`suncalc@2.1.2` se usa con su contrato v2** (`altitude` y `azimuth` ya
   en grados). Aplicar la conversión v1 habría desactivado la detección de
   noche.

## Scope

### In scope

- Los ocho entregables de la issue #18.
- La corrección del encolado de streams.
- Los dos endpoints ambientales y los dos de streams.
- Specs canónicos de `gpx-backend` y `gpx-lab`, y archivado del change.

### Out of scope

- Persistir planes de carrera (no hay tabla ni CRUD de planes).
- Un selector de actividades recientes en Post-Activity (se usa el id de
  Strava; el selector es una mejora posterior).
- Modelo de sombras proyectadas por terreno: `shade` es una heurística de
  orientación/intensidad, no un cálculo de sombra arrojada.
- Cambiar el modelo de autenticación o el contrato de otras rutas.
- Relajar la CSP.

## Risks and mitigations

| Riesgo | Mitigación |
| --- | --- |
| Overpass es lento y tiene límites de uso | Timeout de 25 s, caché de 24 h por track, `around:50` sobre una polilínea decimada y `User-Agent` identificable. |
| Open-Meteo solo cubre su horizonte de previsión | `422` explícito cuando todos los puntos quedan fuera; el panel lo muestra sin romper la sección. |
| Actividades antiguas sin `time` | `409` con el detalle de streams que faltan y botón de re-sincronización. |
| Datos OSM incompletos | `unknown`/`available:false` son resultados válidos, no errores. |
| Caché por proceso en varias réplicas | Se documenta; no hay coordinación entre réplicas. Para el laboratorio es suficiente. |
| Coste de revisión de una fase grande | Cadena de seis PRs con superficies disjuntas y tests junto al comportamiento. |

## Delivery slices

| PR | Unidad | Commits |
| --- | --- | --- |
| 1 | Streams de actividad | `c7144f8` |
| 2 | Proxy de meteo | `1fad7d7` |
| 3 | Proxy de terreno | `572833a`, `6bff217` |
| 4 | Modelo web puro | `ce4b2bb`, `825ee6d`, `9cafe97`, `21b0aa9`, `6e7c06d` |
| 5 | Cliente y UI | `93e07dd`, `2dd47a0` |
| 6 | Post-Activity y docs | `aa41061`, cierre documental |
