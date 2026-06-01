# PRD — Ghamusinos

Ghamusinos es una plataforma personal de análisis y planificación para trail running. Su objetivo no es solo guardar actividades, sino convertir rutas, entrenamientos y métricas en información útil para decidir mejor: cómo está el corredor, cuánta fatiga acumula, cómo progresa y cómo preparar próximas rutas, sesiones o carreras.

## 1. Resumen ejecutivo

Ghamusinos combina tres fuentes principales de valor:

- **Análisis GPX** para entender rutas de trail antes o después de correrlas.
- **Ingesta de actividades desde Strava** para construir un histórico deportivo propio.
- **Métricas de rendimiento y salud/fatiga** para interpretar la evolución del corredor.

La IA con Claude será opcional y deberá enriquecer la interpretación, no sustituir el cálculo determinista de métricas.

## 2. Visión del producto

Convertir datos deportivos dispersos en decisiones accionables para corredores de trail running.

Ghamusinos debe ayudar a responder:

| Pregunta | Respuesta esperada |
|---|---|
| ¿Qué tan dura es esta ruta? | Distancia, desnivel, pendiente, subidas clave, dificultad y esfuerzo estimado |
| ¿Cómo estoy de forma? | Tendencias de rendimiento, carga y fatiga |
| ¿Estoy acumulando demasiada fatiga? | Métricas como ATL, TSB, cardiac drift y evolución reciente |
| ¿Cómo progresé como corredor? | Histórico de actividades, desnivel, carga y eficiencia |
| ¿Qué hice últimamente? | Actividades importadas desde Strava |
| ¿Qué significa esta actividad? | Resumen interpretativo opcional con IA |

## 3. Problema

Los corredores de trail suelen usar varias herramientas separadas para registrar, analizar y planificar:

| Necesidad | Herramientas habituales | Limitación |
|---|---|---|
| Registrar actividades | Strava, Garmin, Suunto | Mucho dato, poca interpretación trail específica |
| Analizar forma | Intervals.icu, TrainingPeaks | Enfoque más general o complejo |
| Analizar rutas GPX | Wikiloc, Komoot, visores GPX | Poca lectura de esfuerzo real de montaña |
| Entender fatiga | Hojas, apps externas, intuición | Falta contexto integrado |
| Planificar entrenos | Calendarios o plataformas avanzadas | No siempre conectado al perfil trail del corredor |

Ghamusinos busca unificar análisis de rutas, actividades y evolución del corredor con foco específico en trail running.

## 4. Propuesta de valor

Ghamusinos transforma tracks GPX y actividades deportivas en información accionable para corredores de montaña.

| Diferenciador | Valor |
|---|---|
| Laboratorio GPX | Analizar rutas antes o después de correrlas |
| Métricas trail | Dificultad, desnivel, VAM, GAP, esfuerzo y subidas clave |
| Strava como fuente | Importar actividades sin duplicar el registro deportivo |
| Salud/fatiga | Interpretar carga, fatiga y rendimiento de forma visual |
| IA opcional | Explicar métricas y actividades en lenguaje natural |
| V2 de planificación | Crear y gestionar entrenos desde el análisis previo |

## 5. Usuarios objetivo

| Usuario | Necesidad | Dolor actual |
|---|---|---|
| Corredor trail amateur | Saber si una ruta es adecuada | Los GPX son difíciles de interpretar |
| Corredor avanzado | Comparar esfuerzo, desnivel y rendimiento | Usa varias herramientas separadas |
| Corredor preparando carrera | Entender dureza y fatiga acumulada | Falta conexión entre ruta, estado y planificación |
| Entrenador | Revisar carga y progresión | Falta lectura trail específica |
| Usuario de Strava | Enriquecer sus actividades | Strava no ofrece suficiente análisis de montaña |

## 6. Principios de producto

- **V1 analiza; V2 planifica.**
- **Strava alimenta el sistema, pero no define el producto.**
- **Las métricas se calculan con fórmulas conocidas y transparentes.**
- **La IA interpreta datos; no inventa métricas.**
- **El producto debe funcionar sin IA.**
- **Las métricas derivadas son estimaciones orientativas, no diagnósticos médicos ni verdades absolutas.**

## 7. Alcance V1

La V1 se centra en análisis GPX, ingesta Strava y dashboard básico de rendimiento/salud-fatiga.

### Incluye

| Área | Funcionalidad |
|---|---|
| Autenticación | Clerk y acceso por invitación |
| Setup inicial | Primera pantalla para conectar Strava o subir GPX |
| Strava | OAuth, refresh tokens, backfill histórico acotado, webhooks y deduplicación |
| Actividades | Lista/carrusel de actividades importadas |
| GPX | Subida, validación y parsing de tracks |
| Métricas de ruta | Distancia, desnivel, pendiente, VAM básica, GAP básico |
| Análisis trail | Dificultad, esfuerzo estimado y subidas clave |
| Visualización | Resumen de ruta y perfil de elevación |
| Dashboard | Rendimiento, volumen, desnivel, carga básica y salud/fatiga |
| Histórico | Tracks y actividades analizadas |
| IA | Análisis opcional con Claude, con consentimiento explícito |

### No incluye

| Funcionalidad | Motivo |
|---|---|
| Creación y gestión de entrenos | Pasa a V2 |
| Calendario de entrenamientos | Pasa a V2 |
| Planes de entrenamiento | Pasa a V2 |
| Feed social, kudos o comentarios | No compite con Strava |
| Promesas de precisión absoluta | Trail tiene mucho ruido de GPS, altitud y contexto |
| Diagnóstico médico | El producto no sustituye criterio profesional |

## 8. Fases de V1

### Fase 1.1 — Base de producto y autenticación

Objetivo: permitir acceso controlado al producto.

- Clerk para registro/login.
- Sistema de invitaciones.
- Setup inicial.
- Estructura base de usuario.
- Preferencias iniciales, incluyendo IA activada/desactivada.

### Fase 1.2 — Ingesta Strava

Objetivo: importar actividades reales del usuario.

- OAuth con Strava.
- Refresh de tokens.
- Backfill histórico acotado.
- Webhooks para nuevas actividades.
- Deduplicación.
- Normalización interna del modelo de actividad.
- Lista/carrusel de actividades.

### Fase 1.3 — Laboratorio GPX (base)

Objetivo: convertir tracks GPX en análisis trail útil. Es el **diferenciador central** del producto y la funcionalidad más madura del código previo (en `old_ghamusinos`, client-side con MapLibre).

- Subida de GPX/GeoJSON y validación de archivo.
- Parsing de geometría, elevación y timestamps si existen.
- Cálculo de distancia, D+/D−, pendiente y segmentos de gradiente.
- Métricas trail: Effort Index, VAM, ITRA, Leg-Breaker, Runnability.
- Detección de subidas, **Km Vertical (tramo de subida sostenida continua)**, King Climb, muros, recovery zones y zonas de riesgo.
- Mapa 3D interactivo (MapLibre) con heatmap de pendientes y perfil de elevación.
- Comparador de rutas (hasta 3).
- Score de dificultad y esfuerzo.
- Persistencia del análisis y hash de fichero para deduplicación.

### Fase 1.4 — Dashboard de rendimiento y salud/fatiga

Objetivo: mostrar estado y evolución básica del corredor.

- Volumen semanal.
- Desnivel semanal.
- Número de actividades.
- Tendencia respecto a periodos anteriores.
- Métricas iniciales de carga/fatiga cuando haya datos suficientes.
- Visualización clara de rendimiento y salud/fatiga.

### Fase 1.5 — IA opcional con Claude

Objetivo: enriquecer la interpretación sin hacer que la IA sea obligatoria.

- Activación explícita por parte del usuario.
- Payload builder controlado.
- Schema de salida validado.
- Reintentos controlados.
- Persistencia de análisis IA.
- Resumen de actividad o ruta.
- Explicación de métricas complejas.

### Fase 1.6 — Laboratorio GPX avanzado

Objetivo: extender el laboratorio con planificación de carrera y contexto ambiental. Funcionalidades ya prototipadas client-side en `old_ghamusinos`; se separan del MVP para no inflar el alcance inicial.

- Race Day: tiempo estimado, presets de ritmo, puntos de nutrición e hitos KM.
- Cutoff Calculator: barreras horarias y verificación de ritmo.
- Strategic Splits: splits por km con factor de fatiga ajustable.
- Terrain Info: superficie y tecnicidad vía OSM/Overpass.
- Weather: previsión en puntos estratégicos de la ruta (Open-Meteo).
- Solar Exposure: tramos de sol/sombra/noche según hora y orientación.
- Gear Checklist: equipo según duración, altitud y meteorología.
- Post-Activity: comparación plan vs. actividad real (Fatigue Index por km).

## 9. Métricas V1

Las métricas se basarán en fórmulas conocidas siempre que sea posible. Deben mostrarse como estimaciones y tendencias.

| Métrica | Uso | Precisión esperada |
|---|---|---|
| Distancia | Longitud de ruta/actividad | Alta si el GPX es correcto |
| Desnivel positivo/negativo | Exigencia de montaña | Media; depende de calidad de elevación |
| Pendiente | Caracterización del recorrido | Media |
| VAM | Rendimiento en subida | Media-alta en subidas limpias |
| GAP | Ritmo ajustado por pendiente | Orientativo |
| TSS | Carga estimada | Orientativo |
| CTL | Tendencia de fitness | Tendencial, no absoluto |
| ATL | Fatiga reciente | Tendencial, no diagnóstico |
| TSB | Balance forma/fatiga | Tendencial |
| EF | Eficiencia aeróbica | Depende de datos de FC/ritmo consistentes |
| Cardiac drift | Fatiga aeróbica | Útil solo con datos cardíacos fiables |
| Dificultad de ruta | Comparar rutas | Score propio orientativo |

## 10. Alcance V2

La V2 se centra en creación y gestión de entrenamientos.

### Incluye

| Área | Funcionalidad |
|---|---|
| Entrenos | Crear entrenamientos manuales |
| Calendario | Gestionar sesiones futuras |
| Objetivos | Definir carreras objetivo |
| Bloques | Organizar semanas o bloques de entrenamiento |
| Comparación | Carga planificada vs carga realizada |
| Reportes | Semana, mes y bloques más largos |
| IA opcional | Ayuda interpretativa para revisar planificación |

## 11. Fases de V2

### Fase 2.1 — Creación de entrenos

- Crear sesión.
- Definir tipo de entrenamiento.
- Añadir duración, distancia, desnivel o intensidad objetivo.
- Asociar sesión a objetivo.

### Fase 2.2 — Calendario

- Vista semanal y mensual.
- Mover sesiones.
- Marcar sesión como completada, omitida o modificada.
- Asociar actividad real a sesión planificada.

### Fase 2.3 — Objetivos de carrera

- Crear carrera objetivo.
- Fecha, distancia, desnivel y prioridad.
- Asociar rutas GPX a carrera.
- Ver progreso hacia el objetivo.

### Fase 2.4 — Reportes avanzados

- Reportes semanales.
- Reportes mensuales.
- Evolución de carga, fatiga, desnivel y rendimiento.
- Resúmenes por bloque.

### Fase 2.5 — IA aplicada a planificación

- Lectura del bloque de entrenamiento.
- Explicación de riesgos de fatiga.
- Sugerencias orientativas si el usuario activa IA.
- Resumen antes/después de una carrera objetivo.

## 12. Arquitectura técnica prevista

La arquitectura objetivo es **Go-first con binario único**. El detalle y los tradeoffs viven en `docs/architecture/technology-stack.md`; el mapeo funcionalidad → módulo en `docs/architecture/feature-inventory.md`.

| Capa | Tecnología |
|---|---|
| Backend | Go + Chi |
| Frontend | React + Vite (embebido en el binario con `embed.FS`) |
| Base de datos | PostgreSQL + TimescaleDB |
| Queries | SQLC |
| Migraciones | Goose |
| Jobs/background | River (sobre PostgreSQL) |
| Autenticación | Clerk |
| Proveedor deportivo | Strava |
| IA | OpenAI / Claude / OpenRouter (opcional, multi-proveedor) |
| Mapas | MapLibre |
| Gráficas | ECharts |

> Nota: una iteración temprana del proyecto se construyó en NestJS + Astro. Se decidió pivotar a Go-first por simplicidad operativa (binario único), rendimiento en procesamiento de datos deportivos y mejor encaje con ingesta, jobs y SQL explícito. El código NestJS legacy se conserva como especificación de referencia.

### Módulos backend sugeridos (Go)

| Módulo | Responsabilidad |
|---|---|
| `internal/auth` | Integración con Clerk, usuario interno y acceso por invitación |
| `internal/invites` | Emisión, aceptación y validación de invitaciones |
| `internal/strava` | OAuth, webhooks, refresh de tokens y cliente de la API |
| `internal/activities` | Actividades importadas, normalizadas y deduplicadas |
| `internal/gpx` | Parsing de tracks, elevación y análisis de rutas |
| `internal/metrics` | TSS, GAP, VAM, EF, cardiac drift, CTL/ATL/TSB |
| `internal/ai` | Claude: payloads, reintentos, schema y persistencia |
| `internal/jobs` | Workers River: import, backfill, dedup, recálculo, insights |
| `internal/workouts` | Creación y gestión de entrenos (V2) |
| `internal/races` | Objetivos de carrera (V2) |
| `internal/reports` | Reportes por periodo (V2) |

## 13. Riesgos

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Alcance demasiado amplio | Alto | Separar V1 análisis y V2 planificación |
| Parecer una copia de Strava | Alto | Posicionar Strava como fuente, no como producto |
| Métricas poco fiables | Alto | Mostrar estimaciones, fórmulas conocidas y versión de algoritmo |
| GPX con mala elevación | Medio | Validación, normalización y avisos al usuario |
| IA con datos sensibles | Alto | Opt-in claro y control de payload |
| Webhooks Strava inconsistentes | Medio | Backfill/reconciliación periódica |
| Dashboard demasiado complejo | Medio | Empezar con tendencias claras y pocas métricas |

## 14. Métricas de éxito del producto

| Métrica | Qué valida |
|---|---|
| Usuarios que suben GPX | Interés en análisis de rutas |
| GPX analizados por usuario | Recurrencia del laboratorio |
| Usuarios que conectan Strava | Valor de integración |
| Actividades sincronizadas correctamente | Robustez de ingesta |
| Uso del dashboard | Valor de rendimiento/salud-fatiga |
| Activación de IA | Interés en interpretación inteligente |
| Retención semanal | Utilidad recurrente |

## 15. Decisiones abiertas

- Fórmulas exactas iniciales para TSS, GAP, CTL, ATL y TSB (referencia portable en el código legacy).
- Límite de backfill histórico desde Strava (legacy: ventana de 42 días).
- Tamaño máximo de GPX admitido.
- Si se guarda el archivo GPX original o solo datos procesados.
- Modelo inicial del score de dificultad.
- Qué datos exactos se enviarán a Claude cuando IA esté activada.
### Decisiones ya cerradas

- **Análisis GPX**: se ejecuta en el **backend (Go)**; el frontend solo renderiza.
- **Credenciales Strava**: **app global**; el usuario solo pulsa "Conectar con Strava" (sin tokens visibles). Ver `docs/decisions/0001-strava-credentials.md`.
- **Proveedor de IA**: **multi-proveedor** tras interfaz común; orden OpenAI → Claude → OpenRouter. Ver `docs/decisions/0002-ai-provider.md`.
- Nivel de detalle del dashboard V1.
