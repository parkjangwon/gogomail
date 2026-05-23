# Web Push Notification — Design Spec

**Date:** 2026-05-23  
**Scope:** Web Push (RFC 8030/8291) end-to-end — no FCM, no APNs  
**Goal:** 탭이 닫혀있어도 새 메일·발송 실패·반송 알림을 브라우저에 전달한다

---

## 1. 아키텍처 개요

```
[이벤트 발생]
    │
    ▼
push-notification-worker
  └─ Redis Stream 구독
       ├─ mail.stored            → 새 메일 수신
       ├─ mail.delivery_failed   → 발송 실패
       └─ mail.delivery_exhausted → 반송
            │
            ▼
       WebPushAdapter
         ├─ DB에서 user의 web_push_subscriptions 조회
         ├─ webpush-go 라이브러리로 RFC 8291 aes128gcm 암호화
         └─ POST → 브라우저 푸시 서비스 (Google/Mozilla/Apple)
                        │
                        ▼
                  Service Worker (sw.js) — 기존 구현 재사용
                        │
                        ▼
             registration.showNotification()
```

캘린더·드라이브 이벤트는 백엔드 이벤트 인프라가 없어 이번 범위 밖.

---

## 2. 데이터 레이어

### 2-1. 새 마이그레이션: `0152_web_push_subscriptions.sql`

```sql
CREATE TABLE web_push_subscriptions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   TEXT NOT NULL,
  p256dh     TEXT NOT NULL,
  auth       TEXT NOT NULL,
  user_agent TEXT,
  status     TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX web_push_subscriptions_endpoint_active_idx
  ON web_push_subscriptions(endpoint) WHERE status = 'active';

CREATE INDEX web_push_subscriptions_user_active_idx
  ON web_push_subscriptions(user_id) WHERE status = 'active';
```

- `endpoint`: 브라우저 푸시 서비스 URL (구독 고유 식별자)
- `p256dh`: base64url 인코딩된 브라우저 공개키
- `auth`: base64url 인코딩된 인증 시크릿
- `status`: `active` | `deleted` (invalid endpoint 410 수신 시 soft-delete)

### 2-2. VAPID 환경변수 (신규 Config)

```
GOGOMAIL_WEBPUSH_VAPID_PRIVATE_KEY=<base64url P-256 private key>
GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY=<base64url P-256 public key>
GOGOMAIL_WEBPUSH_VAPID_SUBJECT=mailto:admin@yourdomain.com
```

- 키 생성: `webpush-go`의 `GenerateVAPIDKeys()` 또는 `openssl ecparam`
- 두 키 모두 설정되지 않으면 push-notification-worker 시작 시 validation error

---

## 3. API

### 3-1. 공개 엔드포인트 (인증 불필요)

```
GET /api/v1/config/web-push
```

응답:
```json
{ "vapidPublicKey": "<base64url public key>" }
```

VAPID 키가 설정되지 않은 경우 `{ "vapidPublicKey": null }` → 프론트엔드가 Web Push 비활성화.

### 3-2. 구독 관리 (JWT 인증 필요)

```
POST   /api/v1/me/push-subscriptions
GET    /api/v1/me/push-subscriptions
DELETE /api/v1/me/push-subscriptions/{id}
```

POST 요청 body:
```json
{
  "endpoint": "https://...",
  "p256dh":   "BNcR...",
  "auth":     "tBH...",
  "userAgent": "Mozilla/5.0 ..."  // optional
}
```

- POST는 endpoint 기준 upsert (같은 endpoint이면 p256dh/auth 갱신)
- Rate limit: POST 30 req/min per user (기존 패턴 따름)

---

## 4. 백엔드 구현

### 4-1. `internal/maildb/web_push_subscriptions.go`

```go
UpsertWebPushSubscription(ctx, req)  // endpoint 기준 INSERT ON CONFLICT UPDATE
ListActiveWebPushSubscriptions(ctx, userID) []WebPushSubscription
DeleteWebPushSubscription(ctx, userID, id)
SoftDeleteByEndpoint(ctx, endpoint)  // 410 Gone 처리용
```

### 4-2. `internal/pushnotify/webpush.go` — 암호화 완성

기존 TODO를 `github.com/SherClockHolmes/webpush-go`로 대체:

```go
func (a *WebPushAdapter) Send(ctx, sub Subscription, payload []byte) error {
    resp, err := webpush.SendNotification(payload, &webpush.Subscription{
        Endpoint: sub.Endpoint,
        Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
    }, &webpush.Options{
        VAPIDPublicKey:  a.vapidPublicKey,
        VAPIDPrivateKey: a.vapidPrivateKey,
        Subscriber:      a.subject,
        TTL:             86400,
    })
    if resp.StatusCode == 410 {
        return ErrInvalidSubscription  // caller가 soft-delete 처리
    }
    ...
}
```

### 4-3. Push Worker 연결 (`internal/app/run.go`)

`GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webpush` 설정 시:
- VAPID 설정 로드 및 validation
- WebPushAdapter 인스턴스화
- Worker가 수신자 user_id → `web_push_subscriptions` 조회 → 각 구독에 전송

### 4-4. 이벤트 → 알림 매핑

| Redis 이벤트 | 알림 제목 | 알림 본문 | tag |
|---|---|---|---|
| `mail.stored` | 발신자 이름 (없으면 주소) | 메일 제목 | `mail-received` |
| `mail.delivery_failed` | 발송 실패 | 수신자 주소 | `mail-send-failed` |
| `mail.delivery_exhausted` | 발송 최종 실패 (반송) | 수신자 주소 | `mail-bounced` |

알림 페이로드 형식 (sw.js가 기대하는 기존 형식):
```json
{
  "title": "John Doe",
  "body":  "프로젝트 업데이트 관련 드립니다",
  "tag":   "mail-received",
  "url":   "/mail"
}
```

---

## 5. 프론트엔드

### 5-1. 새 훅: `apps/webmail/src/hooks/useWebPush.ts`

webmail 레이아웃 초기화 시 실행:
1. `GET /api/v1/config/web-push` → VAPID 공개키 조회 (null이면 중단)
2. `navigator.serviceWorker.register('/sw.js')`
3. 브라우저 알림 권한 `granted` 확인
4. `pushManager.subscribe({ userVisibleOnly: true, applicationServerKey })` 실행
5. 구독 객체 → `POST /api/v1/me/push-subscriptions` 저장
6. `pushsubscriptionchange` 이벤트 감지 시 재등록

### 5-2. 설정 UI

기존 webmail 알림 설정 페이지에 토글 추가:

```
알림 설정
├─ 브라우저 알림 허용  [토글 - 기존]
└─ 백그라운드 푸시 알림  [토글 - 신규]
     탭이 닫혀있을 때도 새 메일 알림을 받습니다.
     브라우저 알림 허용이 먼저 켜져 있어야 합니다.
```

### 5-3. `sw.js`

변경 없음 — 기존 `push` 이벤트 핸들러와 `notificationclick` 핸들러 재사용.

---

## 6. 에러 처리

| 상황 | 처리 |
|---|---|
| 410 Gone (구독 만료) | `SoftDeleteByEndpoint()` 자동 정리 |
| 429 Too Many Requests | 일시 실패로 처리, worker retry |
| VAPID 키 미설정 | worker 시작 실패, `/config/web-push` → null 반환 |
| 브라우저 알림 권한 denied | 프론트엔드에서 구독 시도 안 함 |

---

## 7. 테스트 전략

- **백엔드 유닛**: WebPushAdapter — 암호화 출력이 유효한 aes128gcm 형식인지 검증
- **백엔드 통합**: `web_push_subscriptions` CRUD, upsert 중복 처리
- **프론트엔드 E2E**: 기존 `notifications.spec.ts`에 Web Push 구독 등록/해제 시나리오 추가 (SW mock 활용)

---

## 8. 범위 밖 (Deferred)

- FCM / APNs
- 캘린더·드라이브 이벤트 알림
- 푸시 알림 per-folder DND 적용 (서버사이드) — 현재 클라이언트만 관리
- Admin Console 구독 모니터링 UI
