# ADR 0002 — Proveedor de IA

- **Estado:** aceptado
- **Fecha:** 2026-06-01
- **Contexto:** V1, IA opcional (fase 1.5)

## Decisión

IA con **arquitectura multi-proveedor** tras una interfaz común (`internal/ai`). El dominio nunca conoce el proveedor concreto. Se implementan en este orden:

1. **OpenAI** — primer proveedor soportado (implementación por defecto inicial).
2. **Claude (Anthropic)** — segundo proveedor.
3. **OpenRouter** — tercer proveedor (capa multi-modelo: Claude, DeepSeek, etc.).

El proveedor activo se elige por configuración. Añadir uno nuevo es implementar la interfaz, sin tocar el resto del sistema.

## Por qué

- No atar el producto a un único proveedor desde el inicio. La IA evoluciona rápido y los precios/calidad cambian.
- El usuario decidió empezar por **OpenAI** y sumar Claude y OpenRouter después.
- La IA es **opcional y no crítica**: la abstracción no añade complejidad al core, solo al módulo `ai`.
- El código legacy ya demostró que una interfaz delgada (usaba el SDK de OpenAI apuntando a OpenRouter) permite cambiar de modelo sin rediseño.

## Diseño

```text
internal/ai
├── analyzer.go        // interfaz Analyzer (independiente del proveedor)
├── openai.go          // implementación OpenAI      (1º)
├── claude.go          // implementación Anthropic    (2º)
├── openrouter.go      // implementación OpenRouter    (3º)
└── prompt.go          // payload builder + schema de salida validado
```

- La selección de proveedor y modelo va por variables de entorno (`AI_PROVIDER`, `AI_MODEL`, `*_API_KEY`).
- Schema de salida validado igual para todos los proveedores → la UI no depende del backend de IA.

## Consecuencias

- La generación corre en jobs River y nunca bloquea flujos críticos.
- Reintentos con backoff comunes a todos los proveedores.
- Feature flag global + opt-in por usuario para activar/desactivar IA.
- Cada proveedor es una pieza enchufable y testeable de forma aislada.
