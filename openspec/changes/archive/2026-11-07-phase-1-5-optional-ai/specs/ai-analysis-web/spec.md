# Spec Delta: phase-1-5-optional-ai

> Domain: ai-analysis-web
> Source: openspec/changes/phase-1-5-optional-ai/{proposal,design}.md
> Cross-project: `web/` (React)
> Stack: React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react + jsdom
> Gating test command: `pnpm -C web test:run`
> Quality gates: `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check`

This delta covers the web surface of the optional AI feature: the
shared control that asks for an analysis, the per-activity trigger on
the activities screen, the weekly and load analyses on the performance
dashboard, and the user-facing states of the whole flow. No canonical
`openspec/specs/ai-analysis-web/spec.md` exists yet; the archive step of
the epic will copy this file into the canonical location.

No new screen and no new route are introduced: the AI analysis is
requested under demand from the screens that already exist, and with
the feature off those screens behave exactly as before (S7).

## ADDED Requirements

### Requirement: one shared control gates every AI request behind the feature status (`AW-001`)

`web/src/features/ai/AIAnalysisControl/AIAnalysisControl.tsx` SHALL be
the only component of the web application that requests an AI analysis.
It SHALL read `GET /api/v1/ai/status` before offering anything, and it
SHALL NOT send any request when the feature is disabled, unavailable or
not consented by the user. In those cases it SHALL explain what is
missing and SHALL link to the profile (`/perfil`), where the transfer
notice and the consent checkbox live. Only when the status is `ready`
and the user is consented SHALL it offer the action that requests the
analysis. The control SHALL accept the props `kind`
(`activity | weekly | load`), `activityId`, `windowStart`, `windowDays`,
`title`, `actionLabel`, `autoStart` and `token`, and it SHALL NOT mix an
`activityId` with a window in the request it builds.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/ai/AIAnalysisControl/AIAnalysisControl.tsx`

#### Scenario: with the AI disabled no request leaves the browser

- GIVEN the status endpoint answers `enabled = false`,
  `available = false`, `status = "disabled"` and `consented = false`
- WHEN the control is mounted
- THEN it shows that the AI is not available
- AND it offers no action to request an analysis
- AND the only request that was sent is the status request

#### Scenario: with the AI available but not consented no request leaves the browser

- GIVEN the status endpoint answers `enabled = true`,
  `available = true`, `status = "ready"` and `consented = false`
- WHEN the control is mounted and the user waits
- THEN it explains that the transfer notice has to be accepted
- AND it links to the profile
- AND no request to create an analysis was sent

#### Scenario: with the feature ready and consented the action is offered

- GIVEN the status endpoint answers `enabled = true`,
  `available = true`, `status = "ready"` and `consented = true`
- WHEN the control is mounted
- THEN it shows the action button
- AND no request to create an analysis has been sent yet

#### Scenario: the request never mixes the activity and the window fields

- GIVEN the control is mounted with `kind = "activity"` and an
  `activityId`
- WHEN the user asks for the analysis
- THEN the created request body has `kind = "activity"` and the
  `activity_id`
- AND the body has no `window_start` and no `window_days`

### Requirement: the activities screen offers an analysis per activity (`AW-002`)

`web/src/features/activities/ActivityList/ActivityList.tsx` SHALL offer,
for every activity that has a valid identifier, a trigger labelled
"Analizar con IA" that opens the shared control with `kind = "activity"`
and the identifier of that activity. The trigger SHALL NOT request
anything by itself: it only opens the control, which applies the gate
of `AW-001`. An activity without a valid identifier SHALL NOT offer the
trigger. No other row of the activities screen SHALL change.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/activities/ActivityList/ActivityList.tsx`

#### Scenario: each activity offers its own trigger

- GIVEN a list with three activities, two of them with a valid
  identifier
- WHEN the list is rendered
- THEN two triggers labelled "Analizar con IA" are shown
- AND opening one of them mounts the control with the identifier of that
  activity

#### Scenario: an activity without a valid identifier has no trigger

- GIVEN an activity whose identifier is not valid
- WHEN the list is rendered
- THEN no trigger is shown for that activity

### Requirement: the dashboard offers the weekly and the load analyses (`AW-003`)

`web/src/features/dashboard/DashboardAIAnalysis.tsx` SHALL mount two
instances of the shared control on the performance dashboard: the
weekly analysis, whose window SHALL be the last 7 days, and the load
analysis, whose window SHALL follow the period selected in the panel
and SHALL NOT exceed the 90 days that the API accepts
(`AI_MAX_WINDOW_DAYS`). The window helpers
(`web/src/features/dashboard/aiWindow.ts`) SHALL compute both windows
from the last day of the panel, so the window of the request is always
the one that the panel is showing.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/DashboardAIAnalysis.tsx`, `web/src/features/dashboard/aiWindow.ts`

#### Scenario: the weekly analysis uses a window of 7 days

- GIVEN the panel's last day is `2026-01-28`
- WHEN the weekly window is computed
- THEN the window starts on `2026-01-22` and lasts 7 days

#### Scenario: the load analysis follows the period of the panel and is bounded

- GIVEN the panel's last day is `2026-01-28` and the selected period is
  30 days
- WHEN the load window is computed
- THEN the window starts on `2025-12-30` and lasts 30 days
- AND the number of days never exceeds 90
- GIVEN the selected period is `1y` (365 days)
- WHEN the load window is computed
- THEN the window is truncated to 90 days instead of being rejected by
  the API

### Requirement: the control exposes the states of the analysis to the user (`AW-004`)

The control SHALL render, in this order, the following states: reading
the status ("Comprobando si el análisis con IA está disponible…"), the
feature disabled or unavailable, the user without consent, ready (the
action button), in progress ("Analizando con IA…"), completed (the
validated output) and failed (the safe message and the safe error
code). While an analysis is in progress the control SHALL poll the
status of that analysis every 2 seconds (`AI_POLL_INTERVAL_MS`) and
SHALL stop after 60 attempts (`AI_MAX_POLL_ATTEMPTS`), telling the user
that it stopped instead of waiting silently. The control SHALL treat an
analysis whose status is `pending` as in progress, `completed` as
finished and `failed` as failed.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/ai/AIAnalysisControl/AIAnalysisControl.tsx`, `web/src/features/ai/AIAnalysisControl/constants.ts`

#### Scenario: a pending analysis shows that it is running and is polled

- GIVEN the user asked for an analysis and the status endpoint answers
  `status = "pending"`
- WHEN the control renders
- THEN it shows that the analysis is running
- AND it queries the analysis again after 2 seconds

#### Scenario: a completed analysis shows the validated output

- GIVEN the status endpoint answers `status = "completed"` with an
  output holding a summary, one observation and no caution
- WHEN the control renders
- THEN it shows the summary, the observation and no caution

#### Scenario: a failed analysis shows the safe message and the safe code

- GIVEN the status endpoint answers `status = "failed"` with
  `error_code = "ai_unavailable"` and a safe `error_message`
- WHEN the control renders
- THEN it shows the safe message and the code `ai_unavailable`
- AND it shows no other technical detail

#### Scenario: the polling stops after the maximum number of attempts

- GIVEN the analysis stays `pending` for 60 consecutive polls
- WHEN the 60th attempt is done
- THEN the control stops polling and tells the user that it stopped
- AND it offers to ask again

### Requirement: the web never renders the sent payload nor the provider internals (`AW-005`)

The control SHALL render only the validated output (`summary`,
`observations`, `cautions`) and the safe error code and message of a
failed analysis. It SHALL NEVER render the payload that was sent to the
provider, the configured model, the credential, the raw body returned by
the provider or the URL of the provider. A failure of the request
itself SHALL be translated into a message the user can act on (consent
missing, activity not found, AI unavailable, or a generic retry
message), without showing the technical detail of the problem document.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/ai/AIAnalysisControl/AIAnalysisControl.tsx`

#### Scenario: only the validated output is painted

- GIVEN a completed analysis whose stored payload holds the aggregate
  metrics that were sent
- WHEN the control renders the result
- THEN the rendered text contains the summary, the observations and the
  cautions
- AND it contains none of the numbers that were sent

#### Scenario: a request failure is translated into an actionable message

- GIVEN the request fails with the status `403`
- WHEN the control renders the error
- THEN it explains that the transfer notice has to be accepted before
  asking for an analysis
- GIVEN the request fails with the status `404`
- WHEN the control renders the error
- THEN it explains that the activity was not found
- GIVEN the request fails with the status `503`
- WHEN the control renders the error
- THEN it explains that the AI is not available at the moment
- AND in no case does it render the technical detail of the problem
  document

### Requirement: the feature adds no screen and no route (`AW-006`)

The web application SHALL NOT add a route, a page or a navigation entry
for the AI feature: the analyses are requested under demand from the
activities screen and from the performance dashboard. With the feature
disabled, unavailable or not consented, those screens SHALL render
exactly as they did before the feature existed.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/ai/AIAnalysisControl/`, `web/src/features/dashboard/DashboardAIAnalysis.tsx`

#### Scenario: the activities screen keeps its content without the AI

- GIVEN the status endpoint answers `status = "disabled"`
- WHEN the activities screen is rendered
- THEN the list of activities, the summary and the pagination are
  rendered as usual
- AND the only difference is the explanation of the control

#### Scenario: the dashboard keeps its panels without the AI

- GIVEN the status endpoint answers `status = "disabled"`
- WHEN the dashboard is rendered
- THEN the KPI cards, the load chart, the HR zones and the cardiac
  drift panel are rendered as usual
- AND the AI section explains that the feature is not available
