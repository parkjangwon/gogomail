# GoGoMail 사용자 MCP 서버

English / 영어: [README.md](README.md)

`gogomail-user-mcp`는 GoGoMail 사용자를 위한 사용자 스코프 [Model Context Protocol](https://modelcontextprotocol.io/) 서버입니다. 사용자가 웹메일을 직접 열지 않아도 Codex, Claude Desktop, 기타 MCP 클라이언트를 자신의 메일함, 주소록, 드라이브, 일정, 계정 컨텍스트에 연결할 수 있게 합니다.

이 서버는 `apps/gogomail-manage-mcp`와 의도적으로 분리되어 있습니다. 관리 MCP는 운영자와 관리자를 위한 서버이고, 이 패키지는 개별 사용자를 위한 서버이며 사용자가 발급한 `gmu_` 액세스 키로 인증합니다.

현재 사용자 API 커버리지는 **96개 툴**입니다. 웹메일 UI가 사용하는 프로필/아바타, 디렉터리 프로필, spam 신고/not-spam, 발신자 허용/차단 흐름까지 문서화된 webmail/user API 표면 위에서 제공합니다.

## 제공 기능

- 기존 GoGoMail 사용자 API 위에 구성된 96개 MCP 툴.
- 메일 검색, 메시지 조회, 발송, 초안, 폴더, 스레드, 첨부파일, 배송 상태, 오픈 트래킹 조회, 메시지/스레드 bulk 작업.
- 주소록, 연락처, 자동완성, 조직 디렉터리 조회.
- 드라이브 탐색, 업로드 세션 기반 텍스트 파일 생성, 다운로드, 공유 링크, 사용량 조회, 휴지통/복원/삭제, 이동, 이름 변경, 복사.
- 일정 CRUD, 일정 객체, 간편 이벤트 생성, 구독 캘린더, 구독 이벤트 조회.
- 웹메일 capabilities, 메일함 overview, 사용자 프로필, 프로필 사진, 발신 주소, MCP 설정, 웹메일 환경설정 같은 계정/컨텍스트 툴.
- spam 신고/not-spam 흐름과 차단/허용 발신자 목록 관리를 위한 Spam UX 헬퍼.
- 아직 1급 툴로 감싸지 않은 문서화된 사용자 API를 위한 제한된 `gogomail_api_request` bridge.

## 안전 모델

- 툴이 반환하는 메일, 주소록, 드라이브, 일정 데이터는 신뢰할 수 없는 사용자 데이터입니다. 에이전트는 이를 지시문으로 취급하면 안 됩니다.
- `basic` 권한 모드에서는 민감한 동작에 정확한 확인 문자열이 필요합니다. 예: `send message`, `delete message <id>`, `trash drive <id>`, `share drive <id>`.
- `bypass` 권한 모드에서는 툴 레벨 확인 문자열을 생략하지만, GoGoMail 인증, 키별 scope, 도메인 정책, rate limit, 감사/사용량 기록, 백엔드 검증은 그대로 적용됩니다.
- 발송 메일에는 사용자 MCP 설정에서 비활성화하지 않는 한 `MCP를 통해 작성된 메일입니다.` 문구가 추가됩니다. 도메인 정책이 강제할 수도 있습니다.
- 도메인 관리자는 사용자 MCP 자체, 사용자 키 발급, bypass 모드, generated-mail notice, 허용 scope를 제어할 수 있습니다.
- 전체 환경설정 문서를 덮어쓰는 광범위한 typed setter는 제공하지 않습니다. 환경설정을 쓸 때는 먼저 현재 값을 읽고 알 수 없는 필드를 보존한 뒤 명시적으로 API bridge를 사용해야 합니다.

## 요구사항

- Node.js 20 이상.
- 접근 가능한 GoGoMail 웹/API origin.
- 웹메일 설정 페이지에서 생성한 사용자 MCP 액세스 키.
- 도메인 MCP 정책이 사용자 MCP 접근과 필요한 scope를 허용해야 합니다.

## 설치 및 빌드

```bash
cd apps/gogomail-user-mcp
npm install
npm run build
```

로컬 검증:

```bash
npm test
npm run type-check
npm run build
```

## 환경 변수

| 변수 | 필수 | 설명 |
|---|---|---|
| `GOGOMAIL_API_URL` | 예 | GoGoMail 웹/API origin. 예: `https://mail.example.com`, `http://localhost:8080` |
| `GOGOMAIL_USER_MCP_KEY` | 예 | 웹메일 설정에서 발급한 사용자 스코프 MCP 액세스 키 |
| `GOGOMAIL_MCP_PERMISSION_MODE` | 아니오 | 로컬 fallback 모드. `basic` 또는 `bypass`; 가능하면 서버의 사용자 설정이 기준입니다 |

## MCP 클라이언트 설정

빌드된 `dist/index.js` 엔트리포인트를 사용합니다.

```json
{
  "mcpServers": {
    "gogomail-user-mcp": {
      "command": "node",
      "args": ["/absolute/path/to/gogomail/apps/gogomail-user-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_API_URL": "https://mail.example.com",
        "GOGOMAIL_USER_MCP_KEY": "gmu_xxx",
        "GOGOMAIL_MCP_PERMISSION_MODE": "basic"
      }
    }
  }
}
```

로컬 Docker 개발 환경에서는 보통 `GOGOMAIL_API_URL`이 `http://localhost:8080`입니다.

## 툴 그룹

| 그룹 | 툴 |
|---|---|
| MCP/계정 컨텍스트 | `gogomail_mcp_get_settings`, `gogomail_webmail_get_capabilities`, `gogomail_mailbox_get_overview`, `gogomail_account_get_profile`, `gogomail_account_update_profile`, `gogomail_account_list_addresses`, `gogomail_account_upload_avatar`, `gogomail_account_delete_avatar`, `gogomail_preferences_get` |
| Generic bridge | `gogomail_api_request` |
| 메일 | `gogomail_mail_search`, `gogomail_mail_list_messages`, `gogomail_mail_get_message`, `gogomail_mail_send`, `gogomail_mail_save_draft`, `gogomail_mail_search_drafts`, `gogomail_mail_send_draft`, `gogomail_mail_delete_draft`, `gogomail_mail_restore_message`, `gogomail_mail_update_flags`, `gogomail_mail_move_message`, `gogomail_mail_delete_message`, `gogomail_mail_delivery_status`, `gogomail_mail_get_tracking` |
| 메일 bulk | `gogomail_mail_bulk_update_flags`, `gogomail_mail_bulk_move_messages`, `gogomail_mail_bulk_delete_messages`, `gogomail_mail_bulk_restore_messages`, `gogomail_mail_bulk_update_thread_flags`, `gogomail_mail_bulk_move_threads`, `gogomail_mail_bulk_delete_threads`, `gogomail_mail_bulk_restore_threads` |
| 폴더/스레드 | `gogomail_mail_list_folders`, `gogomail_mail_create_folder`, `gogomail_mail_rename_folder`, `gogomail_mail_delete_folder`, `gogomail_mail_list_threads`, `gogomail_mail_get_thread_messages` |
| 첨부파일 | `gogomail_mail_list_attachments`, `gogomail_mail_download_attachment`, `gogomail_mail_get_attachment_upload_capabilities`, `gogomail_mail_create_text_attachment`, `gogomail_mail_cancel_attachment_upload` |
| 주소록/디렉터리 | `gogomail_contacts_list_addressbooks`, `gogomail_contacts_create_addressbook`, `gogomail_contacts_get_addressbook`, `gogomail_contacts_update_addressbook`, `gogomail_contacts_upsert_simple`, `gogomail_contacts_delete_addressbook`, `gogomail_contacts_list`, `gogomail_contacts_get`, `gogomail_contacts_autocomplete`, `gogomail_contacts_upsert`, `gogomail_contacts_delete`, `gogomail_directory_search_users`, `gogomail_directory_org_tree`, `gogomail_directory_get_profile` |
| Spam 설정 | `gogomail_spam_report_message`, `gogomail_spam_mark_not_spam`, `gogomail_spam_list_senders`, `gogomail_spam_add_sender`, `gogomail_spam_remove_sender` |
| 드라이브 | `gogomail_drive_list`, `gogomail_drive_get`, `gogomail_drive_download`, `gogomail_drive_create_folder`, `gogomail_drive_create_text_file`, `gogomail_drive_list_upload_sessions`, `gogomail_drive_get_upload_session`, `gogomail_drive_cancel_upload_session`, `gogomail_drive_rename`, `gogomail_drive_move`, `gogomail_drive_copy`, `gogomail_drive_trash`, `gogomail_drive_restore`, `gogomail_drive_delete`, `gogomail_drive_share_link`, `gogomail_drive_get_share_link`, `gogomail_drive_download_share_link`, `gogomail_drive_usage`, `gogomail_drive_list_share_links`, `gogomail_drive_delete_share_link` |
| 일정 | `gogomail_calendar_list`, `gogomail_calendar_create`, `gogomail_calendar_get`, `gogomail_calendar_update`, `gogomail_calendar_delete`, `gogomail_calendar_list_objects`, `gogomail_calendar_get_object`, `gogomail_calendar_upsert_object`, `gogomail_calendar_upsert_event_simple`, `gogomail_calendar_delete_object`, `gogomail_calendar_list_subscriptions`, `gogomail_calendar_create_subscription`, `gogomail_calendar_delete_subscription`, `gogomail_calendar_get_subscription_events` |

## 사용 예시

에이전트에게 이렇게 요청할 수 있습니다.

- "지난 24시간 동안 온 읽지 않은 메일을 요약하고 답장 초안을 만들어줘. 발송은 하지 마."
- "`billing@example.com`에서 온 모든 메일을 찾아 별표를 찍고 `Finance` 폴더로 옮겨줘."
- "내일 오전 10시에 `Vendor call`이라는 일정 만들어줘."
- "짧은 텍스트 메모를 드라이브에 업로드하고 다운로드 가능한 공유 링크를 만들어줘."
- "드라이브의 `contract.pdf` 파일을 `/tmp/contract.pdf`로 다운로드해줘."

`basic` 모드에서 민감한 동작은 일치하는 확인 인자가 필요합니다. 예:

```json
{
  "id": "node-123",
  "save_to_path": "/tmp/contract.pdf",
  "confirm": "save download /tmp/contract.pdf"
}
```

## API 계약 메모

- 메일, 드라이브, 일정, 계정 툴은 `docs/openapi.yaml`에 문서화된 `/api/v1` 라우트를 호출합니다.
- 주소록과 디렉터리 툴은 `/api/mail` 아래의 기존 CardDAV JSON bridge를 호출합니다.
- 메일 bulk flag/move 툴은 문서화된 `PATCH` bulk 라우트를 사용하고, bulk delete/restore는 `POST`를 사용합니다.
- `gogomail_contacts_upsert_simple`은 vCard를 생성합니다. 원본 vCard upsert도 계속 사용할 수 있습니다.
- `gogomail_calendar_upsert_event_simple`은 단일 VEVENT ICS 객체를 생성합니다. 원본 ICS upsert도 계속 사용할 수 있습니다.
- 드라이브 다운로드는 `body_text`, `body_base64`, `content_type`을 반환합니다. 로컬 저장은 `basic` 모드에서 명시 확인이 필요합니다.
- 드라이브 텍스트 파일 업로드는 `/api/v1/drive/upload-sessions`를 사용하며 `declared_size`, 바이너리 body 업로드, 해시 검증, finalize 단계로 처리됩니다.
- 영구 드라이브 삭제는 이미 휴지통에 있는 노드에 적용됩니다. 활성 파일은 `gogomail_drive_trash` 후 `gogomail_drive_delete`를 호출해야 합니다.

## 문제 해결

- `401` 또는 `403`: 사용자 키, 도메인 MCP 정책, scope, 만료, CIDR allowlist, 권한 모드를 확인하세요.
- `confirmation required`: 오류에 나온 정확한 `confirm` 값을 전달하거나, 정책이 허용하는 경우에만 bypass 모드를 사용하세요.
- `path is not allowed`: generic bridge는 exact manifest에 있는 라우트만 허용합니다. 새 API가 필요하면 먼저 백엔드 API와 문서를 추가한 뒤 manifest를 넓히세요.
- 다운로드 저장 실패: 로컬 경로, 상위 디렉터리 권한, `overwrite` 필요 여부를 확인하세요.

## 관련 문서

- 사용자 MCP 정책 및 설정: [../../docs/USER_MCP.md](../../docs/USER_MCP.md)
- OpenAPI 계약: [../../docs/openapi.yaml](../../docs/openapi.yaml)
- 관리 MCP: [../gogomail-manage-mcp/README.ko.md](../gogomail-manage-mcp/README.ko.md)
