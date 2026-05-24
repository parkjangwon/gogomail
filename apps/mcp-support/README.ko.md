# GoGoMail Support MCP 서버

English / 영어: [README.md](README.md)

AI 에이전트에게 GoGoMail Admin API, Suppo 헬프데스크(선택), GitHub Issues(선택)에 대한 구조화된 직접 접근을 제공하는 [MCP(Model Context Protocol)](https://modelcontextprotocol.io/) 서버입니다. **무인 24/7 메일 서비스 운영**을 목표로 설계됐습니다 — 에이전트가 배송 오류 진단·수정, 사용자 계정 관리, 메일 큐 확인, 지원 티켓 처리를 사람 없이 수행할 수 있습니다.

---

## 목차

- [아키텍처](#아키텍처)
- [사전 요구 사항](#사전-요구-사항)
- [설치](#설치)
- [설정](#설정)
- [서버 실행](#서버-실행)
  - [Claude Desktop (stdio)](#claude-desktop-stdio)
  - [자율 에이전트 (HTTP + SSE)](#자율-에이전트-http--sse)
- [툴 레퍼런스](#툴-레퍼런스)
  - [GoGoMail Admin (37개)](#gogomail-admin-37개)
  - [Suppo 헬프데스크 (10개)](#suppo-헬프데스크-10개)
  - [GitHub Issues (6개)](#github-issues-6개)
- [워크플로우 예시](#워크플로우-예시)
- [감사 추적](#감사-추적)
- [보안 고려 사항](#보안-고려-사항)

---

## 아키텍처

```
자연어 요청 (사람 또는 티켓 시스템)
          │
          ▼
    AI 에이전트 (Claude, GPT-4, …)
          │  MCP 프로토콜 (JSON-RPC 2.0)
          ▼
  ┌─────────────────────────────┐
  │   apps/mcp-support          │
  │   (Node.js / TypeScript)    │
  │                             │
  │  ┌──────────────────────┐   │
  │  │  GoGoMail Admin      │   │──► GET/PATCH/POST/DELETE /admin/v1/…
  │  │  37개 툴  [필수]      │   │    Bearer: GOGOMAIL_ADMIN_KEY
  │  └──────────────────────┘   │
  │  ┌──────────────────────┐   │
  │  │  Suppo 헬프데스크     │   │──► /api/public/…
  │  │  10개 툴  [선택]      │   │    Bearer: SUPPO_API_KEY
  │  └──────────────────────┘   │
  │  ┌──────────────────────┐   │
  │  │  GitHub Issues       │   │──► api.github.com
  │  │   6개 툴  [선택]      │   │    Token: GITHUB_TOKEN
  │  └──────────────────────┘   │
  └─────────────────────────────┘
```

**전송 모드:**
- **stdio** — Claude Desktop 및 로컬 CLI용. MCP 호스트(Claude Desktop)가 서버를 하위 프로세스로 실행하고 stdin/stdout으로 통신합니다.
- **HTTP + SSE** — 원격 자율 에이전트용. 서버가 HTTP 서비스로 실행되고, 에이전트가 Server-Sent Events로 연결해 POST로 명령을 전송합니다.

---

## 사전 요구 사항

- Node.js 20 이상
- Admin API에 접근 가능한 실행 중인 GoGoMail 인스턴스

---

## 설치

```bash
cd apps/mcp-support
npm install
npm run build        # TypeScript → dist/ 컴파일
```

컴파일된 진입점은 `dist/index.js`입니다.

---

## 설정

모든 설정은 환경 변수로 합니다.

| 변수 | 필수 여부 | 설명 |
|---|---|---|
| `GOGOMAIL_ADMIN_URL` | **필수** | GoGoMail 서버 기본 URL, 예: `https://mail.example.com` |
| `GOGOMAIL_ADMIN_KEY` | **필수** | Admin API Bearer 토큰 |
| `SUPPO_API_URL` | 선택 | Suppo 헬프데스크 기본 URL, 예: `https://support.example.com` |
| `SUPPO_API_KEY` | 선택 | Suppo 공개 API 키 (`crn_live_…`). `kb:write tickets:read tickets:create tickets:update` 스코프 필요 |
| `GITHUB_TOKEN` | 선택 | `repo` 스코프를 가진 GitHub Personal Access Token |
| `GITHUB_REPO` | 선택 | Issues에 사용할 `owner/repo` (기본값: `parkjangwon/gogomail`) |
| `MCP_TRANSPORT` | 선택 | `stdio` (기본값) 또는 `sse` |
| `MCP_HOST` | 선택 | SSE 전송 바인드 주소 (기본값: `127.0.0.1`). 리버스 프록시나 사설 네트워크 리스너가 필요할 때만 명시적으로 설정하세요. |
| `MCP_PORT` | 선택 | SSE 전송 시 HTTP 포트 (기본값: `3100`) |
| `MCP_SECRET` | `MCP_TRANSPORT=sse`일 때 필수 | 모든 SSE 연결에 `Authorization: Bearer <값>` 헤더가 필요합니다. 강력한 랜덤 시크릿(32자 이상)을 사용하세요. |
| `MCP_ALLOWED_ORIGINS` | 선택 | SSE 엔드포인트 호출을 허용할 브라우저 Origin의 콤마 구분 목록. `Origin` 헤더가 있는 요청은 목록에 없으면 거부됩니다. |
| `MCP_ALLOW_INSECURE_UPSTREAMS` | 선택 | 기본값 `false`. 명시적으로 `true`일 때만 loopback이 아닌 `http://` upstream URL을 허용합니다. |

### 최소 설정 (GoGoMail만)

두 개의 변수만 필요합니다. Suppo나 GitHub 없이도 서버가 시작되며, 해당 툴을 호출하면 크래시 대신 명확한 "설정되지 않음" 오류가 반환됩니다.

```bash
export GOGOMAIL_ADMIN_URL=https://mail.example.com
export GOGOMAIL_ADMIN_KEY=your-admin-token
node dist/index.js
```

### 풀스택 설정

```bash
export GOGOMAIL_ADMIN_URL=https://mail.example.com
export GOGOMAIL_ADMIN_KEY=your-admin-token
export SUPPO_API_URL=https://support.example.com
export SUPPO_API_KEY=crn_live_...
export GITHUB_TOKEN=ghp_...
export GITHUB_REPO=parkjangwon/gogomail
node dist/index.js
```

---

## 서버 실행

### Claude Desktop (stdio)

Claude Desktop이 MCP 서버를 하위 프로세스로 실행합니다. `~/Library/Application Support/Claude/claude_desktop_config.json`을 수정하세요:

**GoGoMail만:**

```json
{
  "mcpServers": {
    "gogomail-support": {
      "command": "node",
      "args": ["/절대경로/apps/mcp-support/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://mail.example.com",
        "GOGOMAIL_ADMIN_KEY": "your-admin-token"
      }
    }
  }
}
```

**풀스택 (GoGoMail + Suppo + GitHub):**

```json
{
  "mcpServers": {
    "gogomail-support": {
      "command": "node",
      "args": ["/절대경로/apps/mcp-support/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://mail.example.com",
        "GOGOMAIL_ADMIN_KEY": "your-admin-token",
        "SUPPO_API_URL": "https://support.example.com",
        "SUPPO_API_KEY": "crn_live_...",
        "GITHUB_TOKEN": "ghp_...",
        "GITHUB_REPO": "parkjangwon/gogomail"
      }
    }
  }
}
```

저장 후 Claude Desktop을 재시작하면 채팅 UI에 망치 아이콘(🔨)이 나타나며 MCP 툴이 로드된 것을 확인할 수 있습니다.

### 자율 에이전트 (HTTP + SSE)

원격 에이전트가 연결할 수 있도록 SSE 모드로 서버를 시작합니다:

```bash
MCP_SECRET=your-strong-random-secret \
MCP_TRANSPORT=sse \
MCP_PORT=3100 \
GOGOMAIL_ADMIN_URL=https://mail.example.com \
GOGOMAIL_ADMIN_KEY=your-admin-token \
node dist/index.js
```

서버가 노출하는 엔드포인트:
- `GET  http://localhost:3100/sse` — SSE 스트림. 에이전트가 먼저 여기에 연결합니다.
- `POST http://localhost:3100/messages?sessionId=<id>` — 에이전트가 툴 호출을 여기로 전송합니다.

> **보안:** SSE 모드에서는 `MCP_SECRET`이 필수이며, 없으면 서버가 시작되지 않습니다.
> 모든 요청에 `Authorization: Bearer <MCP_SECRET>` 헤더가 포함되어야 합니다.
> SSE 서버는 기본적으로 `127.0.0.1`에 바인딩됩니다. 사설 인터페이스나 리버스 프록시 뒤로 노출할 때만 `MCP_HOST`를 명시적으로 설정하세요.

---

## 툴 레퍼런스

툴 이름은 `{공급자}_{동작}_{대상}` 패턴을 따릅니다. GoGoMail **액션** 툴(쓰기 작업)은 사람이 읽을 수 있는 `reason`을 필수로 받고 모두 감사 로그가 기록됩니다 — `ticketId`가 지정되면 해당 Suppo 티켓에 내부 댓글로 기록되고, 생략하면 자동으로 별도 감사 티켓이 생성됩니다. 되돌릴 수 없는 삭제는 정확한 `confirm` 문구도 필요합니다. Suppo가 설정되지 않은 경우 감사 기록은 stderr에 출력됩니다.

### GoGoMail Admin (37개)

#### 사용자 및 디렉터리

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_search_principals` | `GET /admin/v1/directory/principals?q=` | 이메일이나 이름으로 사용자·그룹·별칭 검색. 이메일 주소만 아는 경우 **여기서 시작**하세요. |
| `gogomail_list_users` | `GET /admin/v1/users` | 사용자 목록. `domainId` 및/또는 `status`로 필터링 가능. |
| `gogomail_get_user` | `GET /admin/v1/users/{id}` | 사용자 전체 정보 조회: 상태, 역할, 할당량, 도메인. |
| `gogomail_get_user_quota` | `GET /admin/v1/users/{id}/quota` | 할당량 한도 및 현재 사용량(바이트) 조회. |

#### 회사 및 도메인

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_companies` | `GET /admin/v1/companies` | 전체 테넌트 회사 목록. |
| `gogomail_get_company` | `GET /admin/v1/companies/{id}` | ID로 회사 상세 정보 조회. |
| `gogomail_list_domains` | `GET /admin/v1/domains` | 도메인 목록. `companyId`, `status`, `dnsStatus`로 필터링. |
| `gogomail_get_domain_settings` | `GET /admin/v1/domains/{id}/settings` | 도메인 설정 조회: TLS 정책, 사용자별 쿼터, IP 허용 목록, 2FA, 세션 타임아웃, 비밀번호 정책, 초대/재설정 정책. |
| `gogomail_check_domain_dns` | `GET /admin/v1/domains/{id}/dns-check` | DNS 레코드 검증 상태 확인 (SPF, DKIM, DMARC, MX). DNS 오설정으로 인한 메일 오류 진단에 활용. |

#### 메일 흐름 진단

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_mail_flow_logs` | `GET /admin/v1/mail-flow-logs` | 메일 흐름 로그 검색. `userId`, `companyId`, `domainId`, `messageId`, `fromAddr`, `toAddr`, `direction`(`inbound`/`outbound`), `flowStatus`(`delivered`/`bounced`/`deferred`/`rejected`/`quarantined`/`expired`), `since`, `until`, `limit`으로 필터링. |
| `gogomail_get_mail_flow_stats` | `GET /admin/v1/mail-flow-logs/stats` | 특정 기간의 상태별 배송 건수 집계. 대규모 오류 급증 감지에 유용. |

#### 배송 실패

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_delivery_attempts` | `GET /admin/v1/delivery-attempts` | 홉별 오류 상세가 포함된 배송 시도 목록. `messageId`, `status`, `recipientDomain`, `sender`, `since`로 필터링. |
| `gogomail_list_exhausted_deliveries` | `GET /admin/v1/delivery-attempts/exhausted` | 자동 재시도를 모두 소진해 수동 처리가 필요한 메시지 목록. |

#### Dead Letter Queue (DLQ)

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_dlq` | `GET /admin/v1/dlq?stream=` | DLQ 스트림의 항목 목록. `stream`은 필수. |
| `gogomail_delete_dlq_entry` | `DELETE /admin/v1/dlq/{id}?stream=` | 막힌 DLQ 항목 삭제. `reason`과 `confirm="delete <stream>/<id>"` 필요. *(감사 로그 기록)* |

#### 아웃박스 복구

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_retry_outbox` | `POST /admin/v1/outbox/{id}/retry` | ID로 막힌 아웃박스 메시지 수동 재시도. *(감사 로그 기록)* |

#### 수신 거부 목록

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_suppression_list` | `GET /admin/v1/suppression-list` | 수신 거부된 주소 검색. `email`, `domainId`, `reason`(`bounce`/`complaint`/`manual`)으로 필터링. |
| `gogomail_remove_suppression_entry` | `DELETE /admin/v1/suppression-list/{id}` | 수신 거부 목록에서 주소 제거해 메일 수신 재개. *(감사 로그 기록)* |

#### 할당량 관리

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_quota_usage` | `GET /admin/v1/quota-usage` | 할당량 사용량 목록. `overLimit=true`로 한도 초과 사용자만 필터링. |
| `gogomail_list_quota_alerts` | `GET /admin/v1/quota-alerts` | 발동된 할당량 임계값 알럿 목록. |

#### 사용자 액션 (모두 감사 로그 기록)

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_send_invite_email` | `POST /admin/v1/users/{id}/invite` | 비밀번호 설정 초대 이메일 발송. 비밀번호 재설정 요청에 사용. |
| `gogomail_update_user_status` | `PATCH /admin/v1/users/{id}/status` | 계정 상태 변경: `active`, `suspended`, `disabled`. |
| `gogomail_update_user_quota` | `PATCH /admin/v1/users/{id}/quota` | 저장 할당량(바이트) 변경. |
| `gogomail_update_user_role` | `PATCH /admin/v1/users/{id}/role` | 역할 변경: `user`, `company_admin`, `system_admin`. |
| `gogomail_update_user_recovery_email` | `PATCH /admin/v1/users/{id}/recovery-email` | 복구 이메일 주소 변경. |
| `gogomail_create_user` | `POST /admin/v1/users` | 도메인에 새 사용자 계정 생성. |
| `gogomail_delete_user` | `DELETE /admin/v1/users/{id}` | 사용자 영구 삭제. `reason`과 `confirm="delete <userId>"` 필요. **되돌릴 수 없음.** |
| `gogomail_update_domain_settings` | `PUT /admin/v1/domains/{id}/settings` | 도메인 설정 변경: TLS 정책, 사용자별 쿼터, IP 허용 목록, 2FA, 세션 타임아웃, 비밀번호 정책, 초대/재설정 정책. 생략 필드는 현재 설정과 병합해 보존합니다. |

#### 세션 관리

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_list_company_sessions` | `GET /admin/v1/companies/{id}/sessions` | 회사의 모든 활성 로그인 세션 목록. |
| `gogomail_revoke_company_session` | `DELETE /admin/v1/companies/{id}/sessions/{userId}` | 특정 사용자 강제 로그아웃. *(감사 로그 기록)* |

#### 보안 및 모니터링

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_get_spam_filter` | `GET /admin/v1/companies/{id}/security/spam-filter` | 회사의 스팸 필터 정책 조회. |
| `gogomail_get_spam_filter_events` | `GET /admin/v1/companies/{id}/security/spam-filter/events` | 최근 스팸 필터 이벤트 조회. 정상 메일 차단(오탐) 조사에 활용. |
| `gogomail_list_dkim_keys` | `GET /admin/v1/dkim-keys` | DKIM 서명 키 목록. `domainId`를 지정해 특정 도메인만 확인 가능. |
| `gogomail_get_alert_events` | `GET /admin/v1/companies/{id}/alert-events` | 회사의 시스템 알럿 이벤트 조회. |
| `gogomail_get_audit_logs` | `GET /admin/v1/audit-logs` | Admin 감사 로그 조회. `userId`, `companyId`, `from`, `to`로 필터링. |

#### 시스템 상태

| 툴 | 메서드 + 경로 | 설명 |
|---|---|---|
| `gogomail_check_health` | `GET /admin/v1/health` | 시스템 상태 및 컴포넌트 가용성 확인. |
| `gogomail_get_queue_stats` | `GET /admin/v1/queue` | 메일 큐 깊이 및 처리 통계 조회. 높은 큐 깊이는 백로그나 배송 병목을 나타냄. |

---

### Suppo 헬프데스크 (10개)

`SUPPO_API_URL`과 `SUPPO_API_KEY`가 필요합니다. 설정되지 않으면 "설정되지 않음" 오류를 반환합니다.

| 툴 | 설명 |
|---|---|
| `suppo_list_tickets` | 티켓 목록. `status`(`open`/`pending`/`closed`/`resolved`) 및/또는 `priority`(`low`/`normal`/`high`/`urgent`)로 필터링. |
| `suppo_get_ticket` | 전체 댓글 기록 포함 티켓 상세 조회. |
| `suppo_search_tickets` | `customerEmail` 또는 키워드(`query`)로 티켓 검색. |
| `suppo_create_ticket` | 새 지원 티켓 생성. 필드: `customerName`, `customerEmail`, `subject`, `description`, `priority`. |
| `suppo_update_ticket` | 티켓 `status` 및/또는 `priority` 변경. |
| `suppo_add_comment` | 고객에게 보이는 답변 또는 내부 메모(`internal: true`) 추가. |
| `suppo_assign_ticket` | `assigneeId`로 티켓 담당자 지정. |
| `suppo_list_agents` | 가능한 지원 에이전트 목록(id, name, email). `assigneeId` 검색에 활용. |
| `suppo_search_kb` | 공개된 KB 문서 전문 검색. |
| `suppo_create_kb_article` | 새 KB 문서 생성. 반복되는 문제 해결 후 활용. |

---

### GitHub Issues (6개)

`GITHUB_TOKEN`이 필요합니다. 설정되지 않으면 "설정되지 않음" 오류를 반환합니다.

| 툴 | 설명 |
|---|---|
| `github_search_issues` | 설정된 저장소의 이슈 전문 검색. 사용자가 입력한 `repo:`, `org:`, `user:` qualifier는 무시되어 `GITHUB_REPO` 범위를 벗어나지 않습니다. |
| `github_get_issue` | 이슈 번호로 상세 정보 및 댓글 스레드 조회. |
| `github_list_issues` | 이슈 목록. `state`(`open`/`closed`/`all`) 및/또는 `labels`로 필터링. |
| `github_create_issue` | 버그 리포트 또는 기능 요청 생성. `title`, `body`, 선택적 `labels` 포함. |
| `github_add_comment` | 기존 이슈에 댓글 추가. |
| `github_update_issue` | 이슈 `state`(`open`/`closed`) 및/또는 `labels` 변경. |

---

## 워크플로우 예시

AI 에이전트가 따르는 대표적인 패턴들입니다. 에이전트는 툴을 순서대로 호출하며, 결과를 읽어 다음 단계를 결정합니다.

### 시나리오 1 — 메일 발송이 안 된다는 고객 문의

```
사용자: "alice@example.com이 어제부터 메일을 못 보낸다고 합니다."

에이전트:
  1. gogomail_search_principals(q: "alice@example.com")
     → userId: "usr_abc123", domainId: "dom_xyz" 발견

  2. gogomail_get_user(userId: "usr_abc123")
     → status: "active", role: "user" — 계정 정상

  3. gogomail_list_mail_flow_logs(
       userId: "usr_abc123",
       direction: "outbound",
       flowStatus: "rejected",
       since: "2026-05-23T00:00:00Z"
     )
     → 12건: "550 5.7.1 Message rejected due to SPF failure"

  4. gogomail_check_domain_dns(domainId: "dom_xyz")
     → SPF: FAIL — "include:mail.example.com"이 TXT 레코드에 없음

  에이전트 응답: "Alice의 발신 메일이 SPF 실패로 거부되고 있습니다.
  example.com의 SPF 레코드에 GoGoMail 발송 서버가 포함되어 있지 않습니다.
  DNS 공급자에서 다음 TXT 레코드를 추가해야 합니다:
  v=spf1 include:mail.example.com ~all"
```

### 시나리오 2 — 비밀번호를 잊어버린 사용자

```
사용자: "bob@example.com이 비밀번호를 잊어버렸습니다."

에이전트:
  1. gogomail_search_principals(q: "bob@example.com")
     → userId: "usr_def456"

  2. gogomail_get_user(userId: "usr_def456")
     → status: "active" — 초대 이메일 발송 가능

  3. gogomail_send_invite_email(
       userId: "usr_def456",
       reason: "활성 계정 확인 후 고객이 비밀번호 재설정을 요청함",
       ticketId: "tkt-789"
     )
     → 초대 토큰 생성, 이메일 발송
     → tkt-789 티켓에 감사 메모 자동 기록

  에이전트 응답: "bob@example.com으로 비밀번호 재설정 이메일을 발송했습니다.
  링크는 24시간 유효합니다."
```

### 시나리오 3 — 메일 배송 실패가 급증하는 상황

```
사용자: "바운스가 많이 발생하고 있는데, 무슨 일인가요?"

에이전트:
  1. gogomail_get_queue_stats()
     → queues: { outbound: { depth: 4821, processing: 12 } }
     — 큐 깊이 비정상적으로 높음

  2. gogomail_get_mail_flow_stats(
       direction: "outbound",
       since: "2026-05-24T00:00:00Z"
     )
     → delivered: 1,203 / deferred: 892 / bounced: 441

  3. gogomail_list_exhausted_deliveries(since: "2026-05-24T00:00:00Z")
     → 38건 소진, 모두 @hotmail.com 수신자

  4. gogomail_list_delivery_attempts(
       recipientDomain: "hotmail.com",
       status: "failed",
       since: "2026-05-24T00:00:00Z"
     )
     → 오류: "421 4.7.0 Too many connections from your IP"

  에이전트 응답: "Hotmail에서 연결 수 초과(421)로 속도 제한을 걸고 있습니다.
  38건이 재시도 한도를 초과했습니다. 발신 IP 평판을 확인하고
  쿨다운 후 재시도하는 것을 권장합니다.
  아웃박스 메시지들을 재시도할까요?"
```

### 시나리오 4 — 계정 무단 접근 의심

```
사용자: "charlie@example.com 계정이 무단으로 접근된 것 같습니다."

에이전트:
  1. gogomail_search_principals(q: "charlie@example.com")
     → userId: "usr_ghi789", companyId: "co_123"

  2. gogomail_list_company_sessions(companyId: "co_123")
     → charlie의 세션 3개: IP 203.0.113.5 (불상 국가), 2분 전 최종 활동

  3. gogomail_revoke_company_session(
       companyId: "co_123",
       userId: "usr_ghi789",
       reason: "계정 탈취 의심 및 알 수 없는 활성 세션 확인",
       ticketId: "tkt-900"
     )
     → 세션 강제 종료, tkt-900에 감사 기록

  4. gogomail_get_audit_logs(userId: "usr_ghi789", from: "2026-05-20T00:00:00Z")
     → 14건의 로그인, 그 중 9건이 외부 IP에서

  5. gogomail_send_invite_email(
       userId: "usr_ghi789",
       reason: "계정 탈취 의심 후 자격 증명 재설정",
       ticketId: "tkt-900"
     )
     → 비밀번호 재설정 이메일 발송

  에이전트 응답: "Charlie의 세션을 모두 강제 종료하고 비밀번호 재설정 이메일을
  발송했습니다. 최근 4일간 203.0.113.5에서 9회 로그인 기록이 있습니다.
  모든 조치 내역은 tkt-900에 기록됐습니다. 이메일 내용 유출 여부도
  확인해 보시기 바랍니다."
```

---

## 감사 추적

GoGoMail **액션** 툴(쓰기 작업)은 모두 `reason` 입력을 필수로 받고 실행 후 자동으로 감사 메모를 기록합니다. 메모에는 다음이 포함됩니다:

- 툴 이름
- 대상 엔티티 (이메일, userId, domainId 등)
- 운영자/에이전트가 제공한 사유
- 적용 가능한 경우 변경 전 → 후 상태
- UTC 타임스탬프

**메모가 기록되는 위치:**

| 조건 | 동작 |
|---|---|
| `ticketId` 제공 + Suppo 설정됨 | 해당 Suppo 티켓에 내부 댓글로 추가 |
| `ticketId` 생략 + Suppo 설정됨 | 자동으로 별도 감사 티켓 생성 |
| Suppo 미설정 (모든 경우) | `stderr`에 구조화된 로그로 출력 |

감사 쓰기는 대기하지만 best-effort로 처리됩니다. 메모 기록에 실패해도 이미 완료된 **액션 자체를 롤백하거나 실패 처리하지 않습니다**. 대신 툴 결과에 `audit` 객체(`written` 또는 `failed`)가 포함되어 에이전트가 중복 변경 재시도 없이 증적 누락을 에스컬레이션할 수 있습니다.

되돌릴 수 없는 삭제 툴은 두 번째 확인 필드가 필요합니다:
- `gogomail_delete_user`: `confirm` 값이 정확히 `delete <userId>`여야 합니다.
- `gogomail_delete_dlq_entry`: `confirm` 값이 정확히 `delete <stream>/<id>`여야 합니다.

---

## 보안 고려 사항

**Admin 키 보호**

`GOGOMAIL_ADMIN_KEY`는 Admin API 전체 접근 권한을 부여합니다. root 비밀번호처럼 취급하세요:
- AWS Secrets Manager, Vault, 1Password 같은 시크릿 관리자에 저장하고, 소스 컨트롤에 커밋되는 평문 env 파일에는 넣지 마세요.
- 유출된 경우 즉시 교체하세요.

**SSE 엔드포인트 인증**

`MCP_SECRET`은 Bearer 토큰 인증을 활성화하며 `MCP_TRANSPORT=sse`일 때 필수입니다. 모든 SSE 연결(`GET /sse`)과 툴 호출 요청(`POST /messages`)에 다음 헤더가 필요합니다:
```
Authorization: Bearer <MCP_SECRET>
```
토큰 비교는 타이밍 공격을 방지하기 위해 `crypto.timingSafeEqual`을 사용합니다. `MCP_SECRET`이 없으면 SSE 모드는 시작 단계에서 종료됩니다.

프로덕션 권장 사항:
- 강력한 `MCP_SECRET` 설정 (32바이트 이상의 랜덤 값, base64 인코딩)
- 기본 `MCP_HOST=127.0.0.1`을 유지하거나, 리버스 프록시 뒤의 사설 인터페이스에만 바인딩
- 브라우저 기반 에이전트가 SSE 엔드포인트를 호출해야 할 때 `MCP_ALLOWED_ORIGINS` 설정
- upstream URL은 HTTPS 사용. loopback이 아닌 `http://` upstream은 `MCP_ALLOW_INSECURE_UPSTREAMS=true` 없이는 거부됩니다
- 포트를 외부 인터넷에 절대 노출하지 마세요

**최소 권한 원칙**

읽기 전용 진단만 필요한 경우 `SUPPO_API_KEY`와 `GITHUB_TOKEN`을 생략하고, 제한된 권한의 GoGoMail Admin 키를 사용할 수 있습니다.

**감사 로그 무결성**

Suppo 티켓에 기록되는 감사 추적은 이 툴들의 관점에서 추가 전용입니다 — 에이전트는 이 툴로 Suppo 댓글을 삭제할 수 없습니다. 포렌식 목적으로는 GoGoMail 자체의 불변 감사 로그를 읽는 `gogomail_get_audit_logs`와 교차 검증하세요.
