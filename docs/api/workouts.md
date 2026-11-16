# Planned workouts API

All routes are under `/api/v1` and require authentication. Dates are civil dates in `YYYY-MM-DD` format, parsed strictly. Query `from` and `to` are required; both endpoints of the range are inclusive, and `to` must not precede `from`. Results are ordered by `scheduled_on` ascending, then `id` ascending.

- `POST /workouts`: create a session. Required fields: `scheduled_on`, `title`, and `workout_type`. Optional fields: `notes`, `target_duration_sec`, `target_distance_m`, `target_elevation_gain_m`, and `target_intensity`. Omitted optional targets are unplanned (`null` in responses); `notes` defaults to an empty string. Target values must be non-negative signed 32-bit integers. Returns `201` and the session JSON.
- `GET /workouts?from=2026-11-01&to=2026-11-30&limit=20&offset=0`: list sessions in the inclusive date range. `limit` defaults to 20 and must be 1–100; `offset` defaults to 0 and must be a non-negative signed 32-bit integer. Returns `{data,total,limit,offset,has_next}`; `data` is an array, including an empty array when the page is past the end.
- `GET /workouts/{id}`: retrieve one session by UUID.
- `PATCH /workouts/{id}`: update supplied fields only. Omitted fields are preserved; nullable target fields may be set to `null` to clear them. `scheduled_on`, `title`, and `workout_type` cannot be null. Returns the updated session.
- `DELETE /workouts/{id}`: delete a session; returns `204` with no body.

Session JSON has `id`, `scheduled_on`, `title`, `notes`, `workout_type`, `target_duration_sec`, `target_distance_m`, `target_elevation_gain_m`, and `target_intensity`. Numeric targets serialize as plain JSON integers or `null`, never database driver values.

Malformed JSON, invalid dates, invalid ranges, and invalid pagination parameters return `400`. An invalid resource UUID returns `404`, as do missing and other-user sessions. Domain validation errors (empty/oversized title or notes, unknown workout type/intensity, negative target) return `422`. Oversized request bodies return `413`. Errors use RFC 9457 `application/problem+json`.
