# ADR 0002 — Proveedor de IA

- **Estado:** propuesto (pendiente de decisión)
- **Fecha:** 2026-06-01
- **Contexto:** V1, IA opcional (fase 1.5)

## Decisión a tomar

A través de qué proveedor se llama al modelo de lenguaje que genera los análisis interpretativos opcionales.

## Opciones

| Opción | Cómo funciona | Pro | Contra |
|---|---|---|---|
| **A. Claude API directa** | SDK de Anthropic contra el modelo Claude | Coherente con el principio del PRD ("IA con Claude"); prompt caching; control directo | Un único proveedor; sin fallback de modelo |
| **B. OpenRouter (legacy)** | SDK compatible OpenAI contra OpenRouter; multi-modelo (Claude, DeepSeek, etc.) | Flexibilidad de modelo y coste; un cambio de modelo sin tocar código | Capa intermedia; depende de un tercero; menos control de features nativas |

## Recomendación

**Opción A (Claude API directa)** como camino por defecto, manteniendo la integración **aislada tras una interfaz** (`internal/ai`) para poder enchufar OpenRouter u otro proveedor sin tocar el dominio.

Razones:

- El PRD posiciona la IA como "interpretación con Claude"; usar Claude directamente honra esa promesa y habilita prompt caching para abaratar prompts repetidos.
- La IA es **opcional y no crítica**: no necesita la redundancia multi-modelo de OpenRouter en V1.
- El código legacy ya demostró que una interfaz delgada permite cambiar de proveedor; se conserva esa abstracción.

## Consecuencias

- `internal/ai` expone una interfaz (`Analyzer`) independiente del proveedor.
- La implementación por defecto usa el SDK de Anthropic con prompt caching.
- La generación corre en jobs River y nunca bloquea flujos críticos.
- Cambiar a OpenRouter es una implementación alternativa de la misma interfaz.
