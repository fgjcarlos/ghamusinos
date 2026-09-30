# Sync Report: phase-1-3-km-vertical-and-king-climb

## Type of sync

Archive-time sync fallback. The `sdd-sync` phase was not run
during the apply/verify split (the slice uses a 2-PR chain with
no formal `sdd-sync` gate between rounds). The parent orchestrator
explicitly authorised archive-time sync fallback per the
`sdd-archive` contract ("archive may perform the same file-backed
sync only when the parent prompt explicitly approves archive-time
sync fallback").

## Domains synced

| Domain | Canonical spec (before) | Canonical spec (after) | Delta spec |
| --- | --- | --- | --- |
| `gpx-backend` | 109 lines, 1 ADDED Requirement | 783 lines, 9 ADDED Requirements | 743 lines, 8 ADDED Requirements (all new) |
| `gpx-lab` | 386 lines, 7 ADDED Requirements | 753 lines, 14 ADDED Requirements | 431 lines, 7 ADDED Requirements (all new) |

## Operations applied

### gpx-backend

All 8 delta requirements were ADDED. No MODIFIED or REMOVED
operations were required:

1. `Migration creates three new climb-derived tables`
2. `SQLC queries for the three new tables`
3. `StoredTrackDetail carries muros, recovery_zones, km_vertical as top-level fields`
4. `CreateDetail persists muros, recovery zones, and km_vertical`
5. `GetDetail rehydrates muros, recovery zones, and km_vertical`
6. `SQLCStore exposes ListMuros, ListRecoveryZones, GetKmVertical accessor methods`
7. `Upload handler emits StoredTrackDetail directly with no orphan top-level keys`
8. `Tests pin the new wire shape, persistence, and rehydration`

Existing canonical requirement
`storedTrack propagates Analysis.ElevationCoverage` was preserved
verbatim and now precedes the eight new requirements.

Coverage Matrix grew from 1 entry to 9 entries.

### gpx-lab

All 7 delta requirements were ADDED. No MODIFIED or REMOVED
operations were required:

1. `StoredTrackDetail declares muros, recovery_zones, km_vertical`
2. `Normalizers handle muros, recovery_zones, km_vertical`
3. `RouteKmVertical component renders the km_vertical singleton card`
4. `RouteMuros component renders the muros list card`
5. `RouteRecovery component renders the recovery_zones list card`
6. `RouteDetail composes the three new panels`
7. `SPA reads from the persisted GET path, not the transient upload response`

All 7 pre-existing canonical requirements were preserved verbatim.

Coverage Matrix grew from 7 entries to 14 entries.

## Active same-domain change warnings

None. No other change under `openspec/changes/*/specs/{gpx-backend,gpx-lab}/spec.md`
touches these domains at archive time.

## Destructive merge approvals

None. No REMOVED requirements and no large MODIFIED blocks were
applied. The sync was purely additive (8 + 7 new requirements).

## Verification of the merge

Both canonical specs end with a valid Markdown structure:

- `# Delta for gpx-{backend,lab}(web)` header is preserved.
- `## Source`, `## Domain Metadata`, `## ADDED Requirements`,
  `## Out of Scope`, `## Coverage Matrix` sections are in order.
- `## ADDED Requirements` block contains exactly the expected
  number of `### Requirement: ...` headings (gpx-backend: 9,
  gpx-lab: 14).
- Coverage Matrix is well-formed Markdown (single table per
  block).

## Status

**PASS** — file-backed sync completed before folder move.