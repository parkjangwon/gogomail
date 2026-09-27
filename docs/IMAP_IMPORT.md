# IMAP mailbox import (tenant onboarding)

`gogomail imap-import` migrates messages from a source IMAP server (Gmail /
Google Workspace, Outlook / Microsoft 365, Fastmail, Dovecot, or any RFC
3501/9051 server) into a target gogomail user's mailbox. It preserves:

- **flags** — `\Seen` → read, `\Flagged` → starred, `\Answered` → replied,
  `$Forwarded` → forwarded; custom keywords are carried over as keywords.
  `\Recent` is intentionally dropped (session-scoped, not persistable).
- **internal dates** — the source `INTERNALDATE` is preserved on each message.
- **folder structure** — via a configurable mapping (prefix / merge /
  flat-with-labels).
- **Message-IDs for idempotency** — re-runs skip messages already imported.

The command follows the same self-contained subcommand pattern as
`gogomail backup` / `gogomail restore`: it reads storage/database wiring from
the standard config/env, connects to the source, and writes into gogomail using
the same IMAP APPEND path the running server uses.

> Out of scope (documented follow-ups): **contacts and calendar import** and
> **delta sync after cutover**. See [Out of scope](#out-of-scope) below.

---

## Quick start

```bash
export IMAP_IMPORT_PASSWORD='app-password-here'   # never passed on the CLI

gogomail imap-import \
  --config /etc/gogomail/config.yaml \
  --host imap.gmail.com \
  --username alice@oldcompany.com \
  --target-user alice@newtenant.io \
  --mapping prefix --prefix Imported \
  --rate 20 \
  --resume-file /var/lib/gogomail/import-alice.json
```

Always **dry-run first** to see counts and sizes without writing anything:

```bash
gogomail imap-import --host imap.gmail.com --username alice@oldcompany.com \
  --target-user alice@newtenant.io --dry-run
```

---

## Security

- **Credentials are read from the environment or a secret file only** — never
  from CLI flags, and never written to stdout/stderr or logs:

  | Variable | Meaning |
  |---|---|
  | `IMAP_IMPORT_PASSWORD` | source account password / app-password |
  | `IMAP_IMPORT_PASSWORD_FILE` | path to a file containing the password |
  | `IMAP_IMPORT_OAUTH2_TOKEN` | OAuth2 access token (SASL XOAUTH2) |
  | `IMAP_IMPORT_OAUTH2_TOKEN_FILE` | path to a file containing the token |

- **TLS is required by default.** The client connects with implicit TLS
  (port 993). Alternatives:
  - `--starttls` — connect in cleartext then upgrade with STARTTLS.
  - `--insecure-tls` — skip certificate verification (explicit opt-in; testing
    against self-signed servers only).
  - `--allow-insecure` — permit a plaintext connection with no TLS at all
    (explicit opt-in; lab use only). Without one of `--tls`/`--starttls`/
    `--allow-insecure`, the command refuses to connect.

---

## Folder mapping

The source hierarchy is projected onto gogomail's per-user folder model with
`--mapping`:

| Strategy | Behavior | Flags |
|---|---|---|
| `prefix` (default) | Each source folder becomes its own destination folder. The source hierarchy separator is normalized to `/` in the folder name. `--prefix` prepends a common prefix. | `--prefix`, `--separator` |
| `merge` | Every source folder collapses into a single destination folder. | `--merge-target` (default `INBOX`) |
| `flat-labels` | Each folder is flattened to its leaf name; the full source path is added to every message as a `Label_...` keyword so no organizational signal is lost. | `--separator` |

`INBOX` always maps to `INBOX` regardless of strategy (RFC 3501 §5.1).

Virtual/duplicate containers are skipped automatically to avoid importing every
message twice — notably Gmail's `[Gmail]/All Mail` and `[Gmail]/Important`, and
`\Noselect` hierarchy nodes.

### Examples

```bash
# Keep the original tree under an "Imported/" prefix:
--mapping prefix --prefix Imported
#   Work/Projects  → Imported/Work/Projects
#   Sent           → Imported/Sent

# Flatten everything into the inbox:
--mapping merge --merge-target INBOX

# Flatten to leaf folders but tag each message with its source path:
--mapping flat-labels
#   Work/Projects/Q1  → folder "Q1", keyword "Label_Work_Projects_Q1"

# Dovecot-style dotted hierarchy:
--mapping prefix --separator .
```

---

## Robustness: concurrency, rate limiting, resume

- `--concurrency N` — import N folders in parallel. **Default 1.** A single
  IMAP connection cannot safely interleave `SELECT`s, so raising this against a
  single source connection is only useful when the source tolerates it; the sink
  side (storage + DB) is always parallel-safe.
- `--rate R` — cap messages processed per second across the run (0 = unlimited).
  Use this to respect provider throttling (see per-provider notes).
- `--command-timeout D` — per-command timeout on the source connection
  (default 2m) to survive slow servers without hanging forever.
- `--resume-file PATH` — persist per-message and per-folder progress to a JSON
  file. Re-running with the same file **skips already-imported messages and
  fully-drained folders**. The file is written atomically (temp + rename) so an
  interrupted write cannot corrupt resume state.

### Resume procedure

1. Start the import with `--resume-file /path/to/progress.json`.
2. If the run is interrupted (Ctrl-C, `SIGTERM`, network drop), progress is
   saved. The command exits non-zero and prints a resume hint.
3. Re-run the **exact same command**. Already-imported messages (matched by
   Message-ID) and completed folders are skipped; the import continues where it
   left off.

Idempotency is enforced at three layers:

1. **Same run** — an in-memory set de-dupes messages that appear in multiple
   folders (e.g. Gmail labels).
2. **Across runs** — the resume file records every imported Message-ID.
3. **Server-side safety net** — before storing, the sink checks whether the
   target user already has a message with that RFC Message-ID (covers a crash
   where committed inserts outran the progress file). Messages with no usable
   Message-ID fall back to a SHA-256 content hash so they are still de-duped.

---

## Provider-specific notes

### Gmail / Google Workspace

- Host `imap.gmail.com`, port `993`, implicit TLS.
- **App password or OAuth2 required.** With 2-Step Verification, create an
  [App Password](https://support.google.com/accounts/answer/185833) and pass it
  via `IMAP_IMPORT_PASSWORD`. For OAuth2, obtain an access token with the
  `https://mail.google.com/` scope and use `IMAP_IMPORT_OAUTH2_TOKEN`
  (SASL XOAUTH2).
- **IMAP must be enabled** in Gmail settings.
- **Labels vs folders:** Gmail exposes labels as IMAP folders and every message
  also appears under `[Gmail]/All Mail`. The importer skips `[Gmail]/All Mail`
  and `[Gmail]/Important` automatically. With `--mapping prefix` a message with
  multiple labels is imported once per label folder but de-duped by Message-ID,
  so it lands in the first-processed folder only. Use `--mapping flat-labels`
  to instead keep one copy and record labels as keywords.
- **Throttling:** Gmail enforces bandwidth and simultaneous-connection limits.
  Keep `--concurrency 1` and set `--rate 20`–`--rate 40` for large mailboxes to
  avoid `Too many simultaneous connections` / temporary lockouts.

### Outlook.com / Microsoft 365

- Host `outlook.office365.com`, port `993`, implicit TLS.
- **Modern Auth (OAuth2) is required for most tenants** — basic auth is
  disabled by default. Acquire a token for the `IMAP.AccessAsUser.All` scope and
  pass `IMAP_IMPORT_OAUTH2_TOKEN`. IMAP access must be enabled on the mailbox.
- **Throttling:** Exchange Online applies per-mailbox request budgets. Use
  `--rate 10`–`--rate 20` and expect the occasional transient error; re-run with
  the resume file to continue.

### Fastmail

- Host `imap.fastmail.com`, port `993`, implicit TLS.
- **App password required** when 2FA is enabled — create one under Settings →
  Privacy & Security → App Passwords with IMAP access, and pass it via
  `IMAP_IMPORT_PASSWORD`.
- Fastmail uses `/` as the hierarchy separator (the default). Throttling is
  generous; `--rate` is usually unnecessary.

### Generic Dovecot / other servers

- Set `--host`/`--port` and `--separator` (`.` for many Dovecot setups).
- Use `--starttls` if the server only offers STARTTLS on port 143.

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `refusing to connect without TLS` | You disabled TLS without an explicit opt-in. Use `--tls` (default), `--starttls`, or `--allow-insecure` (lab only). |
| `login: imap command failed: ... AUTHENTICATIONFAILED` | Wrong password, or the provider requires an app-password/OAuth2 (Gmail/M365). |
| `xoauth2 authenticate` failures | Token expired or wrong scope; mint a fresh token. |
| `Too many simultaneous connections` (Gmail) | Lower `--concurrency` to 1 and set `--rate`. |
| `target user "…" not found` | The `--target-user` email must be an active gogomail user with a primary address. |
| Import seems to "re-do" work | Ensure you pass the **same** `--resume-file` on every run. |
| Certificate errors against a private server | Use `--insecure-tls` only if you understand the risk, or install the CA. |
| Duplicate messages after switching mapping | Message-IDs de-dupe within a target user; switching strategies mid-migration can create differently-named folders. Pick one strategy per user. |

Enable a dry-run (`--dry-run`) any time to inspect what *would* be imported
without touching the target mailbox.

---

## Out of scope

These are explicit follow-ups, not handled by `imap-import`:

- **Contacts and calendar import.** IMAP only carries mail. Migrating contacts
  (CardDAV / vCard) and calendars (CalDAV / iCalendar) is a separate effort;
  gogomail already speaks CardDAV/CalDAV, so a future `carddav-import` /
  `caldav-import` tool (or a one-shot vCard/ICS loader) is the natural home.
- **Delta sync after cutover.** `imap-import` is a bulk, point-in-time copy.
  Messages that arrive at the *source* after the import starts are not
  continuously synced. The recommended manual cutover is:
  1. Run the import (with `--resume-file`).
  2. Switch MX / mail delivery to gogomail so new mail lands here directly.
  3. Re-run the import once more with the same resume file to sweep up anything
     that arrived at the source between step 1 and step 2 (idempotency ensures
     only the new messages are added).
  4. Decommission the source mailbox.

  A live, ongoing source→gogomail mirror (IMAP IDLE / QRESYNC-based delta sync)
  is intentionally not provided.
