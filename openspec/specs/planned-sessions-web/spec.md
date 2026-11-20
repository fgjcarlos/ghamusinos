# Canonical spec: planned-sessions-web (SPA)

## Source

- Slice 3 of GitHub epic #301 (Fase 2.1 — Creacion de entrenos), issue
  #300, delivered in commit `80b25b9` (`feat(web): add planned workouts
  screen (#326)`).
- API contract: `openspec/specs/planned-sessions-api/spec.md`.

## Domain Metadata

| Project root | `web` |
| --- | --- |
| Stack | React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react |
| Gating test command | `pnpm -C web test:run` |
| Quality gates | `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check` |

Canonical spec for the Entrenos screen of the SPA. The HTTP contract it
consumes is in `openspec/specs/planned-sessions-api/spec.md`.

## ADDED Requirements

### Requirement: the Entrenos screen is reachable from the navigation (`PW-001`)

The AppShell SHALL expose an `Entrenos` entry that navigates to the
route `/entrenos`, and the route SHALL render the planned-sessions
screen. The entry SHALL be present in the shell navigation and in the
top bar, so that the screen is reachable from any other view of the SPA.

**Project root**: `web`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/ui/AppShell/AppShell.tsx`,
`web/src/ui/AppShell/TopBar.tsx`, `web/src/AppRoutes.tsx`

#### Scenario: the navigation carries the entry

- GIVEN the AppShell
- WHEN its navigation is rendered
- THEN an entry labelled `Entrenos` links to `/entrenos`

#### Scenario: the route renders the screen

- GIVEN the SPA at the route `/entrenos`
- WHEN the route is resolved
- THEN the planned-sessions screen is rendered

### Requirement: the screen lists the sessions of the user (`PW-002`)

The screen SHALL request the sessions of the authenticated user for a
date window that starts on the current day and ends on the same month
and day of the following year, clamped to the last day of that month
when the day does not exist in it (29 February becomes 28 February), and
SHALL render them grouped by `scheduled_on` in ascending order. The screen SHALL page with a page size of `20` and
SHALL offer to load the next page while the API reports that more rows
exist. The screen SHALL render only the sessions returned for the
authenticated user.

**Project root**: `web`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/workouts/WorkoutsContainer.tsx`,
`web/src/features/workouts/workout-model.ts` (`getWorkoutWindow`,
`groupWorkouts`)

#### Scenario: the window starts today and ends in a year

- GIVEN a given current day
- WHEN the window is computed
- THEN `from` is that day in `YYYY-MM-DD`
- AND `to` is the same month and day of the following year
- AND `to` is clamped to the last day of that month when the day does not exist in it

#### Scenario: the sessions are grouped by day

- GIVEN sessions on two different dates
- WHEN they are grouped
- THEN one group per date exists
- AND the groups are ordered by date ascending

#### Scenario: the next page is loaded on demand

- GIVEN a first page that reports more rows to come
- WHEN the user asks for more
- THEN the next page is requested with the accumulated offset
- AND the sessions of both pages are rendered together

### Requirement: a session is created and edited from the form (`PW-003`)

The screen SHALL offer a form with a date, a title, a workout type, a
note, the three numeric targets (duration in seconds, distance in
metres, positive elevation gain in metres) and an intensity. The type
SHALL be one of `rodaje`, `tirada_larga`, `series`, `cuestas`, `tempo`,
`fartlek`, `fuerza`, `recuperacion`, `otro`; the intensity SHALL be one
of `z1`..`z5` or unplanned. The form SHALL validate before sending: the
date SHALL be a valid date inside the visible window, the title SHALL be
present and at most 200 bytes, the note at most 2000 bytes, the type
SHALL belong to the vocabulary, the intensity SHALL belong to its
vocabulary, and every target SHALL be a non-negative integer that fits
a signed 32-bit integer. A session opened for editing SHALL be prefilled
with its stored values.

**Project root**: `web`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/workouts/WorkoutForm.tsx`,
`web/src/features/workouts/workout-model.ts` (`validateWorkoutValues`)

#### Scenario: a valid form is sent

- GIVEN a form with a valid date, a title and a type
- WHEN the user saves it
- THEN the session is sent to the API
- AND the form is closed

#### Scenario: a missing title is refused

- GIVEN a form without a title
- WHEN the user saves it
- THEN the title field reports that it is obligatory
- AND no request is sent

#### Scenario: an out-of-window date is refused

- GIVEN a form with a date outside the visible window
- WHEN the user saves it
- THEN the date field reports the allowed range
- AND no request is sent

#### Scenario: a negative target is refused

- GIVEN a form with a negative numeric target
- WHEN the user saves it
- THEN the target field reports the allowed range
- AND no request is sent

#### Scenario: an edited session is prefilled

- GIVEN a session opened for editing
- WHEN the form is rendered
- THEN the fields carry the stored values of that session

### Requirement: a session is deleted after a confirmation (`PW-004`)

The screen SHALL ask for a confirmation before deleting a session, SHALL
send the delete request only after the confirmation, and SHALL reload the
list once the session is gone. The confirmation SHALL name the session
being deleted.

**Project root**: `web`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/workouts/WorkoutsContainer.tsx`

#### Scenario: the deletion is confirmed

- GIVEN a listed session
- WHEN the user confirms the deletion
- THEN the delete request is sent for that session
- AND the list is reloaded

#### Scenario: the deletion is cancelled

- GIVEN a listed session
- WHEN the user refuses the confirmation
- THEN no delete request is sent

### Requirement: the screen states are explicit (`PW-005`)

The screen SHALL render a loading state while the first page is being
fetched, an empty state when the window holds no session, an error
banner with the failure message when a request fails, and a retry
action when the list itself failed. While a write is in flight the
screen SHALL be busy and SHALL NOT send a second write. The screen
SHALL send the bearer token it holds with every request and SHALL
surface the failure of the API as a problem message. Today that token
comes from the `VITE_AUTH_TOKEN` placeholder, pending the real Clerk
session of issue #306; the ownership of the sessions is enforced by the
server, which scopes every query to the authenticated user.

**Project root**: `web`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/workouts/WorkoutsContainer.tsx`,
`web/src/lib/api/workouts.ts`

#### Scenario: the list fails and can be retried

- GIVEN the first page request fails
- WHEN the screen renders the failure
- THEN an error is shown with a retry action
- AND the retry action requests the first page again

#### Scenario: an empty window is explained

- GIVEN a window with no session
- WHEN the screen renders it
- THEN an empty state explains that there is nothing planned
- AND the empty state offers to create a session

#### Scenario: a write failure is surfaced

- GIVEN a create request that fails
- WHEN the screen renders the failure
- THEN the failure message is shown
- AND the session was not added to the list
