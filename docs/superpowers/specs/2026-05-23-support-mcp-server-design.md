# GoGoMail Manage MCP Server — 설계 스펙

**날짜:** 2026-05-23  
**목표:** 기술 지원 업무(문의 처리·오류 추적·이슈 트래킹·기능 답변)를 AI 에이전트가 무인으로 수행할 수 있는 MCP 서버

---

## 1. 개요

### 배경

GoGoMail SaaS 운영 시 고객 기술 지원 업무를 AI 에이전트에게 위임한다. 에이전트는 MCP(Model Context Protocol)를 통해 세 가지 시스템에 접근한다.

- **Suppo** — 헬프데스크 티켓 시스템 (고객 문의 수신·답변·이력)
- **GoGoMail Admin API** — 사용자·메일·도메인 조사 및 조치
- **GitHub API** — 버그/기능 이슈 트래킹 (`parkjangwon/gogomail` 레포)

### 전형적인 지원 플로우

```
Suppo 티켓 수신
  → 고객 이메일로 GoGoMail 사용자 조회
  → 메일 로그·배달 오류·할당량 등 조사
  → 조치 실행 (비밀번호 초기화, 상태 변경, 할당량 조정 등)
  → 티켓에 고객 답변 작성 + 내부 메모(감사 기록)
  → 필요 시 GitHub 이슈 생성 및 연결
```

---

## 2. 아키텍처

```
AI 에이전트 (Claude)
       ↓  MCP protocol (stdio 또는 HTTP+SSE)
┌──────────────────────────────────────────┐
│   apps/gogomail-manage-mcp  (TypeScript/Node.js) │
│                                          │
│   tools/suppo.ts   (10개 도구)           │
│   tools/gogomail.ts (18개 도구)          │
│   tools/github.ts   (6개 도구)           │
│                                          │
│   clients/suppo.ts                       │
│   clients/gogomail.ts                    │
│   clients/github.ts  (@octokit/rest)     │
└────┬──────────────┬──────────────┬───────┘
     ↓              ↓              ↓
  Suppo API   GoGoMail Admin API  GitHub REST API
```

### 핵심 원칙

- **클라이언트와 도구 분리**: `clients/` 는 HTTP 통신만, `tools/` 는 MCP 도구 정의만 담당
- **stateless**: MCP 서버는 상태를 보유하지 않고 매 호출마다 외부 API 호출
- **단일 바이너리**: `node dist/index.js` 한 줄로 실행
- **전송 방식 전환**: `--transport=stdio`(기본) / `--transport=sse` 플래그로 전환

---

## 3. 파일 구조

```
apps/gogomail-manage-mcp/
├── src/
│   ├── index.ts                 # MCP 서버 진입점, 클라이언트 초기화, 도구 등록
│   ├── config.ts                # 환경변수 파싱 및 검증 (필수값 누락 시 즉시 종료)
│   ├── clients/
│   │   ├── suppo.ts             # Suppo Public API 클라이언트
│   │   ├── gogomail.ts          # GoGoMail Admin API 클라이언트
│   │   └── github.ts            # GitHub REST API 클라이언트 (@octokit/rest)
│   └── tools/
│       ├── suppo.ts             # Suppo 도구 10개
│       ├── gogomail.ts          # GoGoMail 도구 18개
│       └── github.ts            # GitHub 도구 6개
├── package.json
└── tsconfig.json
```

---

## 4. 도구 카탈로그

### 4.1 Suppo 도구 (10개)

| 도구 이름 | 설명 | 주요 파라미터 |
|---|---|---|
| `suppo_list_tickets` | 티켓 목록 조회 | `status`, `priority`, `limit` |
| `suppo_get_ticket` | 티켓 상세 + 댓글 이력 | `ticketId` |
| `suppo_search_tickets` | 고객 이메일·키워드로 검색 | `customerEmail`, `query` |
| `suppo_create_ticket` | 새 티켓 생성 | `customerName`, `customerEmail`, `subject`, `description`, `priority` |
| `suppo_update_ticket` | 상태·우선순위 변경 | `ticketId`, `status`, `priority` |
| `suppo_add_comment` | 고객 답변 또는 내부 메모 추가 | `ticketId`, `body`, `internal` (bool) |
| `suppo_assign_ticket` | 담당자 배정 | `ticketId`, `assigneeId` |
| `suppo_list_agents` | 배정 가능한 에이전트 목록 | — |
| `suppo_search_kb` | 지식베이스 검색 | `query` |
| `suppo_create_kb_article` | 해결된 케이스로 KB 문서 생성 | `title`, `content` |

### 4.2 GoGoMail 도구 (18개)

**조회 도구 (9개)**

| 도구 이름 | 설명 | 주요 파라미터 |
|---|---|---|
| `gogomail_find_user` | 이메일로 사용자 검색 | `email` |
| `gogomail_get_user` | 사용자 상세 (상태·역할·할당량) | `userId` |
| `gogomail_get_user_quota` | 스토리지 사용량 상세 | `userId` |
| `gogomail_get_mail_logs` | 메일 플로우 로그 | `userId`, `direction`, `status`, `from`, `to` (시간) |
| `gogomail_trace_message` | 특정 메시지 전송 경로 추적 | `messageId` |
| `gogomail_get_delivery_attempts` | 배달 시도 및 오류 상세 | `messageId` |
| `gogomail_get_audit_logs` | 사용자·회사 감사 로그 | `userId`, `companyId`, `from`, `to` |
| `gogomail_list_user_sessions` | 활성 세션 목록 | `userId` |
| `gogomail_check_health` | 시스템 상태 및 큐 현황 | — |

**조치 도구 (9개)**

| 도구 이름 | 설명 | 주요 파라미터 |
|---|---|---|
| `gogomail_reset_password` | 비밀번호 재설정 초대 메일 발송 | `userId` |
| `gogomail_update_user_status` | 계정 활성화·정지·비활성화 | `userId`, `status` |
| `gogomail_update_user_quota` | 스토리지 할당량 조정 | `userId`, `quotaBytes` |
| `gogomail_revoke_sessions` | 모든 세션 강제 종료 | `userId` |
| `gogomail_update_user_role` | 사용자 역할 변경 | `userId`, `role` |
| `gogomail_get_company` | 회사·도메인 정보 조회 | `companyId` |
| `gogomail_get_domain_settings` | 도메인 설정 조회 | `domainId` |
| `gogomail_update_domain_settings` | 도메인 설정 변경 | `domainId`, `settings` |
| `gogomail_get_alert_events` | 최근 경고 이벤트 조회 | `companyId`, `limit` |

### 4.3 GitHub 도구 (6개)

| 도구 이름 | 설명 | 주요 파라미터 |
|---|---|---|
| `github_search_issues` | 키워드·라벨로 이슈 검색 | `query`, `labels`, `state` |
| `github_get_issue` | 이슈 상세 + 댓글 조회 | `issueNumber` |
| `github_list_issues` | 라벨·마일스톤별 이슈 목록 | `labels`, `milestone`, `state` |
| `github_create_issue` | 새 버그·기능 이슈 생성 | `title`, `body`, `labels` |
| `github_add_comment` | 이슈에 댓글 추가 | `issueNumber`, `body` |
| `github_update_issue` | 라벨·상태 업데이트 | `issueNumber`, `labels`, `state` |

---

## 5. 안전장치 및 감사

### 조치 도구 자동 감사 기록

모든 조치 도구(`gogomail_update_*`, `gogomail_reset_*`, `gogomail_revoke_*`)는 실행 후 연관 Suppo 티켓에 내부 메모를 자동으로 남긴다.

```
[자동 실행] gogomail_update_user_status
- 대상: user@example.com (userId: abc-123)
- 변경: active → suspended
- 실행 시각: 2026-05-23T15:30:00Z
```

티켓 ID가 컨텍스트에 없을 때는 별도 감사 티켓을 생성한다.

### 도구 description 가이드라인

조치 도구의 MCP description에 선행 조건을 명시한다:

> "`gogomail_update_user_status`: 계정 상태를 변경한다. **반드시 `gogomail_get_user`로 현재 상태를 확인한 후 호출할 것.** status는 'active' | 'suspended' | 'disabled'."

---

## 6. Suppo 신규 API

현재 Suppo Public API에 없는 엔드포인트로, 이 MCP 서버를 위해 Suppo에 추가 개발한다.

| 메서드 | 경로 | 설명 | 필요 scope |
|---|---|---|---|
| `POST` | `/api/public/tickets/{id}/comments` | 댓글 추가 (`internal: bool` 구분) | `tickets:update` |
| `GET` | `/api/public/agents` | 에이전트 목록 | `tickets:read` |
| `GET` | `/api/public/kb/articles/search` | KB 검색 (`?q=`) | `tickets:read` |
| `POST` | `/api/public/kb/articles` | KB 문서 생성 | `kb:write` (신규 scope) |
| `GET` | `/api/public/tickets?customerEmail=` | 이미 지원 여부 확인 후 필요 시 추가 | `tickets:read` |

---

## 7. 설정 및 배포

### 환경변수

```env
# GoGoMail
GOGOMAIL_ADMIN_URL=https://admin.gogomail.io
GOGOMAIL_ADMIN_KEY=...

# Suppo
SUPPO_API_URL=https://support.gogomail.io
SUPPO_API_KEY=crn_live_...

# GitHub
GITHUB_TOKEN=ghp_...
GITHUB_REPO=parkjangwon/gogomail

# 전송 방식 (기본: stdio)
MCP_TRANSPORT=stdio  # 또는 sse
MCP_PORT=3100        # SSE 모드일 때
```

### 로컬 / Claude Desktop (stdio)

```json
{
  "mcpServers": {
    "gogomail-manage-mcp": {
      "command": "node",
      "args": ["/path/to/apps/gogomail-manage-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "...",
        "GOGOMAIL_ADMIN_KEY": "...",
        "SUPPO_API_URL": "...",
        "SUPPO_API_KEY": "...",
        "GITHUB_TOKEN": "..."
      }
    }
  }
}
```

### 원격 자율 에이전트 (HTTP+SSE)

```bash
MCP_TRANSPORT=sse MCP_PORT=3100 node dist/index.js
```

---

## 8. 의존성

| 패키지 | 용도 |
|---|---|
| `@modelcontextprotocol/sdk` | MCP 서버 프레임워크 |
| `@octokit/rest` | GitHub REST API 클라이언트 |
| `zod` | 입력 스키마 검증 |
| `typescript` | 개발 언어 |
| `tsx` | 개발 시 실행 |

---

## 9. 구현 범위 외

- Suppo 웹훅 수신 (에이전트 트리거) — 별도 프로젝트
- GoGoMail 사용자 생성·삭제 (오퍼레이션 위험도 높음) — 2차 범위
- 멀티 테넌트 권한 분리 — SaaS 확장 시 추가
