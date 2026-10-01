# Spec Delta: phase-1-4-performance-dashboard

> Domain: dashboard-web
> Source: openspec/changes/phase-1-4-performance-dashboard/{proposal,exploration}.md
> Cross-project: `web/` (React)
> Stack: React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react + jsdom + echarts (added in PR4 per E1)
> Gating test command: `pnpm -C web test:run`
> Quality gates: `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check`

This delta covers PR4: the `/dashboard` SPA page rendered
with ECharts (CTL/ATL/TSB line, HR-zones stacked bar,
summary KPIs with sparkline), the period selector, the
conditional Cardiac Drift panel (G5), the empty state for
users without activities, and the container +
presentational pattern with Vitest tests. No canonical
`openspec/specs/dashboard-web/spec.md` exists yet; archive
will copy this file into the canonical location.

## ADDED Requirements

### Requirement: `/dashboard` page renders with a 7d/30d/90d/1y period selector (`DW-001`)

`web/src/routes/dashboard.tsx` SHALL mount under the
`/dashboard` route inside the existing `AppShell` (added via
`<NavLink to="/dashboard">` in `AppShell.tsx`). The page MUST
render a period selector offering exactly the four options
`7d`, `30d`, `90d`, `1y`. Selecting an option MUST update the
`from`/`to` query params (or local state consumed by the
hooks) and trigger a re-fetch in the `useTrainingLoad`,
`useHRZones`, and `useDashboardSummary` hooks. The page
SHALL compose the three feature components
(`DashboardSummaryCard`, `TrainingLoadChart`, `HRZonesChart`)
in that DOM order, with `var(--gh-*)` design tokens for all
colors (no hardcoded colors).

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/routes/dashboard.tsx`, `web/src/features/dashboard/PeriodSelector.tsx`

#### Scenario: page renders with the default 30-day period

- GIVEN the user navigates to `/dashboard` with no query params
- WHEN the page mounts
- THEN the period selector shows `7d`, `30d`, `90d`, `1y`
  with `30d` selected by default
- AND the `useTrainingLoad`, `useHRZones`,
  `useDashboardSummary` hooks each receive a `from`/`to`
  range that resolves to the last 30 days
- AND the three feature components (`DashboardSummaryCard`,
  `TrainingLoadChart`, `HRZonesChart`) appear in that DOM
  order in the document

#### Scenario: selecting a different period re-queries the hooks

- GIVEN the page is mounted with `30d` selected
- WHEN the user clicks the `90d` option
- THEN the period selector marks `90d` as selected
- AND each hook is invoked with a new `from`/`to` range
  spanning 90 days
- AND the rendered charts re-render with the new data

### Requirement: `TrainingLoadChart` renders CTL/ATL/TSB as ECharts lines (`DW-002`)

`web/src/features/dashboard/TrainingLoadChart.tsx` SHALL
consume `TrainingLoadPoint[]` (from
`web/src/lib/api/dashboard.ts`) and render an ECharts line
chart with three series (CTL, ATL, TSB). The chart SHALL use
`echarts/core` plus the `LineChart`, `GridComponent`,
`TooltipComponent`, `LegendComponent`, and `TitleComponent`
imports (tree-shaken — proposal E1). All colors SHALL come
from `var(--gh-*)` tokens via the ECharts theme integration;
no hardcoded hex colors. The component MUST render its
loading and error states distinctly from the populated state.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/TrainingLoadChart.tsx`

#### Scenario: a populated series renders three ECharts series

- GIVEN a `TrainingLoadSeries` of 7 points with non-zero
  CTL/ATL/TSB on each day
- WHEN `<TrainingLoadChart>` is rendered
- THEN the rendered ECharts canvas (testid `training-load-chart`)
  is in the document
- AND the legend contains three labels: `CTL`, `ATL`, `TSB`
- AND each series contributes at least one data point to the
  chart options passed to ECharts

#### Scenario: a zero-only series renders without crashing

- GIVEN a `TrainingLoadSeries` of 7 points all with
  `ctl = atl = tsb = 0`
- WHEN `<TrainingLoadChart>` is rendered
- THEN the chart is in the document (no crash on the
  all-zeros case)
- AND the three series are present (visible or collapsed to
  the baseline)

### Requirement: `HRZonesChart` renders stacked-bar ECharts with degraded-label support (`DW-003`)

`web/src/features/dashboard/HRZonesChart.tsx` SHALL consume
`HRZonesResponse` (with `degraded: boolean` and `zones:
HRZoneBucket[]`) and render an ECharts stacked-bar chart
with five bars (z1..z5). When `degraded = true`, the chart
SHALL display the header label `"aproximación (sin streams
HR)"` so users understand the bars are derived from
`avg_hr × elapsed`, not from per-sample HR streams. When
`degraded = false`, the degraded label SHALL NOT be present.
All colors SHALL come from `var(--gh-*)` tokens.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/HRZonesChart.tsx`

#### Scenario: degraded=false omits the approximation label

- GIVEN an `HRZonesResponse` with `degraded: false` and five
  `zones` entries
- WHEN `<HRZonesChart>` is rendered
- THEN the chart canvas (testid `hr-zones-chart`) is in the
  document
- AND the approximation label `"aproximación (sin streams HR)"`
  is NOT in the document

#### Scenario: degraded=true shows the approximation label

- GIVEN an `HRZonesResponse` with `degraded: true` and five
  `zones` entries
- WHEN `<HRZonesChart>` is rendered
- THEN the approximation label `"aproximación (sin streams HR)"`
  is in the document (testid `hr-zones-degraded-label`)
- AND the chart canvas is still rendered (the chart is
  labelled, not hidden — proposal's "label, don't hide" stance)

### Requirement: `DashboardSummaryCard` renders four KPI tiles plus a weekly sparkline (`DW-004`)

`web/src/features/dashboard/DashboardSummaryCard.tsx` SHALL
render four `MetricTile` instances (from
`web/src/ui/MetricTile/`) for `weekly_volume_m`,
`weekly_elevation_m`, `weekly_activities_count`, and a
composite trend tile. A weekly sparkline (built from the
last 7 days of `training_load_daily` via the `useTrainingLoad`
hook with `from = today − 6d, to = today`) MUST accompany the
card. When `weekly_activities_count = 0`, the card SHALL
render the empty-state label (see `DW-006`).

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/DashboardSummaryCard.tsx`

#### Scenario: populated card renders four tiles and a sparkline

- GIVEN a `DashboardSummary` with
  `weekly_volume_m = 42500`, `weekly_elevation_m = 850`,
  `weekly_activities_count = 4`, `trend_7d_pct = 12.5`,
  `trend_30d_pct = -3.2`
- AND a `useTrainingLoad` hook returning a 7-day series
- WHEN `<DashboardSummaryCard>` is rendered
- THEN the four KPI tiles are in the document
- AND each tile shows its corresponding value
- AND the sparkline canvas (testid `weekly-sparkline`) is
  rendered with the 7 data points

#### Scenario: empty week renders the empty state instead of zeros

- GIVEN a `DashboardSummary` with
  `weekly_activities_count = 0` and the rest of the fields
  zero or null
- WHEN `<DashboardSummaryCard>` is rendered
- THEN the empty-state element (testid
  `dashboard-empty-state`) is in the document
- AND the KPI tiles are NOT rendered (or rendered with a
  `—` placeholder and a hidden sparkline)

### Requirement: Cardiac Drift panel is rendered only when HR streams are present (`DW-005`)

`web/src/features/dashboard/CardiacDriftPanel.tsx` SHALL
render only when the dependency check (HR streams present
for the queried range) resolves to `true`. When the check
resolves to `false`, the panel MUST be hidden from the DOM
AND SHALL display (or be replaced by) the empty-state label
`"necesita streams HR para calcular"` so users understand why
the drift is missing. This resolves the proposal's G5
decision: Cardiac Drift is gated by `activity_streams`
availability, not invented from `avg_hr` alone (E3).

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/CardiacDriftPanel.tsx`

#### Scenario: HR streams present renders the panel

- GIVEN the dependency check resolves to `hasHRStreams: true`
  for the queried range (the panel's own fetch / header
  returns a non-empty HR-stream indicator)
- WHEN `<CardiacDriftPanel>` is rendered
- THEN the panel container (testid `cardiac-drift-panel`) is
  in the document
- AND the empty-state label `"necesita streams HR para
  calcular"` is NOT in the document

#### Scenario: HR streams absent hides the panel and shows the empty-state label

- GIVEN the dependency check resolves to `hasHRStreams: false`
- WHEN `<CardiacDriftPanel>` is rendered (or its parent
  decides whether to mount it)
- THEN the panel container is NOT in the document
- AND the empty-state label `"necesita streams HR para
  calcular"` (testid `cardiac-drift-empty`) IS in the document

### Requirement: Empty state for users with no activities yet (`DW-006`)

When the dashboard receives an empty result (no activities,
empty `weekly_*`, zero series), the page MUST render the
existing `web/src/ui/EmptyState/` component with a copy
appropriate for first-time users (e.g. "Sincroniza tus
actividades de Strava para empezar" — exact copy is a
product decision; the requirement is that the existing
`EmptyState` component is reused, not a new empty-state
invented). The page MUST NOT render the three charts in this
case (avoid noisy empty charts).

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/routes/dashboard.tsx` empty branch

#### Scenario: no activities yields the EmptyState and no charts

- GIVEN a user with zero activities
- WHEN `/dashboard` is rendered
- THEN the `EmptyState` component (testid `dashboard-empty-state`)
  is in the document
- AND `TrainingLoadChart`, `HRZonesChart`, and
  `DashboardSummaryCard` are NOT in the document (or their
  populated branches are not rendered)

### Requirement: Container + presentational pattern with Vitest tests (`DW-007`)

Each feature under `web/src/features/dashboard/` MUST follow
the existing container + presentational split (verified in
`web/src/features/activities/` and `web/src/features/gpx/`):
a `*Container.tsx` (or `useXxx` hook) owns data fetching,
loading, and error state; a sibling presentational component
owns pure rendering. Tests SHALL live in
`*.test.ts(x)` files alongside the component and MUST cover
the populated state, the empty state, and (where applicable)
the loading and error states using Vitest +
`@testing-library/react` + the existing msw / mock fetcher
setup from the SPA.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/dashboard/`

#### Scenario: the container delegates rendering to the presentational component

- GIVEN `DashboardSummaryCard` is split into
  `DashboardSummaryCardContainer.tsx` (data fetching) and
  `DashboardSummaryCard.tsx` (presentational)
- WHEN `pnpm -C web test:run` runs
- THEN there is at least one test file per component with
  populated, empty, loading, and (where applicable) error
  scenarios
- AND the presentational component's test mocks the data hook
  directly (not the network), demonstrating the separation

#### Scenario: all dashboard tests pass under the project test command

- GIVEN the PR4 code change set
- WHEN `pnpm -C web test:run` runs
- THEN all dashboard-feature tests pass
- AND `pnpm -C web typecheck` and `pnpm -C web lint` pass
  over the changed files
