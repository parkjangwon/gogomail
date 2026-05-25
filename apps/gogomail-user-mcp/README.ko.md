# GoGoMail 사용자 MCP 서버

English / 영어: [README.md](README.md)

`gogomail-user-mcp`는 GoGoMail 개별 사용자를 위한 사용자 스코프 [Model Context Protocol](https://modelcontextprotocol.io/) 서버입니다. 웹메일을 열지 않아도 Claude Desktop, Claude Code, Codex CLI 등 모든 MCP 호환 AI 에이전트를 자신의 메일함, DM, 주소록, 드라이브, 일정, 알림 설정에 직접 연결할 수 있습니다.

GoGoMail의 **핵심 기능**이자 가장 강력한 자동화 레이어입니다. 123개 툴을 통해 메일 수발신부터 파일 공유, 일정 관리, 팀 메시지까지 모든 작업을 자연어 한 줄 명령으로 처리할 수 있습니다.

---

## 목차

1. [머리말](#머리말)
2. [MCP 액세스 키 발급](#mcp-액세스-키-발급)
3. [설치 및 빌드](#설치-및-빌드)
4. [환경 변수](#환경-변수)
5. [MCP 클라이언트 설정](#mcp-클라이언트-설정)
6. [권한 모드: basic vs bypass](#권한-모드-basic-vs-bypass)
7. [툴 레퍼런스](#툴-레퍼런스)
8. [워크플로우 예시](#워크플로우-예시)
9. [확인 문자열 레퍼런스](#확인-문자열-레퍼런스)
10. [문제 해결](#문제-해결)
11. [관련 문서](#관련-문서)

---

## 머리말

### 이 서버가 하는 일

AI 에이전트는 MCP 서버를 통해 외부 시스템에 접근합니다. `gogomail-user-mcp`는 GoGoMail의 사용자 API 전체 표면을 MCP 툴로 감싸서, 에이전트가 여러분의 메일함과 파일을 직접 다룰 수 있게 합니다.

실제로 이런 일이 가능해집니다.

- "지난 일주일간 `contracts@` 발신 메일을 모두 찾아 PDF 첨부파일만 드라이브 `계약서/2026` 폴더에 저장해줘."
- "오늘 받은 미결 메일 10개를 요약하고, 답장이 필요한 것은 초안을 작성해줘."
- "팀 DM 방에 이 보고서 파일을 올리고 요약 메시지도 같이 보내줘."
- "내일 오전 11시 임원 회의 일정을 만들고 참석자 다섯 명에게 초대 메일을 발송해줘."

### 사용자 MCP vs 관리 MCP

GoGoMail은 두 종류의 MCP 서버를 제공합니다.

| 항목 | `gogomail-user-mcp` (이 서버) | `gogomail-manage-mcp` |
|---|---|---|
| 대상 | 개별 사용자 | 운영자 / 관리자 |
| 인증 키 형식 | `gmu_` 사용자 키 | 관리자 키 |
| 발급 위치 | 웹메일 설정 페이지 | 관리 콘솔 |
| 접근 범위 | 자기 자신의 메일, DM, 드라이브, 일정 | 도메인 전체 사용자, 정책, 감사 로그 |
| 툴 수 | 123개 | 별도 문서 참조 |

관리 MCP는 `apps/gogomail-manage-mcp`에 위치합니다. 운영자용 작업(사용자 계정 생성, 도메인 정책 변경, 스토리지 관리 등)은 그쪽 서버를 사용하세요.

### 안전 모델 요약

- 에이전트가 읽어 오는 메일/주소록/드라이브/캘린더 데이터는 **신뢰할 수 없는 사용자 데이터**입니다. 에이전트는 이 내용을 지시문으로 취급하면 안 됩니다. (프롬프트 인젝션 위험)
- `basic` 모드에서 쓰기/삭제/발송 등 민감한 동작은 **정확한 확인 문자열**이 필요합니다.
- `bypass` 모드에서도 GoGoMail 인증, 키 스코프, 도메인 정책, rate limit, 감사 기록은 변하지 않습니다.
- 발송되는 메일에는 기본적으로 `MCP를 통해 작성된 메일입니다.` 문구가 자동으로 추가됩니다.
- 도메인 관리자는 사용자 MCP 자체, 키 발급, bypass 모드, 생성 메일 문구, 허용 스코프를 통제합니다.

---

## MCP 액세스 키 발급

사용자 MCP 서버는 `gmu_` 로 시작하는 사용자 스코프 액세스 키로 인증합니다. 이 키는 웹메일의 설정 페이지에서 직접 발급합니다.

### 1단계: 도메인 정책 확인

키를 발급하기 전에, 도메인 관리자가 사용자 MCP를 활성화했는지 확인해야 합니다. 관리자가 활성화하지 않았다면 키 발급 UI가 보이지 않거나 오류가 발생합니다. 관리자에게 다음 사항을 요청하세요.

- 도메인 MCP 정책에서 `enabled: true` 설정
- `allow_user_access_keys: true` 설정
- 필요한 스코프 허용 (예: `mail`, `dm`, `drive`, `calendar`, `contacts`)
- 필요한 경우 `allow_bypass_mode: true` 설정

### 2단계: 웹메일 설정 페이지 진입

1. 브라우저에서 GoGoMail 웹메일에 로그인합니다.
2. 우측 상단의 사용자 아이콘 또는 프로필 메뉴를 클릭합니다.
3. **설정(Settings)** 페이지로 이동합니다.
4. 좌측 메뉴에서 **자동화 / MCP** 또는 **액세스 키** 섹션을 찾습니다.

### 3단계: MCP 활성화

설정 페이지의 MCP 섹션에서:

1. **MCP 사용 허용** 토글을 활성화합니다.
2. **권한 모드**를 선택합니다: `basic` (권장) 또는 `bypass`.
3. **생성 메일 문구** 설정을 원하는 대로 조정합니다.

### 4단계: 액세스 키 생성

1. **새 액세스 키 생성** 버튼을 클릭합니다.
2. 키 이름(설명)을 입력합니다. 예: `Claude Desktop - MacBook`
3. 필요한 **스코프**를 선택합니다.

| 스코프 | 접근 가능한 기능 |
|---|---|
| `mail` | 메일 읽기, 발송, 초안, 폴더, 스레드, 첨부파일 |
| `dm` | DM 방 조회/생성, 메시지 발송, 첨부파일 |
| `drive` | 드라이브 파일 탐색, 다운로드, 업로드, 공유 링크 |
| `calendar` | 일정/할일 조회, 생성, 수정 |
| `contacts` | 주소록 조회, 연락처 관리, 디렉터리 검색 |
| `account` | 프로필 조회/수정, 발신 주소, 알림 설정 |
| `spam` | 스팸 신고, not-spam, 발신자 차단/허용 |

4. (선택) **CIDR 허용 목록**을 설정하면 특정 IP 대역에서만 키를 사용할 수 있습니다.
5. (선택) **만료일**을 설정합니다. 설정하지 않으면 수동으로 취소할 때까지 유효합니다.
6. (선택) **키 단위 권한 모드**를 설정합니다. 계정 전체 설정과 다르게 이 키만 `bypass`로 설정하거나 반대로 할 수 있습니다.
7. **생성** 버튼을 클릭합니다.

### 5단계: 키 안전하게 보관

키가 생성되면 전체 토큰(`gmu_` 로 시작하는 긴 문자열)이 **딱 한 번만** 표시됩니다. 백엔드는 해시와 토큰 suffix만 저장합니다.

> **중요**: 이 화면을 닫으면 전체 토큰을 다시 확인할 수 없습니다. 반드시 안전한 곳(비밀번호 관리자, 환경 변수 파일 등)에 즉시 저장하세요.

### 키 관리

이미 발급된 키는 설정 페이지의 키 목록에서 관리합니다.

- **조회**: 키 이름, suffix, 스코프, 만료일, 마지막 사용 시각을 확인합니다.
- **취소**: 더 이상 사용하지 않는 키는 즉시 취소합니다. 취소된 키는 즉시 `401` 오류를 반환합니다.
- **권한 모드 변경**: 설정 페이지에서 계정 전체 권한 모드를 변경할 수 있습니다.

---

## 설치 및 빌드

### 요구사항

- Node.js 20 이상 (`node --version`으로 확인)
- npm 10 이상
- 접근 가능한 GoGoMail 웹/API 서버
- 웹메일에서 발급한 `gmu_` 액세스 키

### 저장소 클론 및 빌드

```bash
# 저장소가 이미 있다면 해당 디렉터리로 이동
cd /path/to/gogomail/apps/gogomail-user-mcp

# 의존성 설치
npm install

# TypeScript 빌드
npm run build
```

빌드가 완료되면 `dist/index.js` 파일이 생성됩니다. 이 파일이 MCP 서버 엔트리포인트입니다.

### 로컬 검증

```bash
# 타입 체크
npm run type-check

# 단위 테스트
npm test

# 전체 빌드 (타입 체크 + 컴파일)
npm run build
```

### 개발 환경에서의 Docker 사용

로컬 Docker 개발 환경을 사용하는 경우 `GOGOMAIL_API_URL`을 `http://localhost:8080`으로 설정합니다. `dev-compose.yml`로 전체 스택을 띄우는 방법은 프로젝트 루트의 `docs/DEPLOYMENT.md`를 참조하세요.

---

## 환경 변수

서버 시작 시 다음 환경 변수를 읽습니다.

| 변수 | 필수 | 기본값 | 설명 |
|---|---|---|---|
| `GOGOMAIL_API_URL` | 필수 | - | GoGoMail 웹/API 서버의 origin. 예: `https://mail.company.com` 또는 `http://localhost:8080`. 후행 슬래시는 자동으로 제거됩니다. |
| `GOGOMAIL_USER_MCP_KEY` | 필수 | - | 웹메일 설정에서 발급한 사용자 스코프 MCP 액세스 키 (`gmu_` 로 시작). |
| `GOGOMAIL_MCP_PERMISSION_MODE` | 선택 | `basic` | 로컬 폴백 권한 모드. `basic` 또는 `bypass`. 서버에 저장된 사용자 MCP 설정이 존재하면 그 값이 우선합니다. 이 환경 변수는 서버 설정을 읽어오기 전 폴백 또는 서버 설정 없이 테스트할 때 사용합니다. |

### 유효성 검사

서버 시작 시 각 변수를 검증합니다.

- `GOGOMAIL_API_URL`: 유효한 URL이어야 하며 `http://` 또는 `https://` 스키마를 사용해야 합니다. URL에 자격증명(username:password)을 포함할 수 없습니다.
- `GOGOMAIL_USER_MCP_KEY`: 비어 있거나 줄 바꿈 문자를 포함할 수 없습니다.
- `GOGOMAIL_MCP_PERMISSION_MODE`: `basic` 또는 `bypass` 중 하나여야 합니다.

검증 실패 시 서버는 즉시 종료되고 표준 오류에 메시지를 출력합니다.

---

## MCP 클라이언트 설정

빌드된 `dist/index.js`를 Node.js로 실행하는 방식으로 모든 MCP 클라이언트에서 사용할 수 있습니다. 서버는 **stdio 트랜스포트**를 사용합니다.

### Claude Desktop

Claude Desktop 설정 파일 위치:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/claude/claude_desktop_config.json`

파일을 열고 `mcpServers` 섹션에 다음을 추가합니다.

```json
{
  "mcpServers": {
    "gogomail-user-mcp": {
      "command": "node",
      "args": ["/절대/경로/gogomail/apps/gogomail-user-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_API_URL": "https://mail.company.com",
        "GOGOMAIL_USER_MCP_KEY": "gmu_여기에_발급받은_키를_입력",
        "GOGOMAIL_MCP_PERMISSION_MODE": "basic"
      }
    }
  }
}
```

설정 후 Claude Desktop을 **완전히 종료 후 재시작**합니다. 도구 목록에 `gogomail_*` 툴이 표시되면 정상입니다.

**여러 계정을 사용하는 경우** 서버 이름을 다르게 지정하면 됩니다.

```json
{
  "mcpServers": {
    "gogomail-work": {
      "command": "node",
      "args": ["/경로/dist/index.js"],
      "env": {
        "GOGOMAIL_API_URL": "https://mail.company.com",
        "GOGOMAIL_USER_MCP_KEY": "gmu_work_key_xxx"
      }
    },
    "gogomail-personal": {
      "command": "node",
      "args": ["/경로/dist/index.js"],
      "env": {
        "GOGOMAIL_API_URL": "https://mail.personal.com",
        "GOGOMAIL_USER_MCP_KEY": "gmu_personal_key_xxx"
      }
    }
  }
}
```

### Claude Code

Claude Code 프로젝트 디렉터리의 `.claude/mcp.json` 파일 (또는 글로벌 `~/.claude/mcp.json`)에 추가합니다.

```json
{
  "mcpServers": {
    "gogomail-user-mcp": {
      "command": "node",
      "args": ["/절대/경로/gogomail/apps/gogomail-user-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_API_URL": "https://mail.company.com",
        "GOGOMAIL_USER_MCP_KEY": "gmu_여기에_발급받은_키를_입력",
        "GOGOMAIL_MCP_PERMISSION_MODE": "basic"
      }
    }
  }
}
```

또는 Claude Code 설정 파일(`~/.claude/settings.json`)의 `mcpServers` 섹션에 동일하게 추가할 수 있습니다.

연결 확인:

```
/mcp
```

`gogomail-user-mcp`가 연결된 서버 목록에 표시되고 툴 수(123개)가 나타나면 정상입니다.

### Codex CLI

Codex CLI 설정 파일 (`~/.codex/config.yaml` 또는 프로젝트의 `codex.yaml`)에 추가합니다.

```yaml
mcp_servers:
  - name: gogomail-user-mcp
    command: node
    args:
      - /절대/경로/gogomail/apps/gogomail-user-mcp/dist/index.js
    env:
      GOGOMAIL_API_URL: https://mail.company.com
      GOGOMAIL_USER_MCP_KEY: gmu_여기에_발급받은_키를_입력
      GOGOMAIL_MCP_PERMISSION_MODE: basic
```

### 공통 주의사항

- 경로는 반드시 **절대 경로**를 사용합니다. 상대 경로는 클라이언트에 따라 동작이 다를 수 있습니다.
- 액세스 키는 환경 변수에 직접 넣거나, 클라이언트가 지원하는 경우 `.env` 파일 또는 시스템 키체인을 사용합니다.
- 키를 버전 관리 시스템(git)에 커밋하지 않도록 주의합니다.
- 로컬 Docker 개발 환경에서는 `GOGOMAIL_API_URL`을 `http://localhost:8080`으로 설정합니다.

---

## 권한 모드: basic vs bypass

GoGoMail 사용자 MCP는 두 가지 권한 모드를 제공합니다. 모드는 서버에 저장된 사용자 MCP 설정이 최우선이며, 없는 경우 환경 변수 `GOGOMAIL_MCP_PERMISSION_MODE`가 폴백으로 사용됩니다.

### basic 모드 (권장)

**쓰기, 삭제, 발송 등 민감한 동작에 정확한 확인 문자열이 필요합니다.**

에이전트가 민감한 툴을 호출할 때 `confirm` 파라미터에 지정된 문자열을 정확히 전달해야 합니다. 문자열이 없거나 틀리면 툴은 즉시 오류를 반환합니다.

예시: 메일 발송

```json
{
  "to": ["alice@company.com"],
  "subject": "회의 자료 공유",
  "body_text": "안녕하세요, 회의 자료를 공유합니다.",
  "confirm": "send message"
}
```

`confirm` 없이 호출하면:

```
Error: confirmation required: pass confirm="send message" to proceed
```

예시: 드라이브 파일 다운로드 후 로컬 저장

```json
{
  "id": "file-abc123",
  "save_to_path": "/tmp/report.pdf",
  "confirm": "save download /tmp/report.pdf"
}
```

**basic 모드의 장점**:

- 에이전트가 실수 또는 악의적 프롬프트 인젝션으로 예상치 못한 쓰기를 하는 것을 방지합니다.
- 사용자가 실제로 어떤 동작이 수행되는지 검토할 수 있습니다.
- 도메인 정책의 기본값이며, 대부분의 환경에서 권장됩니다.

### bypass 모드

**확인 문자열 없이 모든 툴을 바로 실행합니다.**

에이전트가 확인 단계를 거치지 않고 곧바로 메일을 발송하거나 파일을 삭제할 수 있습니다. 자동화 스크립트나 충분히 신뢰할 수 있는 환경에서 사용합니다.

> **주의**: bypass 모드는 도메인 관리자가 `allow_bypass_mode: true`를 설정해야만 사용할 수 있습니다. 도메인 정책에서 허용하지 않으면 bypass 모드 키 발급과 사용 모두 차단됩니다.

**bypass 모드에서도 적용되는 제한**:

- GoGoMail API 인증 (Bearer 토큰 검증)
- 키별 스코프 제한
- 도메인 MCP 정책 (enabled, allowed_scopes 등)
- Rate limit 및 일일 발송 한도
- 감사 로그 및 사용량 기록
- 백엔드 검증 (수신자 형식, 파일 존재 여부 등)

### 모드 결정 우선순위

```
서버 저장 사용자 MCP 설정 (permission_mode)
    ↓ 없는 경우
환경 변수 GOGOMAIL_MCP_PERMISSION_MODE
    ↓ 없는 경우
기본값: basic
```

### 키별 권한 모드

웹메일 설정에서 개별 액세스 키마다 다른 권한 모드를 설정할 수 있습니다.

- 계정 전체는 `basic`이지만 특정 자동화 스크립트용 키는 `bypass`
- 반대로 계정은 `bypass`이지만 공유 환경용 키는 `basic`

이 설정은 키 발급 시 또는 이후 설정 페이지에서 변경 가능합니다.

### external recipients 확인

사용자 MCP 설정에서 **외부 수신자 확인**을 활성화하면, 외부 도메인(회사 도메인이 아닌) 주소로 메일을 발송할 때 추가 확인 파라미터가 필요합니다.

```json
{
  "to": ["외부주소@gmail.com"],
  "subject": "외부 발송 테스트",
  "body_text": "안녕하세요.",
  "confirm": "send message",
  "confirm_external_recipients": "external recipients"
}
```

---

## 툴 레퍼런스

총 123개 툴을 12개 그룹으로 분류합니다. 테이블의 "basic 확인 문자열" 열이 있는 툴은 `basic` 모드에서 해당 문자열을 `confirm` 파라미터로 전달해야 합니다.

---

### 계정/컨텍스트 (9개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_mcp_get_settings` | 현재 MCP 자동화 설정 읽기 (권한 모드, 생성 메일 문구 등) | 없음 | - |
| `gogomail_webmail_get_capabilities` | 웹메일 기능 한도 조회 (첨부파일 최대 크기, 메일함 한도 등) | 없음 | - |
| `gogomail_mailbox_get_overview` | 메일함 요약 (총 메시지 수, 읽지 않은 수, 폴더별 카운트) | 없음 | - |
| `gogomail_account_get_profile` | 사용자 프로필 및 할당량 조회 (이름, 이메일, 사용 용량) | 없음 | - |
| `gogomail_account_update_profile` | 프로필 업데이트 | `display_name`, `recovery_email` | - |
| `gogomail_account_list_addresses` | 발신 주소 목록 조회 (기본 주소 및 별칭 목록) | 없음 | - |
| `gogomail_account_upload_avatar` | 프로필 사진 업로드 (PNG/JPEG/GIF/WebP, 최대 256KiB) | `content_base64`, `mime_type` | `upload avatar` |
| `gogomail_account_delete_avatar` | 프로필 사진 삭제 | 없음 | `delete avatar` |
| `gogomail_preferences_get` | 웹메일 환경설정 전체 읽기 | 없음 | - |
| `gogomail_api_request` | `/api/v1` 및 `/api/mail` 라우트 직접 호출용 generic bridge | `method`, `path`, `body` | 경로에 따라 다름 |

---

### 알림/푸시 (9개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_notifications_get_preferences` | 알림 환경설정 조회 (DND, 폴더별, 스레드별 설정) | 없음 | - |
| `gogomail_notifications_update_preferences` | 알림 환경설정 업데이트 | `dnd_enabled`, `dnd_start`, `dnd_end`, `folder_preferences` | - |
| `gogomail_notifications_get_web_push_config` | Web Push 공개 키 설정 조회 | 없음 | - |
| `gogomail_notifications_list_push_subscriptions` | 브라우저 Web Push 구독 목록 조회 | 없음 | - |
| `gogomail_notifications_upsert_push_subscription` | Web Push 구독 등록/업데이트 | `endpoint`, `keys` | - |
| `gogomail_notifications_delete_push_subscription` | Web Push 구독 삭제 | `endpoint` | `delete push subscription <endpoint>` |
| `gogomail_notifications_list_push_devices` | 푸시 디바이스 목록 조회 | 없음 | - |
| `gogomail_notifications_upsert_push_device` | 푸시 디바이스 등록/업데이트 | `device_id`, `platform`, `token` | - |
| `gogomail_notifications_delete_push_device` | 푸시 디바이스 삭제 | `device_id` | `delete push device <device_id>` |

---

### 메일 (14개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_mail_search` | 전체 텍스트 검색 | `q`, `folder`, `from`, `to`, `since`, `until`, `limit` | - |
| `gogomail_mail_list_messages` | 폴더별 메시지 목록 (페이지네이션) | `folder_id`, `limit`, `before`, `after` | - |
| `gogomail_mail_get_message` | 메시지 전체 조회 (헤더/본문/첨부파일 포함) | `id` | - |
| `gogomail_mail_send` | 메일 발송 | `to`, `cc`, `bcc`, `subject`, `body_text`, `body_html`, `attachments`, `from_address`, `thread_id` | `send message` (외부 수신자 시 `confirm_external_recipients: "external recipients"` 추가 필요) |
| `gogomail_mail_save_draft` | 초안 저장 | `to`, `subject`, `body_text`, `body_html` | - |
| `gogomail_mail_search_drafts` | 초안 검색 | `q`, `limit` | - |
| `gogomail_mail_send_draft` | 저장된 초안 발송 | `id` | `send draft <id>` |
| `gogomail_mail_delete_draft` | 초안 삭제 | `id` | `delete draft <id>` |
| `gogomail_mail_restore_message` | 휴지통에서 메시지 복원 | `id` | - |
| `gogomail_mail_update_flags` | 플래그 설정 (읽음/별표/깃발/레이블) | `id`, `flags` (read, starred, flagged, labels) | - |
| `gogomail_mail_move_message` | 메시지를 다른 폴더로 이동 | `id`, `folder_id` | - |
| `gogomail_mail_delete_message` | 메시지를 휴지통으로 이동 | `id` | `delete message <id>` |
| `gogomail_mail_delivery_status` | 발송된 메시지의 배달 상태 확인 | `id` | - |
| `gogomail_mail_get_tracking` | 메시지 열람 트래킹 기록 조회 | `id` | - |

---

### 메일 bulk (8개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_mail_bulk_update_flags` | 여러 메시지 플래그 일괄 변경 | `message_ids`, `flags` | - |
| `gogomail_mail_bulk_move_messages` | 여러 메시지 일괄 폴더 이동 | `message_ids`, `folder_id` | - |
| `gogomail_mail_bulk_delete_messages` | 여러 메시지 일괄 휴지통 이동 | `message_ids` | `bulk delete messages` |
| `gogomail_mail_bulk_restore_messages` | 여러 메시지 일괄 복원 | `message_ids` | - |
| `gogomail_mail_bulk_update_thread_flags` | 여러 스레드 플래그 일괄 변경 | `thread_ids`, `flags` | - |
| `gogomail_mail_bulk_move_threads` | 여러 스레드 일괄 폴더 이동 | `thread_ids`, `folder_id` | - |
| `gogomail_mail_bulk_delete_threads` | 여러 스레드 일괄 휴지통 이동 | `thread_ids` | `bulk delete threads` |
| `gogomail_mail_bulk_restore_threads` | 여러 스레드 일괄 복원 | `thread_ids` | - |

---

### 폴더/스레드 (6개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_mail_list_folders` | 전체 폴더 목록 조회 | 없음 | - |
| `gogomail_mail_create_folder` | 새 폴더 생성 | `name`, `parent_id` | `create folder` |
| `gogomail_mail_rename_folder` | 폴더 이름 변경 | `id`, `name` | - |
| `gogomail_mail_delete_folder` | 폴더 삭제 | `id` | `delete folder <id>` |
| `gogomail_mail_list_threads` | 폴더 내 스레드 목록 | `folder_id`, `limit`, `before` | - |
| `gogomail_mail_get_thread_messages` | 스레드의 모든 메시지 조회 | `thread_id` | - |

---

### 첨부파일 (5개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_mail_list_attachments` | 메시지의 첨부파일 목록 | `message_id` | - |
| `gogomail_mail_download_attachment` | 첨부파일 다운로드 (선택적 로컬 저장) | `message_id`, `attachment_id`, `save_to_path` | 로컬 저장 시: `save download <path>` |
| `gogomail_mail_get_attachment_upload_capabilities` | 첨부파일 업로드 한도 조회 | 없음 | - |
| `gogomail_mail_create_text_attachment` | 초안에 텍스트 첨부파일 생성 | `draft_id`, `filename`, `content` | - |
| `gogomail_mail_cancel_attachment_upload` | 진행 중인 첨부 업로드 취소 | `upload_id` | - |

---

### DM (18개 툴)

DM 툴은 암호화된 참여자 전용 방/메시지 계약을 사용합니다. 참여 중인 방의 메시지는 참여자만 읽을 수 있습니다.

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_dm_list_rooms` | 참여 중인 DM 방 목록 | `limit`, `before` | - |
| `gogomail_dm_list_public_rooms` | 도메인 내 참여 가능한 공개 방 목록 | `limit`, `q` | - |
| `gogomail_dm_create_room` | 1:1 또는 그룹 DM 방 생성 | `room_type` (direct/group), `user_ids`, `name`, `visibility` | `create dm room` |
| `gogomail_dm_add_members` | 그룹 방에 멤버 추가 | `room_id`, `user_ids` | `add dm members <room_id>` |
| `gogomail_dm_remove_member` | 멤버 제거 또는 방 나가기 | `room_id`, `user_id` | `remove dm member <room_id> <user_id>` |
| `gogomail_dm_transfer_owner` | 그룹 방 소유권 이전 | `room_id`, `user_id` | `transfer dm owner <room_id> <user_id>` |
| `gogomail_dm_create_invite` | 초대 링크 생성 | `room_id`, `expires_in` | `create dm invite <room_id>` |
| `gogomail_dm_join_invite` | 초대 토큰으로 방 참여 | `token` | `join dm invite <token>` |
| `gogomail_dm_list_messages` | 방의 메시지 목록 (페이지네이션) | `room_id`, `before`, `after`, `limit` | - |
| `gogomail_dm_send_message` | 텍스트 또는 Drive 링크 메시지 발송 | `room_id`, `body`, `drive_file_id` | `send dm message <room_id>` |
| `gogomail_dm_send_attachment` | 파일 첨부 메시지 발송 (최대 20MiB) | `room_id`, `filename`, `mime_type`, `content_base64` | `send dm attachment <room_id>` |
| `gogomail_dm_mark_read` | 방 읽음 처리 | `room_id` | - |
| `gogomail_dm_search` | 방 내 메시지 검색 | `room_id`, `q`, `limit` | - |
| `gogomail_dm_list_media` | 방의 미디어/링크 목록 | `room_id`, `type` (file/drive_link/link) | - |
| `gogomail_dm_download_attachment` | 첨부파일 다운로드 (선택적 로컬 저장) | `room_id`, `message_id`, `attachment_id`, `save_to_path` | 로컬 저장 시: `save download <path>` |
| `gogomail_dm_edit_message` | 메시지 텍스트 수정 | `room_id`, `message_id`, `body` | `edit dm message <message_id>` |
| `gogomail_dm_delete_message` | 메시지 삭제 | `room_id`, `message_id` | `delete dm message <message_id>` |
| `gogomail_dm_toggle_reaction` | 이모지 반응 토글 | `room_id`, `message_id`, `emoji` | - |

---

### 주소록/디렉터리 (14개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_contacts_list_addressbooks` | 주소록 목록 조회 | 없음 | - |
| `gogomail_contacts_create_addressbook` | 새 주소록 생성 | `name`, `description` | - |
| `gogomail_contacts_get_addressbook` | 주소록 상세 조회 | `id` | - |
| `gogomail_contacts_update_addressbook` | 주소록 수정 | `id`, `name`, `description` | - |
| `gogomail_contacts_delete_addressbook` | 주소록 삭제 | `id` | - |
| `gogomail_contacts_list` | 주소록 내 연락처 목록 | `addressbook_id`, `limit`, `before` | - |
| `gogomail_contacts_get` | 연락처 상세 조회 | `addressbook_id`, `contact_id` | - |
| `gogomail_contacts_upsert_simple` | 연락처 생성/업데이트 (구조화된 필드 입력, vCard 자동 생성) | `addressbook_id`, `name`, `email`, `phone`, `address` | - |
| `gogomail_contacts_upsert` | 연락처 생성/업데이트 (원본 vCard 직접 입력) | `addressbook_id`, `vcard` | - |
| `gogomail_contacts_delete` | 연락처 삭제 | `addressbook_id`, `contact_id` | - |
| `gogomail_contacts_autocomplete` | 이름/이메일로 연락처 자동완성 | `q`, `limit` | - |
| `gogomail_directory_search_users` | 도메인 디렉터리에서 사용자 검색 | `q`, `limit` | - |
| `gogomail_directory_org_tree` | 조직도 조회 | 없음 | - |
| `gogomail_directory_get_profile` | 특정 사용자의 디렉터리 프로필 조회 | `user_id` | - |

---

### 스팸 (5개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_spam_report_message` | 메시지를 스팸으로 신고 | `message_id` | - |
| `gogomail_spam_mark_not_spam` | 메시지를 스팸 아님으로 표시 | `message_id` | - |
| `gogomail_spam_list_senders` | 차단/허용 발신자 목록 조회 | `type` (block/allow), `limit` | - |
| `gogomail_spam_add_sender` | 발신자 차단/허용 목록에 추가 | `email`, `type` (block/allow) | - |
| `gogomail_spam_remove_sender` | 발신자 목록에서 제거 | `email`, `type` | - |

---

### 드라이브 (20개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_drive_list` | 파일/폴더 목록 조회 | `parent_id`, `query`, `limit` | - |
| `gogomail_drive_get` | 파일/폴더 메타데이터 조회 | `id` | - |
| `gogomail_drive_download` | 파일 다운로드 (body_text/body_base64/content_type 반환) | `id`, `save_to_path` | 로컬 저장 시: `save download <path>` |
| `gogomail_drive_create_folder` | 폴더 생성 | `name`, `parent_id` | - |
| `gogomail_drive_create_text_file` | 텍스트 파일 생성 (업로드 세션 사용) | `name`, `content`, `parent_id`, `mime_type` | - |
| `gogomail_drive_list_upload_sessions` | 진행 중인 업로드 세션 목록 | 없음 | - |
| `gogomail_drive_get_upload_session` | 업로드 세션 상태 조회 | `session_id` | - |
| `gogomail_drive_cancel_upload_session` | 업로드 세션 취소 | `session_id` | - |
| `gogomail_drive_rename` | 파일/폴더 이름 변경 | `id`, `name` | - |
| `gogomail_drive_move` | 다른 폴더로 이동 | `id`, `parent_id` | - |
| `gogomail_drive_copy` | 파일 복사 | `id`, `parent_id`, `name` | - |
| `gogomail_drive_trash` | 파일/폴더를 휴지통으로 이동 | `id` | `trash drive <id>` |
| `gogomail_drive_restore` | 휴지통에서 복원 | `id` | - |
| `gogomail_drive_delete` | 영구 삭제 (이미 휴지통에 있어야 함) | `id` | `delete drive <id>` |
| `gogomail_drive_share_link` | 공유 링크 생성 | `id`, `access` (view/download), `expires_in` | `share drive <id>` |
| `gogomail_drive_get_share_link` | 공유 링크 정보 조회 | `link_id` | - |
| `gogomail_drive_download_share_link` | 공유 링크로 파일 다운로드 | `token`, `save_to_path` | 로컬 저장 시: `save download <path>` |
| `gogomail_drive_usage` | 스토리지 사용량 통계 조회 | 없음 | - |
| `gogomail_drive_list_share_links` | 모든 공유 링크 목록 | `limit` | - |
| `gogomail_drive_delete_share_link` | 공유 링크 삭제 | `link_id` | - |

> **영구 삭제 주의**: `gogomail_drive_delete`는 이미 휴지통에 있는 파일에만 적용됩니다. 활성 파일을 영구 삭제하려면 먼저 `gogomail_drive_trash`로 휴지통으로 보낸 뒤 `gogomail_drive_delete`를 호출해야 합니다.

---

### 일정 (14개 툴)

| 툴 이름 | 설명 | 주요 파라미터 | basic 확인 문자열 |
|---|---|---|---|
| `gogomail_calendar_list` | 캘린더 목록 조회 | 없음 | - |
| `gogomail_calendar_create` | 새 캘린더 생성 | `name`, `color`, `description` | - |
| `gogomail_calendar_get` | 캘린더 상세 조회 | `id` | - |
| `gogomail_calendar_update` | 캘린더 수정 | `id`, `name`, `color` | - |
| `gogomail_calendar_delete` | 캘린더 삭제 | `id` | - |
| `gogomail_calendar_list_objects` | 캘린더의 이벤트/할일 목록 | `calendar_id`, `since`, `until` | - |
| `gogomail_calendar_get_object` | 이벤트/할일 상세 조회 | `calendar_id`, `object_id` | - |
| `gogomail_calendar_upsert_object` | 원본 ICS로 이벤트/할일 생성/수정 | `calendar_id`, `object_id`, `ics` | - |
| `gogomail_calendar_upsert_event_simple` | 간편 이벤트 생성 (구조화된 필드, ICS 자동 생성) | `calendar_id`, `title`, `start`, `end`, `description`, `location` | - |
| `gogomail_calendar_delete_object` | 이벤트/할일 삭제 | `calendar_id`, `object_id` | - |
| `gogomail_calendar_list_subscriptions` | 구독 캘린더 목록 | 없음 | - |
| `gogomail_calendar_create_subscription` | 캘린더 구독 추가 | `url`, `name` | - |
| `gogomail_calendar_delete_subscription` | 캘린더 구독 취소 | `id` | - |
| `gogomail_calendar_get_subscription_events` | 구독 캘린더 이벤트 조회 | `id`, `since`, `until` | - |

---

## 워크플로우 예시

아래 예시는 실제로 AI 에이전트에게 입력할 수 있는 프롬프트와, 에이전트가 어떤 툴을 어떤 순서로 호출하는지를 보여줍니다.

---

### 예시 1: 미결 메일 일괄 정리

**상황**: 일주일 출장 후 돌아와서 받은편지함에 쌓인 메일을 정리하고 싶습니다.

**프롬프트**:

```
받은편지함에서 읽지 않은 메일을 모두 가져와줘. 뉴스레터처럼 보이는 것은 별도로 분류하고,
발신자가 사내 도메인(@company.com)인 것은 읽음 처리해줘.
정말 중요해 보이는 것(제목에 "긴급", "urgent", "ASAP"가 있는 것)은 별표를 달아줘.
나머지 외부 메일은 "검토 필요" 폴더로 옮겨줘. 이 폴더가 없으면 만들어줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_mail_list_folders` - 현재 폴더 목록 확인
2. `gogomail_mail_search` - `q: "is:unread"`, `folder: "INBOX"` 로 읽지 않은 메일 검색
3. (폴더 없으면) `gogomail_mail_create_folder` - `name: "검토 필요"`, `confirm: "create folder"`
4. `gogomail_mail_bulk_update_flags` - 사내 도메인 메시지 IDs, `flags: {read: true}`
5. `gogomail_mail_bulk_update_flags` - 긴급 메시지 IDs, `flags: {starred: true}`
6. `gogomail_mail_bulk_move_messages` - 외부 메시지 IDs, `folder_id: <검토 필요 ID>`

---

### 예시 2: 특정 발신자 메일 폴더 이동 및 별표

**상황**: 청구서나 영수증 관련 메일을 자동으로 정리하고 싶습니다.

**프롬프트**:

```
billing@acme.com 에서 온 메일을 모두 찾아줘. 별표를 달고 "Finance/ACME" 폴더로 옮겨줘.
이 폴더가 없으면 "Finance" 하위에 만들어줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_mail_search` - `from: "billing@acme.com"`, `limit: 100`
2. `gogomail_mail_list_folders` - `Finance` 폴더 확인
3. `gogomail_mail_create_folder` - `name: "ACME"`, `parent_id: <Finance 폴더 ID>`, `confirm: "create folder"`
4. `gogomail_mail_bulk_update_flags` - 모든 메시지 IDs, `flags: {starred: true}`
5. `gogomail_mail_bulk_move_messages` - 모든 메시지 IDs, `folder_id: <ACME 폴더 ID>`

---

### 예시 3: 메일 초안 작성 및 검토 후 발송

**상황**: 벤더에게 미팅 제안 메일을 보내야 하는데, 발송 전에 내용을 확인하고 싶습니다.

**프롬프트**:

```
다음 내용으로 메일 초안을 만들어줘:
수신자: vendor@partner.com
제목: 2026년 2분기 협업 미팅 제안
내용: 이번 분기 신규 프로젝트 관련해서 미팅을 제안하고 싶습니다. 
     6월 첫째 주 중 30분 정도 시간이 가능하신지 확인 부탁드립니다.
초안을 보여줘. 아직 발송하지 마.
```

**에이전트 실행 흐름**:

1. `gogomail_mail_save_draft` - 초안 저장
2. `gogomail_mail_get_message` - 저장된 초안 내용 조회 후 사용자에게 보여줌

**사용자가 확인 후 발송 요청**:

```
좋아. 발송해줘.
```

**에이전트**:

```
초안 ID <draft_id>를 발송하겠습니다. basic 모드에서 확인이 필요합니다.
gogomail_mail_send_draft 호출: confirm="send draft <draft_id>"
```

---

### 예시 4: DM 그룹방 생성 및 파일 공유

**상황**: 프로젝트 팀원들과 새 그룹 DM 방을 만들고, 프로젝트 계획서를 공유하고 싶습니다.

**프롬프트**:

```
alice@company.com, bob@company.com, carol@company.com 세 명과 함께
"Q3 프로젝트 팀" 이라는 그룹 DM 방을 만들어줘.
그리고 내 드라이브에 있는 "Q3계획.txt" 파일을 그 방에 공유해줘.
파일 링크와 함께 "Q3 프로젝트 킥오프 자료입니다." 라는 메시지도 같이 보내줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_directory_search_users` - alice, bob, carol의 user_id 확인
2. `gogomail_dm_create_room` - `room_type: "group"`, `name: "Q3 프로젝트 팀"`, `user_ids: [...]`, `confirm: "create dm room"`
3. `gogomail_drive_list` - `query: "Q3계획.txt"` 로 파일 검색
4. `gogomail_dm_send_message` - `room_id: <방 ID>`, `body: "Q3 프로젝트 킥오프 자료입니다."`, `drive_file_id: <파일 ID>`, `confirm: "send dm message <room_id>"`

---

### 예시 5: 드라이브에 보고서 업로드 후 공유 링크 생성

**상황**: 작성한 분석 보고서를 드라이브에 올리고 팀원에게 공유할 수 있는 링크를 만들고 싶습니다.

**프롬프트**:

```
다음 내용으로 드라이브에 "2026-05-분석보고서.txt" 라는 파일을 만들어줘:
[보고서 내용 입력...]
파일이 생성되면 7일간 유효한 다운로드 링크를 만들어서 링크를 알려줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_drive_list` - 루트 폴더 확인
2. `gogomail_drive_create_text_file` - `name: "2026-05-분석보고서.txt"`, `content: "..."`, `mime_type: "text/plain"`
3. `gogomail_drive_share_link` - `id: <파일 ID>`, `access: "download"`, `expires_in: 604800`, `confirm: "share drive <id>"`
4. 생성된 공유 링크 URL을 사용자에게 반환

---

### 예시 6: 일정 생성 및 참석자 메일 발송

**상황**: 내일 오후 2시에 팀 주간 회의를 만들고 참석자들에게 초대 메일을 보내야 합니다.

**프롬프트**:

```
내일(2026년 5월 27일) 오후 2시부터 3시까지 "팀 주간 회의" 일정을 만들어줘.
장소는 "2층 대회의실"이고, 설명은 "주간 진행 상황 공유 및 이슈 논의".
그리고 team@company.com 으로 참석 안내 메일도 보내줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_calendar_list` - 기본 캘린더 확인
2. `gogomail_calendar_upsert_event_simple` - `title: "팀 주간 회의"`, `start: "2026-05-27T14:00:00"`, `end: "2026-05-27T15:00:00"`, `location: "2층 대회의실"`, `description: "주간 진행 상황 공유 및 이슈 논의"`
3. `gogomail_mail_send` - 참석 안내 메일 발송, `confirm: "send message"`

**다음 주부터 매주 반복 설정**이 필요하다면 `gogomail_calendar_upsert_object`로 원본 ICS에 `RRULE:FREQ=WEEKLY`를 추가합니다.

---

### 예시 7: 주소록 정리 및 연락처 일괄 업데이트

**상황**: 거래처 연락처가 흩어져 있어서 새 주소록을 만들고 정리하고 싶습니다.

**프롬프트**:

```
"거래처 2026" 이라는 주소록을 새로 만들어줘.
그리고 다음 연락처들을 추가해줘:
1. 이름: 김민준, 이메일: minjun@acme.com, 전화: 010-1234-5678, 회사: ACME Corp
2. 이름: Sarah Johnson, 이메일: sarah@partner.co, 전화: +1-415-555-0100
기존 "기본 주소록"에서 "acme.com" 이메일이 있는 연락처를 찾아서 새 주소록으로 복사해줘.
```

**에이전트 실행 흐름**:

1. `gogomail_contacts_list_addressbooks` - 기존 주소록 목록 확인
2. `gogomail_contacts_create_addressbook` - `name: "거래처 2026"`
3. `gogomail_contacts_upsert_simple` - 김민준 연락처 추가
4. `gogomail_contacts_upsert_simple` - Sarah Johnson 연락처 추가
5. `gogomail_contacts_list` - 기본 주소록에서 연락처 목록 조회
6. 각 연락처의 이메일을 확인해서 `acme.com` 도메인이 있는 것들을 `gogomail_contacts_upsert_simple`로 새 주소록에 추가

---

### 예시 8: 스팸 신고 및 발신자 차단

**상황**: 반복적으로 스팸 메일을 보내는 발신자를 차단하고 싶습니다.

**프롬프트**:

```
spam@malicious.com 에서 온 메일이 계속 오고 있어. 이 발신자를 차단해줘.
그리고 지난 1주일간 이 발신자에게서 온 메일을 모두 스팸 신고하고 삭제해줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_spam_add_sender` - `email: "spam@malicious.com"`, `type: "block"`
2. `gogomail_mail_search` - `from: "spam@malicious.com"`, `since: "7일 전 날짜"`
3. 각 메시지에 대해 `gogomail_spam_report_message` 호출
4. `gogomail_mail_bulk_delete_messages` - 모든 메시지 IDs, `confirm: "bulk delete messages"`

---

### 예시 9: 알림 설정 최적화

**상황**: 야간에 알림을 끄고, 중요 폴더의 메일만 알림 받도록 설정하고 싶습니다.

**프롬프트**:

```
밤 11시부터 아침 8시까지 방해 금지 모드를 켜줘.
"VIP" 폴더와 "Finance" 폴더에서 온 메일은 방해 금지 중에도 알림이 오도록 설정해줘.
```

**에이전트 실행 흐름**:

1. `gogomail_notifications_get_preferences` - 현재 알림 설정 확인
2. `gogomail_notifications_update_preferences` - `dnd_enabled: true`, `dnd_start: "23:00"`, `dnd_end: "08:00"`, `folder_preferences: [{folder: "VIP", bypass_dnd: true}, {folder: "Finance", bypass_dnd: true}]`

---

### 예시 10: 드라이브 정기 백업 및 정리

**상황**: 드라이브 사용량을 확인하고, 오래된 파일을 정리하고 싶습니다.

**프롬프트**:

```
내 드라이브 사용량을 알려줘.
"임시" 폴더에 있는 파일 중 이름에 "2024"가 포함된 것을 모두 찾아서 목록을 보여줘.
내가 삭제를 확인하면 그때 삭제해줘.
```

**에이전트 실행 흐름** (basic 모드):

1. `gogomail_drive_usage` - 스토리지 사용량 조회
2. `gogomail_drive_list` - `query: "2024"`, `parent_id: <임시 폴더 ID>`
3. 목록을 사용자에게 보여주고 확인 요청
4. 사용자 확인 후 각 파일에 대해 `gogomail_drive_trash` - `confirm: "trash drive <id>"`

---

## 확인 문자열 레퍼런스

`basic` 모드에서 민감한 툴 호출 시 `confirm` 파라미터에 정확히 입력해야 하는 문자열 목록입니다. 문자열은 대소문자를 구분하며 정확히 일치해야 합니다. `<id>`, `<path>` 등은 실제 값으로 대체합니다.

### 메일

| 동작 | confirm 문자열 | 설명 |
|---|---|---|
| 메일 발송 | `send message` | 신규 메일 발송 |
| 초안 발송 | `send draft <id>` | 저장된 초안 발송 (`<id>`는 실제 초안 ID) |
| 초안 삭제 | `delete draft <id>` | 초안 삭제 |
| 메시지 삭제 | `delete message <id>` | 메시지를 휴지통으로 이동 |
| 메시지 bulk 삭제 | `bulk delete messages` | 여러 메시지 일괄 삭제 |
| 스레드 bulk 삭제 | `bulk delete threads` | 여러 스레드 일괄 삭제 |
| 폴더 생성 | `create folder` | 새 폴더 생성 |
| 폴더 삭제 | `delete folder <id>` | 폴더 삭제 |
| 외부 수신자 메일 | `send message` + `confirm_external_recipients: "external recipients"` | 외부 도메인으로 발송 시 추가 확인 |
| 첨부파일 다운로드 저장 | `save download <path>` | 첨부파일을 로컬 경로에 저장 |

### DM

| 동작 | confirm 문자열 | 설명 |
|---|---|---|
| DM 방 생성 | `create dm room` | 1:1 또는 그룹 DM 방 생성 |
| 멤버 추가 | `add dm members <room_id>` | 그룹 방에 멤버 추가 |
| 멤버 제거 | `remove dm member <room_id> <user_id>` | 멤버 제거 또는 방 나가기 |
| 소유권 이전 | `transfer dm owner <room_id> <user_id>` | 그룹 방 소유권 이전 |
| 초대 링크 생성 | `create dm invite <room_id>` | 초대 링크 생성 |
| 초대 수락 | `join dm invite <token>` | 초대 토큰으로 방 참여 |
| 메시지 발송 | `send dm message <room_id>` | 텍스트/Drive 링크 메시지 발송 |
| 첨부파일 발송 | `send dm attachment <room_id>` | 파일 첨부 메시지 발송 |
| 메시지 수정 | `edit dm message <message_id>` | 메시지 텍스트 수정 |
| 메시지 삭제 | `delete dm message <message_id>` | 메시지 삭제 |
| 첨부파일 다운로드 저장 | `save download <path>` | 첨부파일을 로컬 경로에 저장 |

### 드라이브

| 동작 | confirm 문자열 | 설명 |
|---|---|---|
| 파일 다운로드 저장 | `save download <path>` | 파일을 로컬 경로에 저장 |
| 공유 링크 생성 | `share drive <id>` | 공유 링크 생성 |
| 휴지통으로 이동 | `trash drive <id>` | 파일/폴더를 휴지통으로 이동 |
| 영구 삭제 | `delete drive <id>` | 이미 휴지통에 있는 파일/폴더 영구 삭제 |

### 계정

| 동작 | confirm 문자열 | 설명 |
|---|---|---|
| 아바타 업로드 | `upload avatar` | 프로필 사진 업로드 |
| 아바타 삭제 | `delete avatar` | 프로필 사진 삭제 |

### 알림

| 동작 | confirm 문자열 | 설명 |
|---|---|---|
| 푸시 구독 삭제 | `delete push subscription <endpoint>` | Web Push 구독 삭제 |
| 푸시 디바이스 삭제 | `delete push device <device_id>` | 푸시 디바이스 삭제 |

### 확인 문자열 처리 방식

백엔드는 사용자 MCP 키가 `basic` 모드일 때 `X-Gogomail-MCP-Confirm` 요청 헤더에서 확인 문자열을 검증합니다. MCP 서버(이 서버)가 이 헤더를 자동으로 추가합니다. 에이전트는 `confirm` 파라미터만 올바르게 전달하면 됩니다.

---

## 문제 해결

### 인증 오류

**`401 Unauthorized`**

- `GOGOMAIL_USER_MCP_KEY`가 올바르게 설정되었는지 확인합니다.
- 키가 웹메일 설정에서 취소(revoke)되지 않았는지 확인합니다.
- 키의 만료일이 지나지 않았는지 확인합니다.
- 키가 `gmu_` 로 시작하는지 확인합니다 (관리자 키와 혼동하지 않도록 주의).

**`403 Forbidden`**

- 도메인 MCP 정책이 활성화되어 있는지 확인합니다. 관리자에게 `enabled: true` 설정을 요청하세요.
- 사용 중인 툴 스코프가 키에 부여된 스코프에 포함되는지 확인합니다. (예: DM 툴을 쓰려면 `dm` 스코프 필요)
- CIDR 허용 목록이 설정된 경우 현재 IP가 허용 범위 내인지 확인합니다.
- bypass 모드 사용 시 도메인 정책에서 `allow_bypass_mode: true`가 설정되어 있는지 확인합니다.

### 확인 오류

**`confirmation required: pass confirm="..." to proceed`**

`basic` 모드에서 민감한 툴을 호출할 때 `confirm` 파라미터가 없거나 잘못된 경우 발생합니다.

- 오류 메시지에 표시된 정확한 확인 문자열을 `confirm` 파라미터로 전달합니다.
- `<id>`, `<path>` 같은 플레이스홀더는 실제 값으로 대체해야 합니다.
- 확인 문자열은 대소문자를 구분합니다.
- 정책상 허용될 때만 bypass 모드로 전환합니다.

에이전트가 확인 문자열을 자동으로 처리하지 못하는 경우, 사용자가 직접 AI에게 "confirm 파라미터로 `send message`를 전달해서 다시 호출해" 라고 지시할 수 있습니다.

### Generic Bridge 오류

**`path is not allowed`**

`gogomail_api_request`의 generic bridge는 사전에 정의된 허용 경로 목록(manifest)에 있는 라우트만 호출할 수 있습니다. 허용되지 않은 경로를 호출하면 이 오류가 발생합니다.

- 호출하려는 API가 `docs/openapi.yaml`에 문서화되어 있는지 확인합니다.
- 관리/인증/비밀번호/세션/MCP 키 관리 라우트는 의도적으로 차단되어 있습니다.
- 새로운 API 접근이 필요하면 먼저 백엔드 API를 추가하고 문서화한 뒤 manifest를 업데이트해야 합니다.

### 다운로드 저장 실패

**파일이 예상 위치에 저장되지 않음**

- `save_to_path`에 지정한 경로의 상위 디렉터리가 존재하는지 확인합니다.
- 해당 디렉터리에 쓰기 권한이 있는지 확인합니다.
- 이미 동일한 경로에 파일이 있고 `overwrite: false`(기본값)인 경우 `overwrite: true`를 명시합니다.
- `basic` 모드에서 `confirm: "save download <path>"`를 올바르게 전달했는지 확인합니다.

### 드라이브 영구 삭제 실패

**`file is not in trash`**

`gogomail_drive_delete`는 이미 휴지통에 있는 파일에만 적용됩니다. 먼저 `gogomail_drive_trash`로 파일을 휴지통으로 이동한 뒤 `gogomail_drive_delete`를 호출합니다.

### 메일 발송 실패

**일일 발송 한도 초과**

도메인 MCP 정책에서 정한 일일 발송 한도를 초과하면 발송이 거부됩니다. 한도는 도메인 관리자에게 문의합니다.

**외부 수신자 확인 필요**

사용자 MCP 설정에서 외부 수신자 확인이 활성화된 경우, 외부 도메인으로 메일 발송 시 `confirm_external_recipients: "external recipients"` 파라미터도 함께 전달해야 합니다.

### 서버 시작 실패

**`Missing required env var: GOGOMAIL_API_URL`**

환경 변수가 설정되지 않았습니다. MCP 클라이언트 설정의 `env` 섹션을 확인합니다.

**`GOGOMAIL_API_URL must be a valid URL`**

URL 형식이 잘못되었습니다. `http://` 또는 `https://` 스키마를 포함한 올바른 URL인지 확인합니다.

**`GOGOMAIL_MCP_PERMISSION_MODE must be "basic" or "bypass"`**

권한 모드 값이 잘못되었습니다. `basic` 또는 `bypass` 중 하나만 허용됩니다.

### 디버그 팁

서버는 표준 오류(stderr)에 로그를 출력합니다.

```bash
# 서버를 직접 실행해서 오류 확인
GOGOMAIL_API_URL=https://mail.company.com \
GOGOMAIL_USER_MCP_KEY=gmu_xxx \
GOGOMAIL_MCP_PERMISSION_MODE=basic \
node /경로/dist/index.js 2>&1
```

정상 시작 시 `[gogomail-user-mcp] stdio transport ready` 메시지가 표시됩니다.

---

## 관련 문서

- **사용자 MCP 정책 및 설정 상세**: [../../docs/USER_MCP.md](../../docs/USER_MCP.md)
- **OpenAPI 계약 (전체 API 스펙)**: [../../docs/openapi.yaml](../../docs/openapi.yaml)
- **관리 MCP (운영자/관리자용)**: [../gogomail-manage-mcp/README.ko.md](../gogomail-manage-mcp/README.ko.md)
- **백엔드 아키텍처**: [../../docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md)
- **보안 정책**: [../../docs/SECURITY.md](../../docs/SECURITY.md)
- **배포 가이드**: [../../docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md)
