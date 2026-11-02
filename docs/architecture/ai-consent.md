# Consentimiento de IA (fase 1.5 · issue #17)

La IA es opcional y sólo entra en juego cuando el usuario lo autoriza
expresamente. Sin consentimiento no sale ningún dato del sistema, y un
valor guardado heredado no cuenta como consentimiento: la fase 1.5
exige una aceptación nueva para **todas** las cuentas.

## Camino rápido

1. El usuario entra en `/perfil`.
2. Lee el aviso de transferencia que aparece junto a la casilla.
3. Marca **«Autorizar el envío de mis métricas al proveedor de IA»**.
4. Pulsa **«Guardar preferencias»**.
5. Desde ese momento el producto puede enviar métricas agregadas al
   proveedor de IA configurado; si lo desmarca y vuelve a guardar, el
   consentimiento queda revocado.

## Decisiones del contrato

| Tema | Decisión |
|---|---|
| Interruptor | `users.ai_enabled` (preferencias del usuario) |
| Valor por defecto | `false` (migración `00014`) |
| Acto de consentimiento | `PATCH /api/v1/me/preferences` con `ai_enabled: true` explícito |
| `ai_enabled` omitido | se guarda `false`: omitir nunca es consentir |
| Revocación | `ai_enabled: false` explícito + guardar |
| Aviso de transferencia | junto a la casilla en `/perfil` (existente, sin pantalla nueva) |
| Feature flag global | unidad T3 (apagado por defecto) |
| Comprobación del consentimiento | al encolar y antes de enviar (T3) |

El contrato HTTP no cambia: se reutilizan `GET` y `PATCH`
`/api/v1/me/preferences`, con la misma normalización y validación de
#159 (`hr_max`, `lthr`, `ftp`, `level`, `timezone`) y las mismas
semánticas de omisión y `null`. Ningún campo del perfil cambia de
significado por el consentimiento.

## Qué datos pueden salir del sistema

El aviso de la pantalla de preferencias lo dice de forma explícita y es
la referencia para la revisión de seguridad:

- **Sí**: distancia, duración, desnivel, frecuencia cardíaca y carga
  calculada (métricas agregadas de la actividad).
- **No**: nada que identifique al usuario (nombre, correo electrónico),
  ni coordenadas GPS, ni los *streams* crudos de las actividades.

La lista blanca del *payload* vive en `internal/ai` (unidad T1) y es la
que materializa esta tabla.

## Invalidación irreversible (migración `00014`)

`users.ai_enabled` nació con `DEFAULT true`, así que un `true` guardado
no demuestra que el usuario aceptase alguna vez el aviso de
transferencia. La migración `00014_ai_explicit_opt_in.sql`:

1. cambia el default de la columna a `false`, de modo que una cuenta
   nueva nace sin consentimiento;
2. resetea a `false` todos los `true` heredados.

El reseteo es **irreversible a propósito**: la información que
distinguía un consentimiento real del valor por defecto ya no existe.
Las cuentas vuelven a la casilla desmarcada y deben aceptar de nuevo.

El `DOWN` de la migración es deliberadamente conservador:

- no reescribe ningún valor almacenado (quien quedó reseteado, queda
  reseteado);
- no restaura `DEFAULT true`, porque eso consentiría automáticamente a
  las cuentas creadas después del *rollback*, que es exactamente el
  consentimiento implícito que la migración elimina.

El `schema.sql` que usa SQLC declara `DEFAULT false`, de modo que el
esquema en disco y el esquema generado coinciden.

## Riesgos y límites conocidos

- **Sin campo de versión del aviso.** El contrato público no se amplía,
  así que el backend no puede verificar que el cliente haya mostrado el
  aviso: un cliente API podría enviar `ai_enabled: true` sin leerlo. La
  mitigación es la combinación de la casilla (acción deliberada) con el
  reseteo de la migración (ningún consentimiento heredado sobrevive).
  Un *stamp* de versión del aviso sería una ampliación del contrato
  público y queda fuera de esta unidad.
- **El `DOWN` no restituye el estado anterior.** Es intencionado, pero
  conviene saberlo antes de revertir en producción.
- **La comprobación en el envío es de T3.** Esta unidad deja el flag y
  el aviso; que ningún job envíe datos sin consentimiento es de la
  unidad T3 (comprobación al encolar y antes de enviar).

## Dónde está cada cosa

| Pieza | Ruta |
|---|---|
| Migración | `internal/db/migrations/00014_ai_explicit_opt_in.sql` |
| Esquema SQLC | `internal/db/schema.sql` |
| Handlers | `internal/http/handlers/me_preferences.go` |
| Aviso en la UI | `web/src/features/profile/PreferencesForm.tsx` |
| Cliente de la API | `web/src/lib/api/profile.ts` |

## Checklist de revisión

- [ ] La migración fija el default de `ai_enabled` en `false`.
- [ ] La migración resetea los `true` heredados.
- [ ] El `DOWN` no restituye valores ni default que consentan.
- [ ] El aviso nombra proveedor, métricas permitidas y exclusiones.
- [ ] La casilla arranca desmarcada y sólo viaja activada tras marcar
      y guardar.
- [ ] Omitir `ai_enabled` en el `PATCH` no concede consentimiento.
- [ ] El resto de las preferencias se guarda igual que antes.

## Siguiente paso

T3: encender el *feature flag* global, persistir `ai_analysis` y
comprobar el consentimiento **al encolar y antes de enviar** cada
análisis.
