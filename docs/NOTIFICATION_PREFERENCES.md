# Notification Preferences

Per-user notification preferences are stored server-side and cached client-side.
They control whether browser/push notifications fire for newly-delivered mail,
globally (Do-Not-Disturb), on a per-folder basis, and on a per-thread basis.

## Endpoints

Both endpoints require an authenticated user (JWT bearer or API key with the
appropriate mail scope).

### `GET /api/v1/me/notification-preferences`

Returns the current user's preferences. If no row exists yet, returns defaults:
notifications enabled for every folder/thread, no DND, UTC timezone, empty
`folder_overrides`, and empty `thread_overrides`.

### `PUT /api/v1/me/notification-preferences`

Replaces the user's preference document. Request body uses the same shape as
the GET response (excluding `updated_at`). Unknown JSON keys are rejected
(`DisallowUnknownFields`). The response echoes the persisted state.

Rate limit: **30 requests per minute per user**. Bursts beyond the limit get
`429` with `Retry-After: 60`.

## Wire format

```json
{
  "global_dnd_enabled": true,
  "global_dnd_schedule": {
    "weekdays": [0, 6],
    "time_ranges": [{"start": "22:00", "end": "08:00"}],
    "timezone": "Asia/Seoul"
  },
  "folder_overrides": {
    "<folder-uuid>": {
      "enabled": false,
      "dnd_inherit": true,
      "dnd_schedule": {
        "weekdays": [],
        "time_ranges": [],
        "timezone": ""
      }
    }
  },
  "thread_overrides": {
    "<thread-uuid>": {
      "enabled": false
    }
  },
  "updated_at": "2026-05-23T12:34:56Z"
}
```

### Schedule semantics

- `weekdays`: array of integers 0-6 (0 = Sunday, 6 = Saturday). DND fires only
  on listed days. Empty array means "no day matches" (DND never active by
  day-of-week).
- `time_ranges`: up to **8** entries. Each entry has `start` and `end` in
  24-hour `HH:MM` format. If `end <= start` the range crosses midnight (e.g.
  `{"start": "22:00", "end": "08:00"}` means 22:00 today through 08:00
  tomorrow). DND fires when the local time falls inside any range.
- `timezone`: an IANA tz name (e.g. `Asia/Seoul`, `UTC`). Empty string defaults
  to `UTC`. The server validates via `time.LoadLocation` and rejects unknown
  zones.

### Per-folder/thread vs global precedence

For a given message, notifications fire iff **all** of the following hold:

1. `folder_overrides[folder_id].enabled` is `true` (default `true` when no
   override is present).
2. `thread_overrides[thread_id].enabled` is `true` (default `true` when no
   override is present).
3. The folder is **not** currently in DND.

DND for a folder is evaluated as follows:

- If no override exists for the folder, or `dnd_inherit` is `true`: use the
  global schedule (`global_dnd_enabled` + `global_dnd_schedule`).
- If `dnd_inherit` is `false`: use the per-folder `dnd_schedule` directly.

Thus `folder.enabled = false` silences a folder unconditionally, and
`thread.enabled = false` silences a single conversation inside otherwise
enabled folders. `dnd_inherit` lets the user pick between the global schedule
and a local one.

## Limits

- Maximum 8 `time_ranges` per schedule (global or per-folder).
- Maximum 200 entries in `folder_overrides`.
- Maximum 500 entries in `thread_overrides`.
- Folder and thread IDs must be valid UUIDs.

## Storage

Persisted in the `notification_preferences` table (migration 0149), with
`thread_overrides` added by migration 0150. The row is cascaded on user delete.
`updated_at` is set by the database on every upsert and surfaced in the API
response for client-cache invalidation.

## Default behavior

A brand-new account with no preferences set behaves as if all notifications
are on and no DND is active. The client should treat a missing row (returned
as defaults by the API) and an explicit "all defaults" row identically.
