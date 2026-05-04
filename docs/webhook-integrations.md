# gogomail webhook integrations

This document describes the disabled-by-default webhook contracts used to
connect gogomail to external attachment scanners and push notification
gateways. These integrations are process-boundary adapters; SMTP, storage, and
Mail API code should keep depending on gogomail interfaces instead of vendor
SDKs.

## Shared runtime behavior

- Webhook backends are disabled unless their backend environment variable is
  set to `webhook`.
- Production deployments require `https` webhook URLs. Local development and
  private test harnesses may use `http`.
- Optional bearer tokens are sent as `Authorization: Bearer <token>`.
- Bearer tokens are trimmed, must not contain line breaks, and are capped at
  4096 bytes before the adapter is wired.
- Timeout configuration is per adapter and defaults to `2s`.

## Attachment scanner

Enable the synchronous SMTP-stage scanner:

```bash
GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL=https://scanner.example/scan
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT=2s
```

The scanner is called only after a message has been parsed and only when
attachments are present. It is available to `edge-mta`, `inbound-mta`, and
`outbound-mta`, but remains outside SMTP protocol core. The webhook request
normalizes and bounds message, address, subject, recipient, and attachment
metadata before JSON serialization. Recipient arrays are capped at 500 entries,
attachment metadata arrays are capped at 200 entries, and negative message
sizes are sent as `0`.

gogomail sends a JSON `POST` request:

```json
{
  "remote_addr": "203.0.113.10:42312",
  "envelope_from": "sender@example.net",
  "recipients": ["user@example.com"],
  "company_id": "company-123",
  "domain_id": "domain-123",
  "user_id": "user-123",
  "submission_user": "user-123",
  "message_id": "<message@example.net>",
  "subject": "Quarterly report",
  "size": 18432,
  "attachments": [
    {
      "filename": "report.pdf"
    }
  ]
}
```

The scanner must return a 2xx response with a JSON body:

```json
{
  "verdict": "accept",
  "reason": "clean"
}
```

Supported verdicts are:

- `accept`: continue SMTP processing.
- `reject`: reject the message at the scan stage.
- `tempfail`: return a temporary SMTP failure at the scan stage.

Response bodies are capped at 64 KiB before JSON decoding. Trailing JSON tokens
are rejected. Rejection and tempfail reasons are CR/LF-stripped and UTF-8 safely
bounded before they are surfaced as SMTP hook errors.

## Push notification gateway

Enable the asynchronous push gateway handoff:

```bash
GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL=https://push-gateway.example/send
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT=2s
```

`push-notification-worker` consumes committed `mail.stored` events, resolves
active user devices from PostgreSQL, records one candidate attempt per device,
and then invokes the configured sink. Device targets are bounded by
`GOGOMAIL_PUSH_NOTIFICATION_DEVICE_LIMIT` and malformed device IDs, tokens, or
platforms are dropped before sink handoff. The webhook sink also normalizes and
bounds message, recipient, subject, timestamp, and target metadata before JSON
serialization, and drops direct-call targets with blank, CR/LF-bearing,
oversized, or unsupported platform/token values.

gogomail sends a JSON `POST` request:

```json
{
  "message_id": "message-123",
  "rfc_message_id": "<message@example.net>",
  "company_id": "company-123",
  "domain_id": "domain-123",
  "user_id": "user-123",
  "recipient": "user@example.com",
  "subject": "New mail",
  "received_at": "2026-05-04T12:30:00Z",
  "targets": [
    {
      "attempt_id": "attempt-123",
      "device_id": "device-123",
      "platform": "fcm",
      "token": "raw-provider-token",
      "token_suffix": "token",
      "label": "Pixel"
    }
  ]
}
```

The gateway can return any 2xx response to acknowledge that it accepted the
handoff. gogomail does not parse the response body. After a successful handoff,
the worker marks candidate attempts as `queued`; non-2xx responses or transport
errors mark the attempts as `failed` with a bounded diagnostic and cause the
stream handler to return an error for retry.

First-party FCM, APNs, and Web Push delivery adapters are intentionally future
work. Until those adapters exist, provider-specific delivered, failed, and
invalid-token outcomes should be recorded by a future adapter or an operator
workflow that updates `push_notification_attempts` through a dedicated backend
surface.
