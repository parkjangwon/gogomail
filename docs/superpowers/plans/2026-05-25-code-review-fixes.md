# Code Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 코드 리뷰에서 발견된 7가지 문제를 수정한다 — DM 검색 성능, 한국어 하드코딩, ListMedia 타입 불일치, metrics interface{}, Grafana 기본 비밀번호, 임시 스크립트, MCP 설명 오류.

**Architecture:** Go DM 서비스 레이어의 기능 버그와 설계 결함을 먼저 수정하고, 인프라(docker) 보안 설정과 코드베이스 위생 문제를 이어서 고친다. 모든 Go 변경은 `go test ./...` 통과를 검증한다.

**Tech Stack:** Go, TypeScript, Docker Compose YAML

---

## File Map

| 파일 | 수정 내용 |
|------|-----------|
| `internal/dm/dm_store.go` | ListSearchCandidates 상한값 낮춤 |
| `internal/dm/dm.go` | 검색 주석 추가, SystemMessages 구조체 주입, ListMedia 타입 정규화 |
| `internal/caldavgw/handler.go` | metrics interface{} → 로컬 typed interface |
| `internal/carddavgw/handler.go` | metrics interface{} → 로컬 typed interface |
| `internal/imapgw/server.go` | metrics interface{} → 로컬 typed interface |
| `docker/docker-compose.monitoring.yml` | Grafana 기본 비밀번호 제거 |
| `docker/docker-compose.dev.yml` | Grafana 기본 비밀번호 제거 |
| `docker/docker-compose.large.yml` | Grafana 기본 비밀번호 제거 |
| `apps/console/` | 11개 임시 스크립트 삭제 |
| `apps/gogomail-user-mcp/src/tools.ts` | DM search tool description 수정 |

---

### Task 1: DM 검색 candidate limit 낮추기

**Goal:** `ListSearchCandidates`의 최대 10000 레코드 제한을 1000으로 낮추고, 암호화된 DM 검색이 왜 in-memory인지 설명하는 주석을 추가한다.

**Files:**
- Modify: `internal/dm/dm_store.go:725-726`
- Modify: `internal/dm/dm.go:463-500`

**Acceptance Criteria:**
- [ ] `ListSearchCandidates`의 상한이 1000
- [ ] `dm.go`의 `Search` 함수에 in-memory scan 이유를 설명하는 주석 존재
- [ ] `go test ./internal/dm/...` 통과

**Verify:** `go test ./internal/dm/... -v -run TestSearch` → PASS

**Steps:**

- [ ] **Step 1: dm_store.go의 상한값 수정**

`internal/dm/dm_store.go` 725-726행을 수정:

```go
// 변경 전
func (s *PostgresStore) ListSearchCandidates(ctx context.Context, principal Principal, roomID string, beforeMessageID string, limit int) ([]MessageRecord, error) {
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
```

```go
// 변경 후
func (s *PostgresStore) ListSearchCandidates(ctx context.Context, principal Principal, roomID string, beforeMessageID string, limit int) ([]MessageRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
```

- [ ] **Step 2: dm.go의 Search 함수에 주석 추가**

`internal/dm/dm.go`의 `Search` 함수 상단에 주석 추가 (현재 func 선언 바로 아래):

```go
func (s *Service) Search(ctx context.Context, principal Principal, roomID string, q string, before string, limit int) ([]SearchResult, error) {
	// NOTE: DM 메시지는 AES-GCM 암호화로 저장된다. DB 레벨 FTS가 불가능하므로
	// 메시지를 복호화한 뒤 애플리케이션 레이어에서 strings.Contains로 검색한다.
	// 최대 1000개 메시지를 스캔하므로 메시지 수가 많은 방에서는 오래된 메시지가
	// 검색 범위에서 벗어날 수 있다.
	principal = normalizePrincipal(principal)
```

- [ ] **Step 3: 테스트 실행**

```bash
cd /Users/pjw/dev/project/gogomail && go test ./internal/dm/... -v 2>&1 | tail -20
```

Expected: `ok  	gogomail/internal/dm`

- [ ] **Step 4: 커밋**

```bash
git add internal/dm/dm_store.go internal/dm/dm.go
git commit -m "fix(dm): lower search candidate limit 10000→1000, document in-memory scan"
```

---

### Task 2: DM 시스템 메시지 한국어 하드코딩 해결

**Goal:** 서비스 레이어에 하드코딩된 5개 한국어 문자열을 주입 가능한 `SystemMessages` 구조체로 추출하여 i18n-ready 구조를 만든다. 기본값은 현재와 동일한 한국어를 유지한다.

**Files:**
- Modify: `internal/dm/dm.go`

**Acceptance Criteria:**
- [ ] `SystemMessages` 구조체가 `dm.go`에 정의됨
- [ ] `DefaultSystemMessages()` 함수가 현재 한국어 문자열을 반환
- [ ] `Service` 구조체에 `messages SystemMessages` 필드 추가
- [ ] `NewService`는 `DefaultSystemMessages()`로 초기화
- [ ] `Service.WithSystemMessages(SystemMessages) *Service` 메서드로 재정의 가능
- [ ] 5개 하드코딩 한국어 문자열이 모두 `s.messages.*` 참조로 교체됨
- [ ] `go test ./internal/dm/...` 통과

**Verify:** `go test ./internal/dm/... -v` → PASS, `grep -n '"삭제된\|님이 초대\|님이 나갔\|방장이\|님이 참여' internal/dm/dm.go` → 결과 없음

**Steps:**

- [ ] **Step 1: SystemMessages 구조체 및 기본값 정의**

`internal/dm/dm.go`에서 `const` 블록 아래 (약 32행)에 추가:

```go
// SystemMessages holds the text templates for system-generated DM messages.
// All placeholders use %s for a user's display name.
// Override at startup with Service.WithSystemMessages to support other locales.
type SystemMessages struct {
	MessageDeleted   string // shown for soft-deleted messages (no placeholder)
	MemberInvited    string // %s = invitee display name
	MemberLeft       string // %s = leaving member display name
	OwnerTransferred string // %s = new owner display name
	MemberJoined     string // %s = joining member display name
}

// DefaultSystemMessages returns the built-in Korean system message templates.
func DefaultSystemMessages() SystemMessages {
	return SystemMessages{
		MessageDeleted:   "삭제된 메시지입니다.",
		MemberInvited:    "%s님이 초대되었습니다.",
		MemberLeft:       "%s님이 나갔습니다.",
		OwnerTransferred: "방장이 %s님에게 권한을 위임했습니다.",
		MemberJoined:     "%s님이 참여했습니다.",
	}
}
```

- [ ] **Step 2: Service 구조체에 messages 필드 추가**

현재:
```go
type Service struct {
	store       Store
	crypto      *Crypto
	attachments AttachmentStore
	now         func() time.Time
}
```

변경:
```go
type Service struct {
	store       Store
	crypto      *Crypto
	attachments AttachmentStore
	now         func() time.Time
	messages    SystemMessages
}
```

- [ ] **Step 3: NewService에서 기본값 초기화, WithSystemMessages 추가**

현재:
```go
func NewService(store Store, crypto *Crypto) *Service {
	return &Service{store: store, crypto: crypto, now: time.Now}
}
```

변경:
```go
func NewService(store Store, crypto *Crypto) *Service {
	return &Service{store: store, crypto: crypto, now: time.Now, messages: DefaultSystemMessages()}
}

// WithSystemMessages replaces the default (Korean) system message templates.
// Call before the service handles any requests.
func (s *Service) WithSystemMessages(msgs SystemMessages) *Service {
	s.messages = msgs
	return s
}
```

- [ ] **Step 4: 5개 하드코딩 문자열을 s.messages.* 참조로 교체**

`DeleteMessage` (약 439행):
```go
// 변경 전
deleted.Body = "삭제된 메시지입니다."
// 변경 후
deleted.Body = s.messages.MessageDeleted
```

`AddMembers` (약 591행):
```go
// 변경 전
systemMessages, err := s.memberSystemMessages(key, roomID, users, "%s님이 초대되었습니다.")
// 변경 후
systemMessages, err := s.memberSystemMessages(key, roomID, users, s.messages.MemberInvited)
```

`RemoveMember` (약 624행):
```go
// 변경 전
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf("%s님이 나갔습니다.", displayName(users[0])))
// 변경 후
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberLeft, displayName(users[0])))
```

`TransferOwner` (약 662행):
```go
// 변경 전
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf("방장이 %s님에게 권한을 위임했습니다.", displayName(users[0])))
// 변경 후
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.OwnerTransferred, displayName(users[0])))
```

`JoinInvite` (약 706행):
```go
// 변경 전
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf("%s님이 참여했습니다.", displayName(users[0])))
// 변경 후
systemMessage, err := s.systemMessage(key, roomID, fmt.Sprintf(s.messages.MemberJoined, displayName(users[0])))
```

- [ ] **Step 5: 테스트 실행**

```bash
cd /Users/pjw/dev/project/gogomail && go test ./internal/dm/... -v 2>&1 | tail -20
```

Expected: `ok  	gogomail/internal/dm`

- [ ] **Step 6: 하드코딩 잔재 확인**

```bash
grep -n '"삭제된\|님이 초대\|님이 나갔\|방장이\|님이 참여' /Users/pjw/dev/project/gogomail/internal/dm/dm.go
```

Expected: 출력 없음

- [ ] **Step 7: 커밋**

```bash
git add internal/dm/dm.go
git commit -m "feat(dm): extract system messages into injectable SystemMessages struct"
```

---

### Task 3: ListMedia 타입 정규화 수정

**Goal:** `Service.ListMedia`의 타입 switch가 MCP API 계약 타입(`"file"`, `"drive_link"`, `"link"`)을 Store 내부 타입(`"file"`, `"drive"`, `"links"`)으로 올바르게 정규화하도록 고친다.

**Files:**
- Modify: `internal/dm/dm.go:508-511`

**Acceptance Criteria:**
- [ ] MCP 타입 `"file"` → store에 `"file"` 전달
- [ ] MCP 타입 `"drive_link"` → store에 `"drive"` 전달 (listDriveMedia 호출)
- [ ] MCP 타입 `"link"` → store에 `"links"` 전달 (listLinkMedia 호출)
- [ ] 알 수 없는 타입 → `"file"` 기본값
- [ ] `go test ./internal/dm/...` 통과

**Verify:** `go test ./internal/dm/... -v` → PASS

**Steps:**

- [ ] **Step 1: 기존 switch 교체**

`internal/dm/dm.go`의 `ListMedia` 함수에서:

```go
// 변경 전
switch strings.ToLower(strings.TrimSpace(query.Type)) {
case "files", "file", "image", "video", "links", "drive":
default:
	query.Type = "files"
}
```

```go
// 변경 후
// Normalize API-level type names to the store's internal tokens.
// API uses: "file", "drive_link", "link"
// Store uses: "file" (→ listFileMedia), "drive" (→ listDriveMedia), "links" (→ listLinkMedia)
switch strings.ToLower(strings.TrimSpace(query.Type)) {
case "drive_link", "drive":
	query.Type = "drive"
case "link", "links":
	query.Type = "links"
default:
	query.Type = "file"
}
```

- [ ] **Step 2: 테스트 실행**

```bash
cd /Users/pjw/dev/project/gogomail && go test ./internal/dm/... -v 2>&1 | tail -20
```

Expected: `ok  	gogomail/internal/dm`

- [ ] **Step 3: 커밋**

```bash
git add internal/dm/dm.go
git commit -m "fix(dm): normalize ListMedia API types to store tokens (drive_link→drive, link→links)"
```

---

### Task 4: metrics interface{} → 타입 안전 로컬 인터페이스

**Goal:** caldavgw, carddavgw, imapgw의 `metrics interface{}`를 각 패키지에 최소 로컬 인터페이스로 교체하여 런타임 type assertion을 제거한다.

**Files:**
- Modify: `internal/caldavgw/handler.go`
- Modify: `internal/carddavgw/handler.go`
- Modify: `internal/imapgw/server.go`

**Acceptance Criteria:**
- [ ] 세 파일 모두 `metrics interface{}` 필드 없음
- [ ] `SetMetrics(interface{})` 시그니처 없음
- [ ] 런타임 type assertion `.(interface{ RecordCommand... })` 없음
- [ ] `go test ./internal/caldavgw/... ./internal/carddavgw/... ./internal/imapgw/...` 통과

**Verify:** `go test ./internal/caldavgw/... ./internal/carddavgw/... ./internal/imapgw/... -v 2>&1 | tail -10` → 모두 PASS

**Steps:**

- [ ] **Step 1: caldavgw/handler.go 수정**

`Handler` 구조체 정의 근처 (파일 상단)에 인터페이스 정의 추가:

```go
// gatewayMetrics is the minimal interface caldavgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
}
```

`Handler` 구조체의 `metrics` 필드 변경:
```go
// 변경 전
metrics           interface{} // GatewayMetrics (optional, typed as interface{} to avoid import)
// 변경 후
metrics           gatewayMetrics
```

`SetMetrics` 시그니처 변경:
```go
// 변경 전
func (h *Handler) SetMetrics(metrics interface{}) {
// 변경 후
func (h *Handler) SetMetrics(metrics gatewayMetrics) {
```

`recordCommand`, `recordError` 메서드를 단순화 (type assertion 제거):
```go
func (h *Handler) recordCommand(userID string, duration time.Duration) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordCommand(userID, duration)
}

func (h *Handler) recordError(userID string) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordError(userID)
}
```

- [ ] **Step 2: carddavgw/handler.go 수정**

caldavgw와 동일한 패턴 적용. `Handler` 구조체 근처에:

```go
// gatewayMetrics is the minimal interface carddavgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
}
```

필드 변경:
```go
metrics          gatewayMetrics
```

`SetMetrics`:
```go
func (h *Handler) SetMetrics(metrics gatewayMetrics) {
```

`recordCommand`, `recordError`:
```go
func (h *Handler) recordCommand(userID string, duration time.Duration) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordCommand(userID, duration)
}

func (h *Handler) recordError(userID string) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordError(userID)
}
```

- [ ] **Step 3: imapgw/server.go 수정**

imapgw는 connect/disconnect도 사용하므로 인터페이스가 더 넓다:

```go
// gatewayMetrics is the minimal interface imapgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordConnect(userID string)
	RecordDisconnect()
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
}
```

`Server` 구조체 필드:
```go
metrics     gatewayMetrics
```

`SetMetrics`:
```go
func (s *Server) SetMetrics(metrics gatewayMetrics) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()
}
```

각 record 함수에서 type assertion 제거 (imapgw는 mutex로 로컬 복사 후 사용하므로 로컬 변수를 통해):
```go
func (s *Server) recordConnect(userID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordConnect(userID)
}

func (s *Server) recordDisconnect() {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordDisconnect()
}

func (s *Server) recordCommand(userID string, duration time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordCommand(userID, duration)
}

func (s *Server) recordError(userID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordError(userID)
}
```

- [ ] **Step 4: 컴파일 확인**

```bash
cd /Users/pjw/dev/project/gogomail && go build ./internal/caldavgw/... ./internal/carddavgw/... ./internal/imapgw/...
```

Expected: 오류 없음

- [ ] **Step 5: 테스트 실행**

```bash
cd /Users/pjw/dev/project/gogomail && go test ./internal/caldavgw/... ./internal/carddavgw/... ./internal/imapgw/... -v 2>&1 | grep -E "^ok|FAIL|---"
```

Expected: 세 패키지 모두 `ok`

- [ ] **Step 6: interface{} 잔재 확인**

```bash
grep -n 'metrics.*interface{}' /Users/pjw/dev/project/gogomail/internal/caldavgw/handler.go \
  /Users/pjw/dev/project/gogomail/internal/carddavgw/handler.go \
  /Users/pjw/dev/project/gogomail/internal/imapgw/server.go
```

Expected: 출력 없음

- [ ] **Step 7: 커밋**

```bash
git add internal/caldavgw/handler.go internal/carddavgw/handler.go internal/imapgw/server.go
git commit -m "refactor: replace metrics interface{} with typed local interfaces in caldavgw/carddavgw/imapgw"
```

---

### Task 5: Grafana 기본 비밀번호 제거

**Goal:** docker-compose 파일 3곳에서 `:-admin` 기본값을 제거하여, `GRAFANA_PASSWORD` 환경 변수 미설정 시 Grafana가 기동되지 않도록 한다.

**Files:**
- Modify: `docker/docker-compose.monitoring.yml:85`
- Modify: `docker/docker-compose.dev.yml:312`
- Modify: `docker/docker-compose.large.yml:403`

**Acceptance Criteria:**
- [ ] 세 파일 모두 `:-admin` 패턴 없음
- [ ] `GRAFANA_PASSWORD` 변수의 용도를 설명하는 주석이 각 파일에 존재
- [ ] 파일 상단/near Grafana 섹션에 `.env` 또는 환경 변수 설정 방법 안내

**Verify:** `grep ':-admin' docker/docker-compose.monitoring.yml docker/docker-compose.dev.yml docker/docker-compose.large.yml` → 출력 없음

**Steps:**

- [ ] **Step 1: docker-compose.monitoring.yml 수정**

```yaml
# 변경 전
GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_PASSWORD:-admin}"

# 변경 후
# GRAFANA_PASSWORD must be set in environment or .env file. No default — fail loudly if missing.
GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_PASSWORD}"
```

파일 상단 주석도 업데이트:
```yaml
# 변경 전
#   Grafana UI:  http://localhost:3000  (admin / $GRAFANA_PASSWORD)
# 변경 후
#   Grafana UI:  http://localhost:3000  (admin / $GRAFANA_PASSWORD — set in .env or environment)
```

- [ ] **Step 2: docker-compose.dev.yml 수정**

```yaml
# 변경 전
GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_PASSWORD:-admin}"
# 변경 후
# GRAFANA_PASSWORD must be set. No default provided — fail loudly if missing.
GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_PASSWORD}"
```

- [ ] **Step 3: docker-compose.large.yml 수정**

```yaml
# 변경 전
GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD:-admin}
# 변경 후
# GRAFANA_PASSWORD must be set. No default provided — fail loudly if missing.
GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD}
```

- [ ] **Step 4: 검증**

```bash
grep ':-admin' /Users/pjw/dev/project/gogomail/docker/docker-compose.monitoring.yml \
  /Users/pjw/dev/project/gogomail/docker/docker-compose.dev.yml \
  /Users/pjw/dev/project/gogomail/docker/docker-compose.large.yml
```

Expected: 출력 없음

- [ ] **Step 5: 커밋**

```bash
git add docker/docker-compose.monitoring.yml docker/docker-compose.dev.yml docker/docker-compose.large.yml
git commit -m "security: remove Grafana default admin password from docker-compose files"
```

---

### Task 6: apps/console 임시 스크립트 정리

**Goal:** i18n 작업 중 생성된 11개 임시 Python/JS 스크립트를 `apps/console/` 루트에서 삭제한다. `postcss.config.js`는 실제 설정 파일이므로 유지한다.

**Files:**
- Delete: `apps/console/add_all_translations.js`
- Delete: `apps/console/auto_update_all_pages.py`
- Delete: `apps/console/batch_update_pages.js`
- Delete: `apps/console/complete_all_translations.py`
- Delete: `apps/console/comprehensive_i18n_update.py`
- Delete: `apps/console/final_translations.js`
- Delete: `apps/console/fix_unused_pattern.py`
- Delete: `apps/console/full_i18n_complete.py`
- Delete: `apps/console/suppress_unused_warnings.py`
- Delete: `apps/console/translate_strings.py`
- Delete: `apps/console/update_i18n_all_pages.js`

**Acceptance Criteria:**
- [ ] 11개 스크립트 삭제됨
- [ ] `postcss.config.js` 유지됨
- [ ] `apps/console/` 루트에 `*.py`, 번역/i18n 관련 `*.js` 없음

**Verify:** `ls apps/console/*.py apps/console/*translation*.js apps/console/*i18n*.js apps/console/*translate*.js 2>&1` → "No such file" 출력

**Steps:**

- [ ] **Step 1: 삭제 실행**

```bash
cd /Users/pjw/dev/project/gogomail && git rm \
  apps/console/add_all_translations.js \
  apps/console/auto_update_all_pages.py \
  apps/console/batch_update_pages.js \
  apps/console/complete_all_translations.py \
  apps/console/comprehensive_i18n_update.py \
  apps/console/final_translations.js \
  apps/console/fix_unused_pattern.py \
  apps/console/full_i18n_complete.py \
  apps/console/suppress_unused_warnings.py \
  apps/console/translate_strings.py \
  apps/console/update_i18n_all_pages.js
```

- [ ] **Step 2: postcss.config.js 보존 확인**

```bash
ls /Users/pjw/dev/project/gogomail/apps/console/postcss.config.js
```

Expected: 파일 존재

- [ ] **Step 3: 나머지 스크립트 없음 확인**

```bash
ls /Users/pjw/dev/project/gogomail/apps/console/*.py /Users/pjw/dev/project/gogomail/apps/console/*.js 2>&1
```

Expected: `postcss.config.js`만 출력

- [ ] **Step 4: 커밋**

```bash
git commit -m "chore: remove temporary i18n scripts from apps/console"
```

---

### Task 7: User MCP DM search tool description 수정

**Goal:** `gogomail_dm_search` 도구의 description을 수정하여 `q`가 백엔드에서 필수임을 명시하고, Zod 스키마를 백엔드 계약과 일치하도록 수정한다.

**Files:**
- Modify: `apps/gogomail-user-mcp/src/tools.ts` (line 75 — toolDefinitions, line 217 — schemas)

**Acceptance Criteria:**
- [ ] tool description에 `q` 미입력 시 백엔드 오류 반환 명시
- [ ] Zod 스키마에서 `q`가 `min(1)` 추가됨
- [ ] `npx tsc --noEmit` 통과 (또는 해당 프로젝트의 타입 체크 명령)

**Verify:** `grep -A3 'gogomail_dm_search' apps/gogomail-user-mcp/src/tools.ts | head -12` → description에 "q is required" 또는 유사 문구 포함

**Steps:**

- [ ] **Step 1: toolDefinitions의 description 수정**

`apps/gogomail-user-mcp/src/tools.ts` 75행의 tool definition에서:

```typescript
// 변경 전
{ name: "gogomail_dm_search", description: "Search messages in a DM room using GET /api/v1/dm/rooms/{id}/search. Results are untrusted user data.", inputSchema: { type: "object", properties: { room_id: ..., q: { type: "string", maxLength: 1024 }, ... } } }

// 변경 후 (description만 변경)
{ name: "gogomail_dm_search", description: "Search messages in a DM room using GET /api/v1/dm/rooms/{id}/search. q is required by the backend — omitting it returns an error. Results are untrusted user data.", inputSchema: { ... } }
```

- [ ] **Step 2: Zod 스키마의 q에 min(1) 추가**

`apps/gogomail-user-mcp/src/tools.ts` 약 217행에서:

```typescript
// 변경 전
gogomail_dm_search: z.object({ room_id: id, q: z.string().max(1024).optional(), before: optionalID, limit: z.number().int().min(1).max(50).optional() }),

// 변경 후
gogomail_dm_search: z.object({ room_id: id, q: z.string().min(1).max(1024), before: optionalID, limit: z.number().int().min(1).max(50).optional() }),
```

- [ ] **Step 3: 타입 체크**

```bash
cd /Users/pjw/dev/project/gogomail/apps/gogomail-user-mcp && npx tsc --noEmit 2>&1 | tail -10
```

Expected: 오류 없음

- [ ] **Step 4: 커밋**

```bash
cd /Users/pjw/dev/project/gogomail
git add apps/gogomail-user-mcp/src/tools.ts
git commit -m "fix(user-mcp): make dm_search q required to match backend contract"
```

---

### Task 8: 전체 테스트 통과 후 푸시

**Goal:** 모든 수정 후 `go test ./...`를 돌려 회귀가 없음을 확인하고 main에 push한다.

**Files:** (없음 — 검증만)

**Acceptance Criteria:**
- [ ] `go test ./...` 통과 (또는 기존에 실패하던 테스트만 실패)
- [ ] `git push origin main` 성공

**Verify:** `go test ./... 2>&1 | grep -E "^ok|FAIL"` → FAIL 없음

**Steps:**

- [ ] **Step 1: 전체 테스트**

```bash
cd /Users/pjw/dev/project/gogomail && go test ./... 2>&1 | grep -E "^ok|FAIL|---" | head -60
```

Expected: `FAIL` 없음

- [ ] **Step 2: push**

```bash
cd /Users/pjw/dev/project/gogomail && git push origin main
```

Expected: `main -> main` 성공 메시지
