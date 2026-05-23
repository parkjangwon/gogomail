# Web Push Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** End-to-end Web Push (RFC 8030/8291) — 탭이 닫혀 있어도 새 메일 알림을 브라우저에 전달한다.

**Architecture:** 새 `web_push_subscriptions` PostgreSQL 테이블에 VAPID 구독 정보를 저장하고, `push-notification-worker`가 `mail.stored`/`mail.delivery_exhausted` 이벤트를 소비해 `github.com/SherClockHolmes/webpush-go`로 aes128gcm 암호화 후 브라우저 푸시 서비스로 전송. 프론트엔드는 `useWebPush` 훅이 Service Worker를 등록하고 구독을 저장.

**Tech Stack:** Go · `github.com/SherClockHolmes/webpush-go` · PostgreSQL · Next.js · `lib/webpush.ts` (base64url 헬퍼 기존)

**Spec:** `docs/superpowers/specs/2026-05-23-web-push-notifications-design.md`

---

## File Map

| 파일 | 작업 |
|---|---|
| `migrations/0152_web_push_subscriptions.sql` | 신규 생성 |
| `internal/maildb/web_push_subscriptions.go` | 신규 생성 |
| `internal/maildb/web_push_subscriptions_test.go` | 신규 생성 |
| `internal/pushnotify/handler.go` | EnvelopeFrom 필드 추가 |
| `internal/pushnotify/webpush_sink.go` | 신규 생성 |
| `internal/pushnotify/webpush_sink_test.go` | 신규 생성 |
| `internal/pushnotify/delivery_handler.go` | 신규 생성 |
| `internal/config/validate.go` | webpush enum + VAPID 검증 추가 |
| `internal/app/run.go` | WebPushSink 연결 + 이벤트 라우팅 |
| `internal/httpapi/mail.go` | push-subscriptions API 4개 + config endpoint |
| `apps/webmail/src/hooks/useWebPush.ts` | 신규 생성 |
| `apps/webmail/src/components/Providers.tsx` | useWebPush 호출 |
| `apps/webmail/src/components/settings-view/SettingsNotificationsSection.tsx` | 백그라운드 푸시 토글 추가 |
| `apps/webmail/messages/en.json` | i18n 키 추가 |
| `apps/webmail/messages/ko.json` | i18n 키 추가 |

---

## Task 1: DB Migration — web_push_subscriptions

**Goal:** `web_push_subscriptions` 테이블과 인덱스를 생성하는 마이그레이션 파일을 만든다.

**Files:**
- Create: `migrations/0152_web_push_subscriptions.sql`

**Acceptance Criteria:**
- [ ] 마이그레이션 파일이 `0152_` prefix로 존재한다
- [ ] 테이블에 `id`, `user_id`, `endpoint`, `p256dh`, `auth`, `user_agent`, `status`, `created_at`, `updated_at` 컬럼이 있다
- [ ] endpoint 유니크 인덱스 (`status = 'active'`) 가 있다
- [ ] user_id 파셜 인덱스 (`status = 'active'`) 가 있다
- [ ] `go test ./...` 통과

**Verify:** `ls migrations/0152_web_push_subscriptions.sql && go test ./...` → ok

**Steps:**

- [ ] **Step 1: 마이그레이션 파일 생성**

```sql
-- migrations/0152_web_push_subscriptions.sql
CREATE TABLE web_push_subscriptions (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   TEXT        NOT NULL,
  p256dh     TEXT        NOT NULL,
  auth       TEXT        NOT NULL,
  user_agent TEXT,
  status     TEXT        NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX web_push_subscriptions_endpoint_active_idx
  ON web_push_subscriptions(endpoint)
  WHERE status = 'active';

CREATE INDEX web_push_subscriptions_user_active_idx
  ON web_push_subscriptions(user_id)
  WHERE status = 'active';
```

- [ ] **Step 2: 테스트 통과 확인**

```bash
go test ./...
```

Expected: `ok` (마이그레이션 파일은 Go 컴파일에 영향 없음)

- [ ] **Step 3: 커밋**

```bash
git add migrations/0152_web_push_subscriptions.sql
git commit -m "feat: add web_push_subscriptions migration"
```

---

## Task 2: DB Layer — web_push_subscriptions CRUD

**Goal:** `web_push_subscriptions` 테이블의 upsert/list/delete/soft-delete를 제공하는 DB 레이어를 구현한다.

**Files:**
- Create: `internal/maildb/web_push_subscriptions.go`
- Create: `internal/maildb/web_push_subscriptions_test.go`

**Acceptance Criteria:**
- [ ] `UpsertWebPushSubscription()` — endpoint 기준 INSERT ON CONFLICT UPDATE
- [ ] `ListActiveWebPushSubscriptions()` — user_id 기준 활성 구독 목록
- [ ] `DeleteWebPushSubscription()` — user_id + id로 soft-delete
- [ ] `SoftDeleteWebPushSubscriptionByEndpoint()` — 410 Gone 처리용
- [ ] 유닛 테스트 통과
- [ ] `go test ./internal/maildb/...` 통과

**Verify:** `go test ./internal/maildb/... -run TestWebPushSubscription -v` → PASS

**Steps:**

- [ ] **Step 1: 실패 테스트 작성**

```go
// internal/maildb/web_push_subscriptions_test.go
package maildb_test

import (
    "testing"
    "github.com/gogomail/gogomail/internal/maildb"
)

func TestWebPushSubscriptionRequest_Validate(t *testing.T) {
    tests := []struct{
        name    string
        req     maildb.UpsertWebPushSubscriptionRequest
        wantErr bool
    }{
        {
            name: "valid",
            req: maildb.UpsertWebPushSubscriptionRequest{
                UserID:   "11111111-1111-1111-1111-111111111111",
                Endpoint: "https://updates.push.services.mozilla.com/wpush/v2/abc123",
                P256DH:   "BNcRdreALRFXTkOOUHK1EtK2wtCONKTMl7aEkYaSs8k",
                Auth:     "tBHItJI5svbpez7KI4CCXg",
            },
        },
        {
            name:    "missing user_id",
            req:     maildb.UpsertWebPushSubscriptionRequest{Endpoint: "https://example.com", P256DH: "a", Auth: "b"},
            wantErr: true,
        },
        {
            name:    "missing endpoint",
            req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", P256DH: "a", Auth: "b"},
            wantErr: true,
        },
        {
            name:    "endpoint not https",
            req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "http://example.com", P256DH: "a", Auth: "b"},
            wantErr: true,
        },
        {
            name:    "missing p256dh",
            req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "https://example.com", Auth: "b"},
            wantErr: true,
        },
        {
            name:    "missing auth",
            req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "https://example.com", P256DH: "a"},
            wantErr: true,
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.req.Validate()
            if (err != nil) != tc.wantErr {
                t.Fatalf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
            }
        })
    }
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
go test ./internal/maildb/... -run TestWebPushSubscriptionRequest_Validate -v
```

Expected: FAIL (함수 없음)

- [ ] **Step 3: 구현**

```go
// internal/maildb/web_push_subscriptions.go
package maildb

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "strings"
    "time"
)

// WebPushSubscription is an active browser push subscription.
type WebPushSubscription struct {
    ID        string
    UserID    string
    Endpoint  string
    P256DH    string
    Auth      string
    UserAgent string
    Status    string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// UpsertWebPushSubscriptionRequest is the input for UpsertWebPushSubscription.
type UpsertWebPushSubscriptionRequest struct {
    UserID    string
    Endpoint  string
    P256DH    string
    Auth      string
    UserAgent string
}

// Validate checks required fields.
func (r *UpsertWebPushSubscriptionRequest) Validate() error {
    r.UserID = strings.TrimSpace(r.UserID)
    r.Endpoint = strings.TrimSpace(r.Endpoint)
    r.P256DH = strings.TrimSpace(r.P256DH)
    r.Auth = strings.TrimSpace(r.Auth)
    r.UserAgent = strings.TrimSpace(r.UserAgent)
    if r.UserID == "" {
        return fmt.Errorf("user_id is required")
    }
    if strings.ContainsAny(r.UserID, "\r\n") {
        return fmt.Errorf("user_id must not contain line breaks")
    }
    if r.Endpoint == "" {
        return fmt.Errorf("endpoint is required")
    }
    if !strings.HasPrefix(r.Endpoint, "https://") {
        return fmt.Errorf("endpoint must be an HTTPS URL")
    }
    if len(r.Endpoint) > 2048 {
        return fmt.Errorf("endpoint must not exceed 2048 characters")
    }
    if strings.ContainsAny(r.Endpoint, "\r\n") {
        return fmt.Errorf("endpoint must not contain line breaks")
    }
    if r.P256DH == "" {
        return fmt.Errorf("p256dh is required")
    }
    if r.Auth == "" {
        return fmt.Errorf("auth is required")
    }
    return nil
}

// UpsertWebPushSubscription inserts or updates a web push subscription by endpoint.
func (r *Repository) UpsertWebPushSubscription(ctx context.Context, req UpsertWebPushSubscriptionRequest) (WebPushSubscription, error) {
    if err := req.Validate(); err != nil {
        return WebPushSubscription{}, err
    }
    var sub WebPushSubscription
    err := r.db.QueryRowContext(ctx, `
        INSERT INTO web_push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, status)
        VALUES ($1, $2, $3, $4, $5, 'active')
        ON CONFLICT (endpoint) WHERE status = 'active'
        DO UPDATE SET
            p256dh     = EXCLUDED.p256dh,
            auth       = EXCLUDED.auth,
            user_agent = EXCLUDED.user_agent,
            updated_at = now()
        RETURNING id, user_id, endpoint, p256dh, auth, COALESCE(user_agent, ''), status, created_at, updated_at
    `, req.UserID, req.Endpoint, req.P256DH, req.Auth, req.UserAgent).Scan(
        &sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth,
        &sub.UserAgent, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt,
    )
    if err != nil {
        return WebPushSubscription{}, fmt.Errorf("upsert web push subscription: %w", err)
    }
    return sub, nil
}

// ListActiveWebPushSubscriptions returns active subscriptions for a user.
func (r *Repository) ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]WebPushSubscription, error) {
    userID = strings.TrimSpace(userID)
    if userID == "" {
        return nil, fmt.Errorf("user_id is required")
    }
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, endpoint, p256dh, auth, COALESCE(user_agent, ''), status, created_at, updated_at
        FROM web_push_subscriptions
        WHERE user_id = $1 AND status = 'active'
        ORDER BY created_at DESC
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("list web push subscriptions: %w", err)
    }
    defer rows.Close()
    var subs []WebPushSubscription
    for rows.Next() {
        var sub WebPushSubscription
        if err := rows.Scan(
            &sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth,
            &sub.UserAgent, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("scan web push subscription: %w", err)
        }
        subs = append(subs, sub)
    }
    return subs, rows.Err()
}

// DeleteWebPushSubscription soft-deletes a subscription owned by userID.
func (r *Repository) DeleteWebPushSubscription(ctx context.Context, userID, id string) error {
    userID = strings.TrimSpace(userID)
    id = strings.TrimSpace(id)
    if userID == "" || id == "" {
        return fmt.Errorf("user_id and id are required")
    }
    result, err := r.db.ExecContext(ctx, `
        UPDATE web_push_subscriptions
        SET status = 'deleted', updated_at = now()
        WHERE user_id = $1 AND id = $2 AND status = 'active'
    `, userID, id)
    if err != nil {
        return fmt.Errorf("delete web push subscription: %w", err)
    }
    n, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("delete web push subscription rows affected: %w", err)
    }
    if n == 0 {
        return fmt.Errorf("web push subscription %q not found", id)
    }
    return nil
}

// SoftDeleteWebPushSubscriptionByEndpoint soft-deletes a subscription by its endpoint.
// Used when the push service returns 410 Gone (subscription expired).
func (r *Repository) SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
    endpoint = strings.TrimSpace(endpoint)
    if endpoint == "" {
        return fmt.Errorf("endpoint is required")
    }
    _, err := r.db.ExecContext(ctx, `
        UPDATE web_push_subscriptions
        SET status = 'deleted', updated_at = now()
        WHERE endpoint = $1 AND status = 'active'
    `, endpoint)
    if err != nil {
        return fmt.Errorf("soft-delete web push subscription by endpoint: %w", err)
    }
    return nil
}

// webPushSubscriptionExists returns true when a push subscription exists and is not deleted.
func webPushSubscriptionExists(err error) bool {
    return !errors.Is(err, sql.ErrNoRows)
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
go test ./internal/maildb/... -run TestWebPushSubscriptionRequest_Validate -v
```

Expected: PASS

- [ ] **Step 5: 커밋**

```bash
git add internal/maildb/web_push_subscriptions.go internal/maildb/web_push_subscriptions_test.go
git commit -m "feat: add web_push_subscriptions DB layer"
```

---

## Task 3: WebPushSink — 실제 암호화 전송

**Goal:** `webpush-go` 라이브러리를 추가하고, `Sink` 인터페이스를 구현하는 `WebPushSink`를 만들어 실제 암호화 푸시를 전송한다. `mail.stored` Event에 `EnvelopeFrom` 필드를 추가한다.

**Files:**
- Modify: `internal/pushnotify/handler.go` (EnvelopeFrom 추가)
- Create: `internal/pushnotify/webpush_sink.go`
- Create: `internal/pushnotify/webpush_sink_test.go`
- Modify: `go.mod`, `go.sum` (webpush-go 추가)

**Acceptance Criteria:**
- [ ] `go get github.com/SherClockHolmes/webpush-go` 성공
- [ ] `Event.EnvelopeFrom` 필드가 `json:"envelope_from"` 태그로 추가됨
- [ ] `Notification.EnvelopeFrom` 필드 추가됨
- [ ] `WebPushSink`가 `Sink` 인터페이스 구현
- [ ] 410 Gone 응답 시 `SoftDeleteWebPushSubscriptionByEndpoint()` 호출
- [ ] `go test ./internal/pushnotify/...` 통과

**Verify:** `go test ./internal/pushnotify/... -run TestWebPushSink -v` → PASS

**Steps:**

- [ ] **Step 1: 의존성 추가**

```bash
cd /Users/pjw/dev/project/gogomail && go get github.com/SherClockHolmes/webpush-go
```

Expected: 성공 (go.mod, go.sum 갱신됨)

- [ ] **Step 2: Event + Notification에 EnvelopeFrom 추가**

`internal/pushnotify/handler.go`의 `Event` 구조체에 필드 추가:

```go
// Event 구조체에 추가 (기존 ReceivedAt 아래):
EnvelopeFrom string `json:"envelope_from"`
```

`Notification` 구조체에도 추가:

```go
// Notification 구조체에 추가 (기존 ReceivedAt 아래):
EnvelopeFrom string
```

`notificationFromEvent()` 함수에도 매핑 추가:

```go
func notificationFromEvent(event Event) Notification {
    return Notification{
        // ... 기존 필드 유지 ...
        EnvelopeFrom: event.EnvelopeFrom,
    }
}
```

- [ ] **Step 3: 실패 테스트 작성**

```go
// internal/pushnotify/webpush_sink_test.go
package pushnotify_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gogomail/gogomail/internal/pushnotify"
)

type fakeWebPushSubReader struct {
    subs []pushnotify.WebPushSubData
    deletedEndpoint string
}

func (f *fakeWebPushSubReader) ListActiveWebPushSubscriptions(_ context.Context, _ string) ([]pushnotify.WebPushSubData, error) {
    return f.subs, nil
}

func (f *fakeWebPushSubReader) SoftDeleteWebPushSubscriptionByEndpoint(_ context.Context, endpoint string) error {
    f.deletedEndpoint = endpoint
    return nil
}

func TestWebPushSink_EnqueuePush_NoSubscriptions(t *testing.T) {
    sink, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
        VAPIDPublicKey:  "BNcRdreALRFXTkOOUHK1EtK2wtCONKTMl7aEkYaSs8k",
        VAPIDPrivateKey: "d0zXjT4bOaXv1qVBUoV6yLBmgGZ8-1Q9pPaQJF-Qbio",
        ContactEmail:    "admin@example.com",
        DB:              &fakeWebPushSubReader{},
    })
    if err != nil {
        t.Fatalf("NewWebPushSink: %v", err)
    }
    err = sink.EnqueuePush(context.Background(), pushnotify.Notification{UserID: "u1"})
    if err != nil {
        t.Fatalf("EnqueuePush with no subs should not error: %v", err)
    }
}

func TestWebPushSink_EnqueuePush_GoneDeletesSubscription(t *testing.T) {
    server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusGone)
    }))
    defer server.Close()

    reader := &fakeWebPushSubReader{
        subs: []pushnotify.WebPushSubData{{
            ID:       "sub1",
            Endpoint: server.URL,
            P256DH:   "BNcRdreALRFXTkOOUHK1EtK2wtCONKTMl7aEkYaSs8k",
            Auth:     "tBHItJI5svbpez7KI4CCXg",
        }},
    }
    sink, err := pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
        VAPIDPublicKey:  "BNcRdreALRFXTkOOUHK1EtK2wtCONKTMl7aEkYaSs8k",
        VAPIDPrivateKey: "d0zXjT4bOaXv1qVBUoV6yLBmgGZ8-1Q9pPaQJF-Qbio",
        ContactEmail:    "admin@example.com",
        DB:              reader,
        HTTPClient:      server.Client(),
    })
    if err != nil {
        t.Fatalf("NewWebPushSink: %v", err)
    }
    _ = sink.EnqueuePush(context.Background(), pushnotify.Notification{UserID: "u1", Subject: "Test"})
    if reader.deletedEndpoint != server.URL {
        t.Errorf("expected endpoint to be soft-deleted, got %q", reader.deletedEndpoint)
    }
}
```

- [ ] **Step 4: 테스트 실패 확인**

```bash
go test ./internal/pushnotify/... -run TestWebPushSink -v
```

Expected: FAIL (함수/타입 없음)

- [ ] **Step 5: WebPushSink 구현**

```go
// internal/pushnotify/webpush_sink.go
package pushnotify

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "strings"

    webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushSubData holds keys needed to send an encrypted push notification.
type WebPushSubData struct {
    ID       string
    Endpoint string
    P256DH   string
    Auth     string
}

// WebPushSubReader reads web push subscriptions from storage.
type WebPushSubReader interface {
    ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]WebPushSubData, error)
    SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
}

// WebPushSinkOptions configures a WebPushSink.
type WebPushSinkOptions struct {
    VAPIDPublicKey  string
    VAPIDPrivateKey string
    ContactEmail    string
    DB              WebPushSubReader
    HTTPClient      *http.Client
    Logger          *slog.Logger
}

// WebPushSink implements Sink by sending encrypted Web Push notifications.
type WebPushSink struct {
    vapidPublicKey  string
    vapidPrivateKey string
    contactEmail    string
    db              WebPushSubReader
    client          *http.Client
    logger          *slog.Logger
}

// NewWebPushSink creates a WebPushSink.
func NewWebPushSink(opts WebPushSinkOptions) (*WebPushSink, error) {
    opts.VAPIDPublicKey = strings.TrimSpace(opts.VAPIDPublicKey)
    opts.VAPIDPrivateKey = strings.TrimSpace(opts.VAPIDPrivateKey)
    if opts.VAPIDPublicKey == "" {
        return nil, fmt.Errorf("webpush: VAPID public key is required")
    }
    if opts.VAPIDPrivateKey == "" {
        return nil, fmt.Errorf("webpush: VAPID private key is required")
    }
    if opts.DB == nil {
        return nil, fmt.Errorf("webpush: DB reader is required")
    }
    client := opts.HTTPClient
    if client == nil {
        client = http.DefaultClient
    }
    logger := opts.Logger
    if logger == nil {
        logger = slog.Default()
    }
    sub := opts.ContactEmail
    if sub != "" && !strings.HasPrefix(sub, "mailto:") {
        sub = "mailto:" + sub
    }
    return &WebPushSink{
        vapidPublicKey:  opts.VAPIDPublicKey,
        vapidPrivateKey: opts.VAPIDPrivateKey,
        contactEmail:    sub,
        db:              opts.DB,
        client:          client,
        logger:          logger,
    }, nil
}

// EnqueuePush sends an encrypted push notification to all active subscriptions for the user.
func (s *WebPushSink) EnqueuePush(ctx context.Context, n Notification) error {
    subs, err := s.db.ListActiveWebPushSubscriptions(ctx, n.UserID)
    if err != nil {
        return fmt.Errorf("webpush: list subscriptions: %w", err)
    }
    if len(subs) == 0 {
        return nil
    }
    b, err := json.Marshal(map[string]string{
        "title": webPushTitle(n),
        "body":  n.Subject,
        "tag":   webPushTag(n),
        "url":   "/mail",
    })
    if err != nil {
        return fmt.Errorf("webpush: marshal payload: %w", err)
    }
    for _, sub := range subs {
        s.sendToSubscription(ctx, sub, b)
    }
    return nil
}

func (s *WebPushSink) sendToSubscription(ctx context.Context, sub WebPushSubData, body []byte) {
    resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
        Endpoint: sub.Endpoint,
        Keys: webpush.Keys{
            Auth:   sub.Auth,
            P256dh: sub.P256DH,
        },
    }, &webpush.Options{
        HTTPClient:      s.client,
        VAPIDPublicKey:  s.vapidPublicKey,
        VAPIDPrivateKey: s.vapidPrivateKey,
        Subscriber:      s.contactEmail,
        TTL:             86400,
        Urgency:         webpush.UrgencyNormal,
    })
    if err != nil {
        s.logger.Warn("webpush send error",
            "endpoint_suffix", endpointSuffix(sub.Endpoint),
            "error", err,
        )
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusGone {
        if delErr := s.db.SoftDeleteWebPushSubscriptionByEndpoint(ctx, sub.Endpoint); delErr != nil {
            s.logger.Warn("webpush soft-delete gone subscription", "error", delErr)
        }
    }
}

func webPushTitle(n Notification) string {
    from := strings.TrimSpace(n.EnvelopeFrom)
    if from != "" {
        return from
    }
    if n.Recipient != "" {
        return n.Recipient
    }
    return "새 메일"
}

func webPushTag(n Notification) string {
    if n.MessageID != "" {
        tag := "mail-" + n.MessageID
        if len(tag) > 128 {
            return tag[:128]
        }
        return tag
    }
    return "mail-received"
}

func endpointSuffix(endpoint string) string {
    if len(endpoint) <= 16 {
        return endpoint
    }
    return "..." + endpoint[len(endpoint)-16:]
}
```

- [ ] **Step 6: WebPushSubReader를 maildb.Repository에 연결**

`internal/maildb/web_push_subscriptions.go`에 `pushnotify.WebPushSubReader` 인터페이스를 만족하는 어댑터 메서드 추가:

```go
// WebPushSubData returns the data needed by pushnotify.WebPushSubReader.
// This method satisfies the pushnotify.WebPushSubReader interface.
func (r *Repository) WebPushSubData(ctx context.Context, userID string) ([]struct{ ID, Endpoint, P256DH, Auth string }, error) {
    subs, err := r.ListActiveWebPushSubscriptions(ctx, userID)
    // ... convert
}
```

대신, `pushnotify.WebPushSubData`를 반환하는 래퍼를 `run.go`에서 구성합니다 (Task 4에서 처리).

- [ ] **Step 7: 테스트 통과 확인**

```bash
go test ./internal/pushnotify/... -run TestWebPushSink -v
```

Expected: PASS

- [ ] **Step 8: 전체 테스트 확인**

```bash
go test ./...
```

Expected: ok

- [ ] **Step 9: 커밋**

```bash
git add internal/pushnotify/handler.go internal/pushnotify/webpush_sink.go internal/pushnotify/webpush_sink_test.go go.mod go.sum
git commit -m "feat: add WebPushSink with aes128gcm encryption via webpush-go"
```

---

## Task 4: Config 검증 + Push Worker 연결

**Goal:** `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush` 설정 시 VAPID 유효성 검사와 WebPushSink 인스턴스화가 이루어진다.

**Files:**
- Modify: `internal/config/validate.go`
- Modify: `internal/app/run.go`

**Acceptance Criteria:**
- [ ] `validate.go`의 `validateEnum`에 "webpush"가 추가됨
- [ ] backend=webpush일 때 `WebPushVAPIDPublicKey`, `WebPushVAPIDPrivateKey`, `WebPushContactEmail` 비어있으면 에러
- [ ] `pushNotificationSinkForConfig()`에 "webpush" case 추가됨
- [ ] `WebPushSink`에 DB 어댑터가 연결됨
- [ ] `go test ./internal/config/...` 통과
- [ ] `go test ./...` 통과

**Verify:** `go test ./internal/config/... -v -run TestValidate` → PASS

**Steps:**

- [ ] **Step 1: maildb → pushnotify 어댑터 타입 정의**

`internal/app/run.go`에 로컬 어댑터를 추가합니다. 이 파일 맨 아래 (import 안에서 이미 maildb가 사용되므로 추가 import 불필요):

```go
// webPushSubReaderAdapter adapts *maildb.Repository to pushnotify.WebPushSubReader.
type webPushSubReaderAdapter struct {
    repo *maildb.Repository
}

func (a *webPushSubReaderAdapter) ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]pushnotify.WebPushSubData, error) {
    subs, err := a.repo.ListActiveWebPushSubscriptions(ctx, userID)
    if err != nil {
        return nil, err
    }
    out := make([]pushnotify.WebPushSubData, len(subs))
    for i, s := range subs {
        out[i] = pushnotify.WebPushSubData{
            ID:       s.ID,
            Endpoint: s.Endpoint,
            P256DH:   s.P256DH,
            Auth:     s.Auth,
        }
    }
    return out, nil
}

func (a *webPushSubReaderAdapter) SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
    return a.repo.SoftDeleteWebPushSubscriptionByEndpoint(ctx, endpoint)
}
```

- [ ] **Step 2: validate.go 수정**

현재 코드 (line ~417):
```go
if err := validateEnum("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", c.PushNotifyBackend, "none", "slog", "webhook"); err != nil {
```

변경 후:
```go
if err := validateEnum("GOGOMAIL_PUSH_NOTIFICATION_BACKEND", c.PushNotifyBackend, "none", "slog", "webhook", "webpush"); err != nil {
    return err
}
if strings.EqualFold(strings.TrimSpace(c.PushNotifyBackend), "webpush") {
    if strings.TrimSpace(c.WebPushVAPIDPublicKey) == "" {
        return fmt.Errorf("GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY is required when GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush")
    }
    if strings.TrimSpace(c.WebPushVAPIDPrivateKey) == "" {
        return fmt.Errorf("GOGOMAIL_WEBPUSH_VAPID_PRIVATE_KEY is required when GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush")
    }
    if strings.TrimSpace(c.WebPushContactEmail) == "" {
        return fmt.Errorf("GOGOMAIL_WEBPUSH_CONTACT_EMAIL is required when GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush")
    }
}
```

- [ ] **Step 3: pushNotificationSinkForConfig() 수정**

`internal/app/run.go:2817`의 `pushNotificationSinkForConfig()` 함수에 webpush case 추가:

```go
func pushNotificationSinkForConfig(cfg config.Config, logger *slog.Logger, repo *maildb.Repository) (pushnotify.Sink, error) {
    switch strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend)) {
    case "slog":
        return pushnotify.SlogSink{Logger: logger}, nil
    case "webhook":
        return pushnotify.NewWebhookSink(pushnotify.WebhookOptions{
            Endpoint: strings.TrimSpace(cfg.PushNotifyWebhookURL),
            Token:    cfg.PushNotifyWebhookToken,
            Client:   webhookguard.GuardedHTTPClient(&http.Client{Timeout: cfg.PushNotifyWebhookTimeout}, webhookguard.OutboundURLGuardOptions{}),
        })
    case "webpush":
        return pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
            VAPIDPublicKey:  cfg.WebPushVAPIDPublicKey,
            VAPIDPrivateKey: cfg.WebPushVAPIDPrivateKey,
            ContactEmail:    cfg.WebPushContactEmail,
            DB:              &webPushSubReaderAdapter{repo: repo},
            Logger:          logger,
        })
    default:
        return nil, errors.New("unsupported push notification backend")
    }
}
```

- [ ] **Step 4: runPushNotificationWorker에서 repo 전달**

`runPushNotificationWorker()`에서 `pushNotificationSinkForConfig(cfg, logger)`를 `pushNotificationSinkForConfig(cfg, logger, repository)`로 변경. 단, `repository`는 이미 아래에서 초기화되므로 순서를 조정:

```go
func runPushNotificationWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
    backend := strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend))
    if backend == "" || backend == "none" {
        return waitForShutdown(ctx, logger, ModePushWorker)
    }

    db, err := openDatabase(ctx, cfg)
    if err != nil {
        return err
    }
    defer db.Close()

    repository := maildb.NewRepository(db)

    sink, err := pushNotificationSinkForConfig(cfg, logger, repository)
    if err != nil {
        return err
    }
    // ... 나머지 기존 코드 유지
```

- [ ] **Step 5: 테스트 통과 확인**

```bash
go test ./internal/config/... -v -run TestValidate
go test ./...
```

Expected: PASS

- [ ] **Step 6: 커밋**

```bash
git add internal/config/validate.go internal/app/run.go
git commit -m "feat: wire WebPushSink into push-notification-worker"
```

---

## Task 5: Backend API — push-subscriptions + VAPID config

**Goal:** 프론트엔드가 Web Push 구독을 등록/조회/삭제하고 VAPID 공개키를 가져올 수 있는 API 엔드포인트를 추가한다.

**Files:**
- Modify: `internal/httpapi/mail.go`

**Acceptance Criteria:**
- [ ] `GET /api/v1/config/web-push` — 인증 없이 `{ "vapidPublicKey": "..." }` 반환 (키 미설정 시 `null`)
- [ ] `POST /api/v1/me/push-subscriptions` — JWT 인증, 구독 upsert
- [ ] `GET /api/v1/me/push-subscriptions` — JWT 인증, 구독 목록
- [ ] `DELETE /api/v1/me/push-subscriptions/{id}` — JWT 인증, 구독 삭제
- [ ] 잘못된 요청에 400 반환
- [ ] `go test ./internal/httpapi/...` 통과

**Verify:** `go test ./internal/httpapi/... -v` → PASS

**Steps:**

- [ ] **Step 1: MailService 인터페이스에 메서드 추가**

`internal/httpapi/mail.go` 파일 상단 `MailService` 인터페이스 (약 76~80라인)에 추가:

```go
UpsertWebPushSubscription(ctx context.Context, req maildb.UpsertWebPushSubscriptionRequest) (maildb.WebPushSubscription, error)
ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]maildb.WebPushSubscription, error)
DeleteWebPushSubscription(ctx context.Context, userID, id string) error
```

- [ ] **Step 2: GET /api/v1/config/web-push 추가**

`mux.HandleFunc(...)` 블록에 추가 (기존 push-devices 근처):

```go
mux.HandleFunc("GET /api/v1/config/web-push", func(w http.ResponseWriter, r *http.Request) {
    key := strings.TrimSpace(webPushVAPIDPublicKey)
    var keyVal interface{}
    if key != "" {
        keyVal = key
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "vapidPublicKey": keyVal,
    })
})
```

`webPushVAPIDPublicKey`는 `RegisterMailRoutes` 함수 파라미터에 추가:

```go
func RegisterMailRoutes(mux *http.ServeMux, service MailService, tokenManager TokenManager, webPushVAPIDPublicKey string) {
```

- [ ] **Step 3: POST /api/v1/me/push-subscriptions 추가**

```go
mux.HandleFunc("POST /api/v1/me/push-subscriptions", func(w http.ResponseWriter, r *http.Request) {
    userID, ok := requireJWTUserID(w, r, tokenManager)
    if !ok {
        return
    }
    var body struct {
        Endpoint  string `json:"endpoint"`
        P256DH    string `json:"p256dh"`
        Auth      string `json:"auth"`
        UserAgent string `json:"userAgent"`
    }
    if err := decodeJSONBody(r, &body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    req := maildb.UpsertWebPushSubscriptionRequest{
        UserID:    userID,
        Endpoint:  body.Endpoint,
        P256DH:    body.P256DH,
        Auth:      body.Auth,
        UserAgent: body.UserAgent,
    }
    sub, err := service.UpsertWebPushSubscription(r.Context(), req)
    if err != nil {
        var ve *maildb.ValidationError
        if errors.As(err, &ve) {
            writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
            return
        }
        writeError(w, http.StatusInternalServerError, "internal_error", "failed to save subscription")
        return
    }
    writeJSON(w, http.StatusOK, sub)
})
```

- [ ] **Step 4: GET /api/v1/me/push-subscriptions 추가**

```go
mux.HandleFunc("GET /api/v1/me/push-subscriptions", func(w http.ResponseWriter, r *http.Request) {
    userID, ok := requireJWTUserID(w, r, tokenManager)
    if !ok {
        return
    }
    subs, err := service.ListActiveWebPushSubscriptions(r.Context(), userID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal_error", "failed to list subscriptions")
        return
    }
    if subs == nil {
        subs = []maildb.WebPushSubscription{}
    }
    writeJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs})
})
```

- [ ] **Step 5: DELETE /api/v1/me/push-subscriptions/{id} 추가**

```go
mux.HandleFunc("DELETE /api/v1/me/push-subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
    userID, ok := requireJWTUserID(w, r, tokenManager)
    if !ok {
        return
    }
    id := r.PathValue("id")
    if err := service.DeleteWebPushSubscription(r.Context(), userID, id); err != nil {
        if strings.Contains(err.Error(), "not found") {
            writeError(w, http.StatusNotFound, "not_found", "subscription not found")
            return
        }
        writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete subscription")
        return
    }
    w.WriteHeader(http.StatusNoContent)
})
```

- [ ] **Step 6: run.go에서 RegisterMailRoutes 호출 시 vapidPublicKey 전달**

`internal/app/run.go`에서 `RegisterMailRoutes` 호출 부분을 찾아 `cfg.WebPushVAPIDPublicKey` 추가:

```bash
grep -n "RegisterMailRoutes" internal/app/run.go
```

해당 라인을 수정하여 `cfg.WebPushVAPIDPublicKey` 전달.

- [ ] **Step 7: 테스트 통과 확인**

```bash
go test ./internal/httpapi/... -v
go test ./...
```

Expected: PASS

- [ ] **Step 8: 커밋**

```bash
git add internal/httpapi/mail.go internal/app/run.go
git commit -m "feat: add web push subscription API endpoints"
```

---

## Task 6: Frontend — useWebPush 훅 + 레이아웃 연결

**Goal:** 웹메일 앱 초기화 시 Service Worker를 등록하고 Web Push 구독을 서버에 저장하는 훅을 구현한다.

**Files:**
- Create: `apps/webmail/src/hooks/useWebPush.ts`
- Modify: `apps/webmail/src/components/Providers.tsx`

**Acceptance Criteria:**
- [ ] `useWebPush` 훅이 `GET /api/v1/config/web-push`로 VAPID 공개키를 가져온다
- [ ] `vapidPublicKey`가 null이면 아무 작업도 하지 않는다
- [ ] 브라우저 알림 권한이 `granted`일 때 SW 등록 + 구독 시도
- [ ] 구독 성공 시 `POST /api/v1/me/push-subscriptions` 호출
- [ ] `pushsubscriptionchange` 이벤트 시 재등록
- [ ] `Providers.tsx`에서 훅 호출
- [ ] `pnpm -C apps/webmail type-check` 통과

**Verify:** `pnpm -C apps/webmail type-check` → 에러 없음

**Steps:**

- [ ] **Step 1: 실패 타입 체크 확인**

```bash
pnpm -C apps/webmail type-check
```

Expected: PASS (아직 수정 없으므로)

- [ ] **Step 2: useWebPush 훅 작성**

```typescript
// apps/webmail/src/hooks/useWebPush.ts
'use client';

import { useEffect, useRef } from 'react';
import { webPushPublicKeyToUint8Array, arrayBufferToBase64URL } from '@/lib/webpush';

async function fetchVAPIDPublicKey(): Promise<string | null> {
  try {
    const res = await fetch('/api/v1/config/web-push');
    if (!res.ok) return null;
    const data = await res.json() as { vapidPublicKey?: string | null };
    return data.vapidPublicKey ?? null;
  } catch {
    return null;
  }
}

async function saveSubscription(sub: PushSubscription): Promise<void> {
  const json = sub.toJSON();
  const keys = json.keys as { p256dh?: string; auth?: string } | undefined;
  if (!json.endpoint || !keys?.p256dh || !keys?.auth) return;
  await fetch('/api/v1/me/push-subscriptions', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      endpoint: json.endpoint,
      p256dh: keys.p256dh,
      auth: keys.auth,
      userAgent: navigator.userAgent.slice(0, 256),
    }),
  });
}

async function registerWebPush(vapidPublicKey: string): Promise<void> {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) return;
  if (Notification.permission !== 'granted') return;

  const registration = await navigator.serviceWorker.register('/sw.js');
  await navigator.serviceWorker.ready;

  const applicationServerKey = webPushPublicKeyToUint8Array(vapidPublicKey);
  let sub = await registration.pushManager.getSubscription();
  if (!sub) {
    sub = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey,
    });
  }
  await saveSubscription(sub);
}

export function useWebPush(): void {
  const registered = useRef(false);

  useEffect(() => {
    if (registered.current) return;
    registered.current = true;

    void (async () => {
      const vapidPublicKey = await fetchVAPIDPublicKey();
      if (!vapidPublicKey) return;

      await registerWebPush(vapidPublicKey);

      navigator.serviceWorker?.addEventListener('message', (event: MessageEvent) => {
        if ((event.data as { type?: string } | null)?.type === 'pushsubscriptionchange') {
          void registerWebPush(vapidPublicKey);
        }
      });
    })();
  }, []);
}
```

- [ ] **Step 3: Providers.tsx에 훅 추가**

`apps/webmail/src/components/Providers.tsx` 내용을 읽고 훅을 호출하는 컴포넌트 추가:

```typescript
// Providers.tsx에서 useWebPush import 후 내부 컴포넌트에 추가:
import { useWebPush } from '@/hooks/useWebPush';

function WebPushInitializer() {
  useWebPush();
  return null;
}

// Providers의 children 반환 부분에 추가:
// <WebPushInitializer />
// {children}
```

- [ ] **Step 4: 타입 체크 통과 확인**

```bash
pnpm -C apps/webmail type-check
```

Expected: 에러 없음

- [ ] **Step 5: 커밋**

```bash
git add apps/webmail/src/hooks/useWebPush.ts apps/webmail/src/components/Providers.tsx
git commit -m "feat: add useWebPush hook and register SW subscription"
```

---

## Task 7: Frontend — Settings UI + i18n

**Goal:** 알림 설정 화면에 "백그라운드 푸시 알림" 토글을 추가하고, 한국어/영어 i18n 키를 추가한다.

**Files:**
- Modify: `apps/webmail/src/components/settings-view/SettingsNotificationsSection.tsx`
- Modify: `apps/webmail/messages/en.json`
- Modify: `apps/webmail/messages/ko.json`

**Acceptance Criteria:**
- [ ] `en.json`과 `ko.json`에 `misc.settingsNotif.pushLabel`, `pushDesc`, `pushUnsupported` 키 추가
- [ ] 브라우저가 Web Push를 지원하지 않으면 토글 대신 "지원하지 않음" 메시지 표시
- [ ] 브라우저 알림 권한이 `denied`이면 토글 비활성화
- [ ] 토글 상태가 localStorage의 `webmail_webpush_enabled`에 저장됨
- [ ] `pnpm -C apps/webmail type-check` 통과

**Verify:** `pnpm -C apps/webmail type-check` → 에러 없음

**Steps:**

- [ ] **Step 1: i18n 키 추가**

`apps/webmail/messages/en.json`에서 `settingsNotif` 블록을 찾아 다음 키 추가:

```json
"pushLabel": "Background push notifications",
"pushDesc": "Receive new mail alerts even when the tab is closed. Requires browser notifications to be allowed.",
"pushUnsupported": "Not supported in this browser"
```

`apps/webmail/messages/ko.json`에서 `settingsNotif` 블록에 추가:

```json
"pushLabel": "백그라운드 푸시 알림",
"pushDesc": "탭이 닫혀있을 때도 새 메일 알림을 받습니다. 브라우저 알림 허용이 먼저 켜져 있어야 합니다.",
"pushUnsupported": "이 브라우저에서는 지원하지 않습니다"
```

- [ ] **Step 2: SettingsNotificationsSection에 props + UI 추가**

`SettingsNotificationsSection.tsx`의 props 인터페이스에 추가:

```typescript
webPushEnabled: boolean;
setWebPushEnabled: (value: boolean) => void;
webPushSupported: boolean;
```

컴포넌트 내부 (dndEnd Row 아래 적절한 위치)에 Row 추가:

```tsx
<Row
  label={t('misc.settingsNotif.pushLabel')}
  description={
    !webPushSupported
      ? t('misc.settingsNotif.pushUnsupported')
      : notifPerm === 'denied'
      ? t('misc.settingsNotif.desktopDeniedDesc')
      : t('misc.settingsNotif.pushDesc')
  }
>
  {webPushSupported
    ? <Toggle
        value={webPushEnabled}
        onChange={setWebPushEnabled}
        ariaLabel={t('misc.settingsNotif.pushLabel')}
        disabled={notifPerm === 'denied'}
      />
    : <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
        {t('misc.settingsNotif.pushUnsupported')}
      </span>
  }
</Row>
```

- [ ] **Step 3: SettingsView에서 새 props 연결**

`apps/webmail/src/components/SettingsView.tsx` (또는 settings-view를 호출하는 상위 컴포넌트)에서:

```typescript
const [webPushEnabled, setWebPushEnabled] = useLocalStorage('webmail_webpush_enabled', false);
const webPushSupported = typeof window !== 'undefined'
  && 'serviceWorker' in navigator
  && 'PushManager' in window;
```

props에 `webPushEnabled`, `setWebPushEnabled`, `webPushSupported` 전달.

- [ ] **Step 4: 타입 체크 통과 확인**

```bash
pnpm -C apps/webmail type-check
```

Expected: 에러 없음

- [ ] **Step 5: 커밋**

```bash
git add apps/webmail/src/components/settings-view/SettingsNotificationsSection.tsx \
  apps/webmail/messages/en.json apps/webmail/messages/ko.json
git commit -m "feat: add background web push toggle to notification settings"
```

---

## Task 8: 발송 실패/반송 푸시 알림 이벤트 라우팅

**Goal:** `mail.delivery_exhausted` 이벤트를 소비해 메일 발송자에게 반송 알림 푸시를 전송하는 핸들러를 추가한다.

**Files:**
- Create: `internal/pushnotify/delivery_handler.go`
- Create: `internal/pushnotify/delivery_handler_test.go`
- Modify: `internal/app/run.go`

**Acceptance Criteria:**
- [ ] `DeliveryExhaustedHandler`가 `mail.delivery_exhausted` 이벤트를 파싱한다
- [ ] 이벤트의 `message_id`로 sender의 `user_id`를 DB에서 조회한다
- [ ] WebPushSink에 알림을 enqueue한다
- [ ] 알림 제목: "발송 실패", 본문: recipient 주소
- [ ] `go test ./internal/pushnotify/... -run TestDeliveryHandler -v` 통과
- [ ] `go test ./...` 통과

**Verify:** `go test ./internal/pushnotify/... -run TestDeliveryHandler -v` → PASS

**Steps:**

- [ ] **Step 1: 실패 테스트 작성**

```go
// internal/pushnotify/delivery_handler_test.go
package pushnotify_test

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/gogomail/gogomail/internal/eventstream"
    "github.com/gogomail/gogomail/internal/pushnotify"
)

type fakeSink struct {
    notifications []pushnotify.Notification
}

func (f *fakeSink) EnqueuePush(_ context.Context, n pushnotify.Notification) error {
    f.notifications = append(f.notifications, n)
    return nil
}

type fakeMessageUserLookup struct {
    userID string
}

func (f *fakeMessageUserLookup) GetMessageSenderUserID(_ context.Context, messageID string) (string, error) {
    return f.userID, nil
}

func TestDeliveryExhaustedHandler_HandleEvent(t *testing.T) {
    sink := &fakeSink{}
    lookup := &fakeMessageUserLookup{userID: "user-123"}
    handler := pushnotify.NewDeliveryExhaustedHandler(sink, lookup)

    payload, _ := json.Marshal(map[string]interface{}{
        "event":      "mail.delivery_exhausted",
        "message_id": "msg-abc",
        "company_id": "co-1",
        "domain_id":  "d-1",
        "sender":     "user@example.com",
        "recipients": []string{"bob@external.com"},
    })
    if err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload}); err != nil {
        t.Fatalf("HandleEvent error: %v", err)
    }
    if len(sink.notifications) != 1 {
        t.Fatalf("expected 1 notification, got %d", len(sink.notifications))
    }
    n := sink.notifications[0]
    if n.UserID != "user-123" {
        t.Errorf("expected UserID=user-123, got %q", n.UserID)
    }
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
go test ./internal/pushnotify/... -run TestDeliveryExhaustedHandler -v
```

Expected: FAIL

- [ ] **Step 3: 구현**

```go
// internal/pushnotify/delivery_handler.go
package pushnotify

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/gogomail/gogomail/internal/eventstream"
)

const EventMailDeliveryExhausted = "mail.delivery_exhausted"

// MessageUserLookup looks up the sender's user_id from a message_id.
type MessageUserLookup interface {
    GetMessageSenderUserID(ctx context.Context, messageID string) (string, error)
}

// DeliveryExhaustedHandler handles mail.delivery_exhausted events and sends push notifications.
type DeliveryExhaustedHandler struct {
    sink   Sink
    lookup MessageUserLookup
}

// NewDeliveryExhaustedHandler creates a handler for delivery exhausted events.
func NewDeliveryExhaustedHandler(sink Sink, lookup MessageUserLookup) *DeliveryExhaustedHandler {
    return &DeliveryExhaustedHandler{sink: sink, lookup: lookup}
}

type deliveryExhaustedEvent struct {
    Event      string   `json:"event"`
    MessageID  string   `json:"message_id"`
    CompanyID  string   `json:"company_id"`
    DomainID   string   `json:"domain_id"`
    Sender     string   `json:"sender"`
    Recipients []string `json:"recipients"`
}

// HandleEvent implements eventstream.Handler for mail.delivery_exhausted.
func (h *DeliveryExhaustedHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
    var ev deliveryExhaustedEvent
    if err := json.Unmarshal(msg.Payload, &ev); err != nil {
        return fmt.Errorf("delivery exhausted: decode: %w", err)
    }
    ev.MessageID = strings.TrimSpace(ev.MessageID)
    if ev.MessageID == "" {
        return fmt.Errorf("delivery exhausted: message_id is required")
    }

    userID, err := h.lookup.GetMessageSenderUserID(ctx, ev.MessageID)
    if err != nil {
        return fmt.Errorf("delivery exhausted: lookup sender user_id: %w", err)
    }
    if userID == "" {
        return nil
    }

    recipient := ""
    if len(ev.Recipients) > 0 {
        recipient = strings.Join(ev.Recipients, ", ")
        if len(recipient) > 100 {
            recipient = recipient[:100] + "…"
        }
    }

    return h.sink.EnqueuePush(ctx, Notification{
        MessageID: ev.MessageID,
        CompanyID: strings.TrimSpace(ev.CompanyID),
        DomainID:  strings.TrimSpace(ev.DomainID),
        UserID:    userID,
        Subject:   "발송 최종 실패",
        Recipient: recipient,
    })
}
```

- [ ] **Step 4: maildb에 GetMessageSenderUserID 추가**

`internal/maildb/` 에 해당 메서드를 추가 (적절한 파일에):

```go
// GetMessageSenderUserID returns the user_id who sent the message identified by messageID.
func (r *Repository) GetMessageSenderUserID(ctx context.Context, messageID string) (string, error) {
    messageID = strings.TrimSpace(messageID)
    if messageID == "" {
        return "", fmt.Errorf("message_id is required")
    }
    var userID string
    err := r.db.QueryRowContext(ctx, `
        SELECT user_id FROM messages WHERE id = $1 LIMIT 1
    `, messageID).Scan(&userID)
    if errors.Is(err, sql.ErrNoRows) {
        return "", nil
    }
    if err != nil {
        return "", fmt.Errorf("get message sender user_id: %w", err)
    }
    return userID, nil
}
```

- [ ] **Step 5: run.go에 이벤트 등록**

`runPushNotificationWorker()`의 `router.Register` 블록에 추가:

```go
if err := router.Register(pushnotify.EventMailDeliveryExhausted,
    pushnotify.NewDeliveryExhaustedHandler(sink, repository),
); err != nil {
    return err
}
```

- [ ] **Step 6: 테스트 통과 확인**

```bash
go test ./internal/pushnotify/... -run TestDeliveryExhaustedHandler -v
go test ./...
```

Expected: PASS

- [ ] **Step 7: 커밋**

```bash
git add internal/pushnotify/delivery_handler.go internal/pushnotify/delivery_handler_test.go \
  internal/maildb/ internal/app/run.go
git commit -m "feat: add delivery exhausted push notification handler"
```

---

## Task 9: 최종 검증

**Goal:** 전체 구현이 완성됐음을 확인한다.

**Files:** 없음 (검증만)

**Acceptance Criteria:**
- [ ] `go test ./...` 전체 통과
- [ ] `pnpm -C apps/webmail type-check` 통과
- [ ] `go build ./...` 성공
- [ ] `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush` 없이 validate 통과
- [ ] `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush` + VAPID 키 없이 validate 실패 (에러 메시지 정확)

**Verify:**
```bash
go test ./... && go build ./... && pnpm -C apps/webmail type-check
```
→ 모두 성공

**Steps:**

- [ ] **Step 1: 전체 Go 테스트**

```bash
go test ./...
```

Expected: `ok github.com/gogomail/gogomail/...` (실패 없음)

- [ ] **Step 2: Go 빌드**

```bash
go build ./...
```

Expected: 성공 (출력 없음)

- [ ] **Step 3: 프론트엔드 타입 체크**

```bash
pnpm -C apps/webmail type-check
```

Expected: 에러 없음

- [ ] **Step 4: Config validation 확인**

```bash
GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush \
GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_GROUP=g \
GOGOMAIL_PUSH_NOTIFICATION_CONSUMER_NAME=n \
go test ./internal/config/... -run TestValidate -v 2>&1 | grep -i "vapid\|webpush\|PASS\|FAIL"
```

Expected: VAPID 관련 에러 메시지 확인됨

- [ ] **Step 5: 최종 커밋 (필요 시)**

```bash
git add -A && git commit -m "chore: final web push integration verification"
```
