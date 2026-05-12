# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ TASK-175: CalDAV sync-collection 동시성/락 경합 고도화 및 캐시 힌트

### 배경

쓰기 요청(객체 upsert/delete 및 calendar 속성 변경)에서 `FOR UPDATE` 잠금과 선행 `SELECT`가 동시 요청 시 병목을 만들고 있었습니다.
또한 UID 중복 선체크는 2회 조회 비용을 추가하고, sync marker 보장 쿼리도 다중 왕복을 유발했습니다.
현재 목표(베타 운영 대비 RFC-정합성 유지)는 유지하면서 동시성 대비 처리량을 먼저 끌어올리는 것이 핵심이었습니다.

### 구현 대상

- `internal/caldavgw/repository.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] 객체 upsert/delete/calendar property 업데이트에서 비필수한 `FOR UPDATE` 잠금을 제거해 동시성 경쟁 구간을 축소한다.
- [x] UID 중복 선체크(`ensureCalendarObjectUIDAvailable`) 경로를 제거해 업서트 1회 쿼리 실패 처리로 수렴한다.
- [x] sync marker 초기화 경로를 CTE 단일 쿼리로 정리해 marker 조회+삽입 비용을 축소한다.
- [x] 에러 시나리오(미활성 캘린더/미발견) 처리 및 기존 RFC 동작(412/412/428 등)은 유지한다.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-176: CalDAV 동시성-쓰기 병목 계측 및 적응형 지연 제어

## ✅ TASK-176: CalDAV 동시성-쓰기 병목 계측 및 적응형 지연 제어

### 배경

쓰기 경로(객체 upsert/delete)는 동시 요청량이 높을 때 `serialization_failure`, deadlock, lock wait가 반복 발생해
재시도 없이 즉시 실패하거나 롤백되는 구간이 존재했다.
또한 객체 존재/변경조건 검사에서 본문(ICS)을 함께 조회하는 경로가 실제로는 메타데이터만 필요한 상황에서도 I/O 부하를 만들었다.

다음 단계로는 쓰기 경로에서 조건부 판정 비용을 낮추고, 트랜잭션 경쟁 구간에서 재시도와 회복 대기(backoff)로
동일 작업의 성공률을 높여 처리량을 끌어올릴 수 있다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/repository.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `servePutObject`, `serveDeleteObject`의 객체 조건부 판정에서 메타데이터 조회 경로를 우선 사용한다.
- [x] 객체 존재/ETag 판정 실패 시 전체 ICS 본문을 읽는 경로를 제거한다.
- [x] `repository`의 `UpsertObject`, `DeleteObject`를 트랜잭션 재시도 래퍼로 감싸
  serialization/deadlock/lock contention 에러를 백오프로 복구한다.
- [x] `calendar` 컬렉션 변경 경로인 `DeleteCalendar`, `UpdateCalendarProperties`도
  동일한 재시도 래퍼로 감싸 동시성 경합 시 복구율을 높인다.
- [x] `serveGetObject`와 `propfind`의 달력 객체 분기를 메타데이터 우선 조회로 분리해
  `calendar-data` 미요청/조건부 판정 상황에서 ICS 본문 로딩 횟수를 줄인다.
- [x] RFC 4791/4918 조건부 규약(`If-Match`, `If-None-Match`, `If-Unmodified-Since`, 412 동작)을 변경하지 않는다.
- [x] 기본 성능 회귀 리스크를 만들지 않는 방향으로 코드 수정 범위를 종료한다.
- [x] 개발 문서를 함께 갱신한다.

### 다음 태스크

TASK-177: CalDAV 쓰기/동기화 경로 장기 지연 정합성 정교화

## ✅ TASK-177: CalDAV 조건부 PUT 단일문 쿼리 정합성 강화

### 배경

`If-Match` 경합에서 기존 경로는 선행 조회 후 `UpsertObject`가 별도 SQL을 수행해
짧은 창의 TOCTOU 경합이 남아 있었습니다.
또한 재시도 구간에서 레이스 윈도우가 발생하면 `upsert`가 `no rows`로 끝나
412 처리 규칙으로 끌어올리기 어렵던 지점이 남았습니다.

### 구현 대상

- `internal/caldavgw/repository.go`
- `internal/caldavgw/handler.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- `If-Match` 조건부 upsert를 `INSERT ... SELECT + ON CONFLICT DO UPDATE` 단일 문으로 수렴한다.
- 조건 불일치(존재/ETag 불일치) 시 `QueryRowContext`가 0행인 경우를 안전하게 해석해
  조건부 실패로 매핑한다.
- `servePutObject`에서 조건부 실패 경로가 412로 유지되도록 보강한다.
- 문서 상태가 최신 반영되어 다음 작업 전개의 기준점이 유지된다.

### 다음 태스크

TASK-178: CalDAV 쓰기 경로 종합 계측 + 자동 성능 임계 대응

## ✅ TASK-173: CalDAV `calendar-query` 컴포넌트 필터 경로 고속화

### 배경

`calendar-query`는 클라이언트 동기화에서 반복 호출되는 핫 패스다. 기존에는 컴포넌트 필터(`VEVENT`, `VTODO`)가 주로 핸들러에서 후처리되었고,
목록 정렬 후 필터링이 결합되어 있었기 때문에 컴포넌트가 많은 환경에서 쿼리/스캔 비용이 높았다.

이번 라운드는 컴포넌트 필터를 가능하면 스토어에서 선적용하고, 정렬·제한 쿼리 계획을 강화해 RFC 4791 동작을 유지하면서 처리량을 끌어올리는 것을 목표로 한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/repository.go`
- `migrations/0095_caldav_calendar_object_performance_index.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `calendar-query`에서 컴포넌트가 지정된 경우 스토어 기반 컴포넌트 제한 경로가 동작한다.
- [x] 컴포넌트 지정 시 핸들러 후처리 비교 비용이 줄어든다.
- [x] `calendar-query` 관련 인덱스가 컴포넌트/정렬/제한 패턴을 타게 된다.
- [x] RFC 4791 동작(404 규칙, truncation, 시간대/시간 구간 동작)을 변경 없이 유지한다.
- [x] `docs/CURRENT_STATUS.md` 업데이트 완료.

### 다음 태스크

TASK-175: CalDAV sync-collection 동시성/락 경합 고도화 및 캐시 힌트

## ✅ TASK-174: CalDAV sync-collection 증분 응답 단일 조회 고도화

**STATUS: COMPLETE**

### 배경

`sync-collection`은 토큰 기반 변경 집합을 읽은 뒤 객체명을 기준으로 객체를 재조회해 응답을 조립했다.
동기화 변경량이 많아지면 변경 집합 조회와 객체 조회 경로가 분리되면서 `sync` 응답 지연과 동시 요청 시 DB 왕복이 비례적으로 늘어났다.

이번 라운드는 `sync-collection` 증분 경로를 `sync_changes`와 객체 데이터의 조인 경로 한 번으로 통합해
요청당 쿼리 횟수를 줄이고 `calendar-data` 요청 여부에 따라 ICS를 선별적으로 로드하도록 만들었다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/types.go`
- `internal/caldavgw/repository.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `sync-collection` 증분 응답에서 변경 이벤트와 객체 메타데이터를 `ListCalendarChangesWithObjectsSince` 단일 경로로 조회한다.
- [x] `object-deleted`는 기존 `404` 규칙을 유지한다.
- [x] `calendar-data` 요청 시 ICS가 포함되고, 비요청 시에는 ICS를 생략해 오버헤드를 줄인다.
- [x] 기존 RFC 4918/4791 동기화 token 유효성, truncation 동작을 변경 없이 유지한다.
- [x] `docs/CURRENT_STATUS.md` 업데이트 완료.

### 다음 태스크

TASK-175: CalDAV sync-collection 동시성/락 경합 고도화 및 캐시 힌트

---

## ✅ TASK-172: CalDAV 성능 급상승 라운드 — 배치 질의 분할 및 sync-change 커버링 경로 고도화

**STATUS: COMPLETE**

### 배경

TASK-171에서 크로스 캘린더 배치 조회가 적용된 뒤에도 대규모 동기화에서 2차 병목은
요청당 바운드 객체 목록 크기에 비례한 단일 쿼리 파라미터 폭주, 반복 쿼리 생성 오버헤드였다.
또한 sync-change 조회에서는 동시성 높은 읽기에서 인덱스 커버리지 한계가 체감 성능 저하를 유발하고 있었다.

이번 라운드는 배치 조회를 청크 분할해 안정적으로 처리하고, sync-change 조회 경로에 커버링 인덱스를 추가해
DB 스캔 및 힙 접근을 줄였다. RFC 4791 동작은 변경하지 않는다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/repository.go`
- `migrations/0093_caldav_calendar_object_lookup_index_v2.sql`
- `migrations/0094_caldav_calendar_sync_changes_covering_index.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 다중 캘린더 객체명 조회를 청크(최대 256개) 단위로 분할해 동적 SQL 크기 상한을 안정화한다.
- [x] `ListCalendarObjectsByNameGroups`와 `ListCalendarObjectsByNames`를 공통 `VALUES` 기반 단일 경로로 수렴한다.
- [x] `calendar-query` 컴포넌트 필터 정규화를 루프 외부로 이동해 반복 정규화 비용을 줄인다.
- [x] sync-change 읽기 경로에 `INCLUDE` 절을 갖는 커버링 인덱스를 추가해 인덱스 스캔 효율을 높인다.
- [x] 기존 WebDAV/CalDAV RFC 동작과 `object-deleted` 404 응답 규칙은 변경 없이 유지한다.
- [x] `docs/CURRENT_STATUS.md` 업데이트 완료.
- [x] `docs/ACTIVE_TASK.md` 현재 항목으로 정리 완료.

### 다음 태스크

TASK-173: CalDAV 동기화 경로 캐시-측정 고도화 및 반복 트랜잭션 최소화

---

## ✅ TASK-170: CalDAV 성능 고도화 — 배치 조회 기반 sync/calendar-multiget 최적화

**STATUS: COMPLETE**

### 배경

대량 캘린더 변경 동기화에서 `calendar-multiget`와 `sync-collection`의 객체 조회가 현재는 변경된 객체 수만큼 N+1 조회 패턴으로 수행되어,
동일 사용자의 동기화 변경량이 늘어날수록 DB 부하가 급격히 증가한다.

표준 동작(객체 상태/권보장/삭제 응답, 에러 처리)을 그대로 유지하면서 다건 객체명을 한 번에 조회하도록 정합성을 보존한 고성능 경로로 교체했다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/repository.go`
- `migrations/0091_caldav_calendar_object_lookup_index.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `calendar-multiget` 응답에서 요청 href 범위 내 객체를 배치 조회로 처리한다.
- [x] `sync-collection` 변경 이벤트 객체 조회에서 동일 캘린더/객체명을 중복 제거해 배치로 조회한다.
- [x] 삭제된 객체(`object-deleted`)는 `404` status 응답 규칙을 유지한다.
- [x] 객체명 일괄 조회용 DB 인덱스를 추가해 동기화·멀티겟 경로의 실행 계획이 개선되었다.
- [x] 기존 WebDAV/CalDAV RFC 동작은 변경되지 않는다.
- [x] `go test ./...` 기준 회귀 위험이 낮은 범위에서 통과한다.
- [x] 개발 문서가 최신 상태로 갱신된다.

### 다음 태스크

TASK-171: CalDAV 배치 조회 캐시/계측 성능 검증 및 튜닝

---

## ✅ TASK-168: 사용자 웹메일 베타 안정화 — 드라이브 DnD 이동 및 폴더 업로드 보강

**STATUS: COMPLETE**

### 배경

드라이브 사용에서 내부 노드 이동(드래그 앤 드롭)과 브라우저 드롭 업로드가 모두 동작해야
베타 사용자 플로우가 완성된다.

현재는 드라이브 업로드가 파일 목록 기반으로 동작해 폴더 업로드와 폴더 간 이동이 제한되어 있다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/DriveView.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 드라이브 폴더 노드 카드로 내부 파일/폴더 노드 드래그 시 이동이 가능하다.
- [x] 폴더 카드 드롭 시 이동 대상이 하이라이트된다.
- [x] 드라이브 화면 내부 빈 영역(현재 폴더 뷰)에 파일 드롭 업로드가 동작한다.
- [x] 폴더(Directory) 드롭 시 하위 파일 구조가 보존되어 업로드된다.
- [x] 폴더 업로드 중 생성된 중간 폴더가 중복 생성되지 않는다.
- [x] 기존 디자인 톤/드롭존 문구/작업 아이콘은 유지한다.
- [x] 개발 문서가 최신 상태로 갱신된다.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-169: 사용자 웹메일 베타 안정화 — 파일 아이콘/드래그 UX 폴리싱

---

## ✅ TASK-167: 사용자 웹메일 베타 안정화 — 주소록 선택기 연락처 표시 정합성

**STATUS: COMPLETE**

### 배경

메일쓰기 수신자 선택기에서 주소록 탭은 주소록 그룹 발송 토큰만 보이고 실제 연락처가 보이지 않았다.
연락처 앱에서도 시드 연락처가 이름 대신 `.vcf` 객체명으로 보였기 때문에, 사용자 입장에서는 시드 데이터가 깨진 것처럼 보였다.

실제 원인은 프론트엔드 vCard 파서가 표준 `FN:value`, `EMAIL:value` 속성 형태를 제대로 파싱하지 못하는 데 있었다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/OrgPickerModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] base64 인코딩된 vCard 본문에서 표준 `FN:value`, `EMAIL:value` 속성을 읽는다.
- [x] `EMAIL;TYPE=WORK:value`처럼 파라미터가 있는 vCard 속성도 유지 지원한다.
- [x] 메일쓰기 주소록 탭이 주소록 그룹 발송 행과 실제 연락처 행을 함께 표시한다.
- [x] 주소록 그룹 발송 행이 있는 상태에서 잘못된 `연락처 없음` 빈 상태가 먼저 보이지 않는다.
- [x] 기존 디자인 톤은 변경하지 않는다.
- [x] 개발 문서를 함께 갱신한다.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-169: 사용자 웹메일 베타 안정화 — 주소록/조직도 수신자 선택 브라우저 회귀 점검

---

## ✅ TASK-098: 사용자 웹메일 베타 안정화 — API base-path 정합성 + 베타 시드 데이터

**STATUS: COMPLETE**

### 배경

목표는 사용자 웹메일 베타서비스다. 관리자 콘솔 이후 사용자 웹메일 프론트엔드가 진행 중이며, 프론트엔드와 백엔드 기능이 실제로 연결되어 동작해야 한다.

현재 가장 먼저 해소해야 할 베타 블로커는 웹메일 API base-path 드리프트다.

- 웹메일 브라우저 코드는 `/api/mail/...`을 호출한다.
- 백엔드 CardDAV/Directory 라우트는 `/api/mail/...`로 등록되어 있다.
- 기존 Mail API와 OpenAPI 문서에는 `/api/v1/...` 라우트가 여전히 존재한다.
- 따라서 프록시는 기능 영역별로 의도된 backend base path를 명확히 선택해야 한다.

### 구현 대상

- `apps/webmail/src/app/api/mail/[...path]/route.ts`
- `apps/webmail/src/components/OrgPickerModal.tsx`
- `scripts/seed_dev_beta.sh`
- `scripts/seed_dev_data.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `/api/mail/addressbooks`, `/api/mail/contacts`, `/api/mail/directory` 프록시가 backend `/api/mail/...`로 전달된다.
- [x] 기존 Mail API 요청은 backend `/api/v1/...` 전달을 유지한다.
- [x] 조직도 피커가 사용자 소속 조직을 기본 선택하고 부모 체인을 확장한다.
- [x] Docker PostgreSQL 컨테이너에 풍부한 어드민/웹메일 베타 데이터를 넣는 실행 스크립트가 있다.
- [x] 시드 데이터는 조직도, 주소록, 사용자, 메일 목록 테스트에 충분하다.
- [x] 디자인 토큰/레이아웃/시각 톤은 변경하지 않는다.
- [x] 표준 경로 드리프트를 문서화한다.
- [x] 관련 검증 통과 후 기능 단위 커밋.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-099: 사용자 웹메일 베타 안정화 — 핵심 메일 화면/작성 흐름 점검 및 보강

---

## ✅ TASK-099: 사용자 웹메일 베타 안정화 — 핵심 메일 화면/작성 흐름 점검

**STATUS: COMPLETE**

### 배경

TASK-098에서 웹메일 API base-path 정합성과 베타 시드 실행 경로를 안정화했다.
다음 베타 목표는 사용자 웹메일의 기본 사용 흐름이 실제 데이터로 자연스럽게 동작하도록 만드는 것이다.

디자인은 현재 상태를 유지한다. 시각 톤, 레이아웃, 디자인 토큰을 갑자기 바꾸지 않는다.

### 구현 대상

- `apps/webmail/src/app/mail/page.tsx`
- `apps/webmail/src/components/ComposeModal.tsx`
- `apps/webmail/src/components/MessageList.tsx`
- `apps/webmail/src/components/ReadingPane.tsx`
- 관련 API helper 또는 문서

### 완료 조건

- [x] 베타 seed 데이터 기준으로 로그인 후 메일 목록/읽기/작성 기본 흐름을 점검한다.
- [x] 프론트엔드-백엔드 API 계약 불일치가 있으면 표준/기존 계약을 우선해 수정한다.
- [x] 실제 폴더 선택 전 또는 가상 폴더 상태에서 잘못된 일반 메시지 목록 API 호출을 방지한다.
- [x] 디자인 톤은 유지한다.
- [x] 변경 내용과 검증 결과를 `docs/CURRENT_STATUS.md`에 기록한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-100: 사용자 웹메일 베타 안정화 — 작성/전송/초안 흐름 계약 점검

---

## ✅ TASK-100: 사용자 웹메일 베타 안정화 — 작성/전송/초안 흐름 계약 점검

**STATUS: COMPLETE**

### 배경

메일 작성은 사용자 웹메일 베타의 핵심 경로다. UI는 reply-all 같은 사용자 편의 동작을 제공하지만, 백엔드 compose 계약은 표준적으로 `new`, `reply`, `forward`만 허용한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `reply_all` UI 동작이 백엔드 계약에서 허용되는 `reply` intent로 정규화된다.
- [x] 초안 자동저장/수동저장/닫기 전 저장/전송 payload가 같은 intent 정규화 경로를 사용한다.
- [x] 초안 저장 수신자 필드는 전송과 같은 주소 파서로 처리한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-101: 사용자 웹메일 베타 안정화 — 첨부파일/드라이브 첨부 계약 점검

---

## ✅ TASK-101: 사용자 웹메일 베타 안정화 — 첨부파일/드라이브 첨부 계약 점검

**STATUS: COMPLETE**

### 배경

사용자 웹메일 베타에서 첨부파일은 작성/초안/전송 흐름의 핵심 기능이다.
백엔드 초안 계약은 `attachment_ids`를 지원하므로, 프론트엔드 초안 저장 경로도 전송 경로와 동일하게 준비된 첨부 ID를 보존해야 한다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 초안 저장 payload 타입이 `attachment_ids`를 지원한다.
- [x] 자동저장/수동저장/닫기 전 저장에서 업로드 완료된 첨부 ID가 초안에 포함된다.
- [x] 업로드 중이거나 실패한 첨부는 초안 `attachment_ids`에 포함하지 않는다.
- [x] 전송 경로의 기존 첨부 동작을 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-102: 사용자 웹메일 베타 안정화 — 첨부파일 업로드/전송 실패 상태 UX 점검

---

## ✅ TASK-102: 사용자 웹메일 베타 안정화 — 첨부파일 업로드/전송 실패 상태 UX 점검

**STATUS: COMPLETE**

### 배경

사용자 웹메일 베타에서는 첨부파일이 업로드 중이거나 실패한 상태에서 전송이 진행되면 사용자는 첨부가 포함됐다고 기대하지만 실제 payload에는 누락될 수 있다.
전송 경로는 조용히 실패 상태를 무시하지 말고, 완료된 첨부만 전송한다는 계약을 명확히 지켜야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 업로드 중인 첨부가 있으면 메일 전송을 차단하고 사용자에게 명확히 안내한다.
- [x] 업로드 실패 첨부가 있으면 전송을 차단하고 제거/재시도 필요 상태를 안내한다.
- [x] 완료된 첨부만 전송 payload에 포함하는 기존 계약을 유지한다.
- [x] 초안 저장 동작은 기존처럼 완료 첨부만 보존한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-103: 사용자 웹메일 베타 안정화 — 첨부파일 실패 복구/재시도 흐름 점검

---

## ✅ TASK-103: 사용자 웹메일 베타 안정화 — 첨부파일 실패 복구/재시도 흐름 점검

**STATUS: COMPLETE**

### 배경

TASK-102에서 업로드 중/실패 첨부가 있는 상태의 전송은 차단했다.
다음 기준은 사용자가 실패한 로컬 파일 첨부를 제거만 하는 것이 아니라 같은 파일로 다시 업로드할 수 있어야 한다는 것이다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 로컬 파일 업로드 실패 상태가 원본 `File`을 보존한다.
- [x] 실패 첨부 칩에서 재시도 액션을 제공한다.
- [x] 재시도 중에는 업로드 중 상태로 표시된다.
- [x] 재시도 성공 시 서버 첨부 ID로 교체되어 기존 draft/send 계약에 합류한다.
- [x] 재시도 실패 시 실패 상태를 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-104: 사용자 웹메일 베타 안정화 — 예약/되돌리기 전송 첨부 상태 계약 점검

---

## ✅ TASK-104: 사용자 웹메일 베타 안정화 — 예약/되돌리기 전송 첨부 상태 계약 점검

**STATUS: COMPLETE**

### 배경

되돌리기 전송 카운트다운은 실제 전송 payload를 `pendingMsgRef`에 보관한다.
카운트다운 중 첨부 상태가 바뀌면 사용자는 최신 첨부 상태가 반영된다고 기대하지만, 보관된 payload는 이전 `attachment_ids`를 전송할 수 있다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 되돌리기 카운트다운 중 첨부 업로드/실패/제거로 상태가 바뀌면 pending send를 취소한다.
- [x] 취소 시 사용자가 다시 확인 후 전송해야 함을 명확히 안내한다.
- [x] 카운트다운 시작 직후 정상 상태에서는 불필요하게 취소하지 않는다.
- [x] 예약 전송/즉시 전송의 기존 준비 상태 가드는 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-105: 사용자 웹메일 베타 안정화 — 작성창 전송 중 중복 제출 방지

---

## ✅ TASK-105: 사용자 웹메일 베타 안정화 — 작성창 전송 중 중복 제출 방지

**STATUS: COMPLETE**

### 배경

전송 버튼은 UI에서 비활성화되지만, 단축키나 드롭다운 액션 등 여러 진입점이 `handleSend`를 호출할 수 있다.
베타 품질 기준에서는 버튼 상태가 아니라 전송 함수 자체가 중복 제출을 방어해야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 이미 전송 중이면 추가 전송 요청을 무시한다.
- [x] 이미 전송 완료된 작성창이면 추가 전송 요청을 무시한다.
- [x] 되돌리기 카운트다운 대기 중이면 추가 전송 요청을 차단하고 안내한다.
- [x] 기존 즉시/예약/되돌리기 전송 동작은 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-106: 사용자 웹메일 베타 안정화 — 주소 입력 검증/오류 안내 강화

---

## ✅ TASK-106: 사용자 웹메일 베타 안정화 — 주소 입력 검증/오류 안내 강화

**STATUS: COMPLETE**

### 배경

작성창은 수신자 문자열을 파싱해 백엔드로 전달한다.
베타 품질에서는 명백히 잘못된 주소를 백엔드까지 보낸 뒤 실패시키는 대신, 전송 직전에 프론트엔드에서 명확히 안내해야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] To/Cc/Bcc 주소가 전송 직전에 구문 검증된다.
- [x] 잘못된 주소가 있으면 전송을 차단하고 문제가 되는 주소를 안내한다.
- [x] 표시 이름이 있는 `Name <addr@example.com>` 형식은 계속 지원한다.
- [x] 초안 저장의 유연성은 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-107: 사용자 웹메일 베타 안정화 — 예약 전송 시간 검증 강화

---

## ✅ TASK-107: 사용자 웹메일 베타 안정화 — 예약 전송 시간 검증 강화

**STATUS: COMPLETE**

### 배경

예약 전송 입력은 UI에서 최소 시간을 제한하지만, 실제 전송 함수는 문자열 상태를 그대로 ISO 시간으로 변환한다.
베타 품질에서는 UI 제약만 믿지 말고, 전송 직전에 유효한 미래 시간인지 다시 검증해야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 예약 전송 시간이 잘못된 날짜면 전송을 차단한다.
- [x] 예약 전송 시간이 현재 시각 이후가 아니면 전송을 차단한다.
- [x] 정상 예약 전송 payload의 `scheduled_at` 생성은 유지한다.
- [x] 즉시/되돌리기 전송 동작은 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-108: 사용자 웹메일 베타 안정화 — 예약 전송 UI 상태 정리

---

## ✅ TASK-108: 사용자 웹메일 베타 안정화 — 예약 전송 UI 상태 정리

**STATUS: COMPLETE**

### 배경

예약 전송은 드롭다운 preset과 사용자 지정 날짜 입력이 함께 존재한다.
사용자 지정 날짜를 열었는데 값이 비어 있거나, 예약 값을 해제할 명확한 액션이 없으면 전송 버튼 상태와 실제 payload 의도가 어긋날 수 있다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 사용자 지정 예약 날짜를 열 때 기본 미래 시간이 채워진다.
- [x] 예약 전송 상태를 명확히 해제할 수 있다.
- [x] 예약 해제 시 `scheduledAt`과 사용자 지정 입력 표시 상태가 함께 정리된다.
- [x] 기존 preset 예약 전송 동작은 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-109: 사용자 웹메일 베타 안정화 — 전송 후 초안 정리 계약 점검

---

## ✅ TASK-109: 사용자 웹메일 베타 안정화 — 전송 후 초안 정리 계약 점검

**STATUS: COMPLETE**

### 배경

작성창 자동저장은 백엔드 draft를 생성한다.
사용자가 같은 작성창에서 전송을 완료하면, 성공한 메시지의 draft가 초안함에 계속 남지 않도록 프론트엔드도 백엔드 삭제 계약을 호출해야 한다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 웹메일 API helper가 `DELETE /drafts/{id}` 계약을 제공한다.
- [x] 전송 성공 후 현재 작성창 draft가 있으면 삭제를 시도한다.
- [x] draft 삭제 실패는 이미 성공한 전송을 실패로 바꾸지 않는다.
- [x] 즉시/예약/되돌리기 전송 성공 경로 모두 동일하게 정리한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-110: 사용자 웹메일 베타 안정화 — draft send 전용 백엔드 계약 활용 검토

---

## ✅ TASK-110: 사용자 웹메일 베타 안정화 — draft send 전용 백엔드 계약 활용 검토

**STATUS: COMPLETE**

### 배경

백엔드는 `POST /api/v1/drafts/{id}/send` 계약을 제공하고, 이 경로는 draft를 읽어 전송한 뒤 `MarkDraftSent`로 draft/첨부 상태를 정리한다.
프론트엔드는 현재 일반 `POST /messages/send` 후 best-effort draft 삭제를 수행한다.
다만 draft-send 계약은 예약 전송과 수신확인 옵션을 직접 표현하지 않으므로, 기능 퇴행 없이 안전한 조건에서만 활용해야 한다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 웹메일 API helper가 `POST /drafts/{id}/send` 계약을 제공한다.
- [x] 비예약/수신확인 미사용 전송은 최신 draft 저장 후 draft-send 계약을 사용할 수 있다.
- [x] 예약 전송 또는 수신확인 전송은 기존 direct send 계약을 유지한다.
- [x] undo-countdown 전송도 draft-send 사용 여부를 보존한다.
- [x] draft-send 성공 시 별도 삭제 호출 없이 로컬 draft 상태를 정리한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-111: 사용자 웹메일 베타 안정화 — 수신확인 draft-send parity 검토

---

## ✅ TASK-111: 사용자 웹메일 베타 안정화 — 수신확인 draft-send parity 검토

**STATUS: COMPLETE**

### 배경

TASK-110에서 draft-send 계약은 안전한 조건에서만 사용하도록 연결했다.
남은 gap은 수신확인(`track_opens`)이다. direct send는 `track_opens`를 지원하지만 draft 저장/전송 계약은 이를 보존하지 않아, 추적 전송은 direct send로 fallback해야 했다.

### 구현 대상

- `internal/mailservice/draft_contract.go`
- `internal/mailservice/service.go`
- `internal/mailservice/service_test.go`
- `internal/maildb/drafts.go`
- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] draft 저장 계약이 `track_opens`를 표현한다.
- [x] draft DB 저장이 `track_opens`를 보존한다.
- [x] draft-send가 저장된 `track_opens`를 `SendText`에 전달한다.
- [x] 프론트 draft payload가 수신확인 상태를 저장한다.
- [x] 수신확인 전송도 예약 전송이 아니면 draft-send 경로를 사용할 수 있다.
- [x] 예약 전송은 기존 direct send를 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-112: 사용자 웹메일 베타 안정화 — 예약 전송 draft 계약 확장 검토

---

## ✅ TASK-112: 사용자 웹메일 베타 안정화 — 예약 전송 draft 계약 확장 검토

**STATUS: COMPLETE**

### 배경

TASK-111 이후 수신확인 전송은 draft-send parity를 갖췄다.
남은 direct-send fallback은 예약 전송이다. `SendTextRequest`는 이미 `ScheduledAt`을 지원하므로, draft 저장/조회/전송 계약이 예약 시각을 보존하면 예약 전송도 표준 draft-send 경로를 사용할 수 있다.

### 구현 대상

- `internal/mailservice/draft_contract.go`
- `internal/mailservice/service.go`
- `internal/mailservice/service_test.go`
- `internal/maildb/compose_requests.go`
- `internal/maildb/drafts.go`
- `apps/webmail/src/lib/api.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] draft 저장 계약이 `scheduled_at`을 표현한다.
- [x] draft DB 저장이 예약 시각을 보존한다.
- [x] draft-send가 저장된 예약 시각을 `SendText`에 전달한다.
- [x] 프론트 draft payload가 예약 전송 시각을 저장한다.
- [x] 예약 전송도 draft-send 경로를 사용할 수 있다.
- [x] 기존 예약 시간 검증과 UI 상태 정리는 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-113: 사용자 웹메일 베타 안정화 — draft-send 실패 복구/상태 유지 점검

---

## ✅ TASK-113: 사용자 웹메일 베타 안정화 — draft-send 실패 복구/상태 유지 점검

**STATUS: COMPLETE**

### 배경

TASK-112 이후 전송 전 최신 작성 상태는 draft로 저장되고, draft-send가 표준 전송 경로가 됐다.
전송 실패 시에는 draft가 남아 있으므로 사용자가 재시도할 수 있어야 하며, UI도 전송 실패와 초안 보존 상태를 분리해서 알려야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 준비를 위해 draft 저장에 성공하면 저장 상태가 UI에 반영된다.
- [x] draft-send/direct-send 실패 시 pending draft-send 상태가 정리된다.
- [x] 전송 실패 메시지가 초안 보존/재시도 가능성을 안내한다.
- [x] 성공 경로의 draft 정리 동작은 유지한다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-114: 사용자 웹메일 베타 안정화 — 초안 자동저장 예약/수신확인 상태 보존

---

## ✅ TASK-114: 사용자 웹메일 베타 안정화 — 초안 자동저장 예약/수신확인 상태 보존

**STATUS: COMPLETE**

### 배경

TASK-111/TASK-112에서 draft 저장 계약은 `track_opens`와 `scheduled_at`을 표현할 수 있게 됐다.
하지만 작성창의 자동저장/수동저장/닫기 전 저장 경로가 이 상태를 모두 포함하지 않으면, 초안 자체가 사용자의 전송 설정을 잃을 수 있다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 자동저장이 수신확인 상태를 draft payload에 포함한다.
- [x] 자동저장이 예약 전송 시각을 draft payload에 포함한다.
- [x] 수동저장이 수신확인/예약 상태를 draft payload에 포함한다.
- [x] 닫기 전 저장이 수신확인/예약 상태를 draft payload에 포함한다.
- [x] 기존 전송 직전 draft 저장 계약은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-115: 사용자 웹메일 베타 안정화 — 작성창 draft payload 중복 제거/계약 헬퍼화

---

## ✅ TASK-115: 사용자 웹메일 베타 안정화 — 작성창 draft payload 중복 제거/계약 헬퍼화

**STATUS: COMPLETE**

### 배경

작성창에는 자동저장, 수동저장, 닫기 전 저장, 전송 준비 저장이 각각 draft payload를 만든다.
TASK-101~114에서 계약을 맞췄지만 같은 필드 조합이 여러 곳에 반복되면 이후 `attachment_ids`, `track_opens`, `scheduled_at` 같은 필드가 다시 누락될 위험이 있다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] draft payload 생성이 단일 헬퍼를 통해 이뤄진다.
- [x] 자동저장/수동저장/닫기 전 저장/전송 준비 저장이 같은 헬퍼를 사용한다.
- [x] `attachment_ids`, `track_opens`, `scheduled_at`, `from` 계약이 모든 draft 저장 경로에서 유지된다.
- [x] backend가 받지 않는 draft-only payload 필드는 제거한다.
- [x] 동작과 디자인 톤은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-116: 사용자 웹메일 베타 안정화 — draft-send HTTP 계약 테스트 보강

---

## ✅ TASK-116: 사용자 웹메일 베타 안정화 — draft-send HTTP 계약 테스트 보강

**STATUS: COMPLETE**

### 배경

TASK-110~115에서 draft-send 중심 계약을 확장했다.
프론트/서비스/DB 검증은 통과했지만 HTTP 경계에서 `track_opens`, `scheduled_at`, bodyless draft-send, unknown query rejection, 응답 정규화가 회귀하지 않도록 테스트가 필요하다.

### 구현 대상

- `internal/httpapi/mail_test.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] draft 저장 HTTP 테스트가 `track_opens`를 서비스 요청까지 검증한다.
- [x] draft 저장 HTTP 테스트가 `scheduled_at`을 서비스 요청까지 검증한다.
- [x] draft-send HTTP 테스트가 응답 상태 정규화를 검증한다.
- [x] draft-send HTTP 테스트가 request body를 거부하는 계약을 검증한다.
- [x] draft-send HTTP 테스트가 unknown query key를 거부하는 계약을 검증한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-117: 사용자 웹메일 베타 안정화 — draft-send OpenAPI 계약 문서 점검

---

## ✅ TASK-117: 사용자 웹메일 베타 안정화 — draft-send OpenAPI 계약 문서 점검

**STATUS: COMPLETE**

### 배경

TASK-111/TASK-112에서 draft 저장 계약은 `track_opens`와 `scheduled_at`을 표현하고 draft-send로 전달한다.
HTTP 구현과 테스트는 보강됐지만, 공개 OpenAPI 계약이 이 옵션을 명확히 문서화하지 않으면 클라이언트 생성/외부 연동에서 drift가 생긴다.

### 구현 대상

- `docs/openapi.yaml`
- `internal/httpapi/openapi_contract_test.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] OpenAPI `ComposeRequest`가 `track_opens`를 문서화한다.
- [x] OpenAPI `ComposeRequest`가 `scheduled_at`을 draft 저장에도 적용되는 공용 계약으로 유지한다.
- [x] OpenAPI draft save request body가 `ComposeRequest`를 사용한다는 계약 테스트가 있다.
- [x] OpenAPI draft send operation이 bodyless로 문서화된다는 계약 테스트가 있다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-118: 사용자 웹메일 베타 안정화 — draft scheduled/tracking DB 통합 테스트 보강

---

## ✅ TASK-118: 사용자 웹메일 베타 안정화 — draft scheduled/tracking DB 통합 테스트 보강

**STATUS: COMPLETE**

### 배경

TASK-111/TASK-112에서 draft 저장은 `track_opens`와 `scheduled_at`을 flags에 보존하고 `GetDraftForSend`에서 복원한다.
단위/HTTP/OpenAPI 계약은 잠겼지만, 실제 PostgreSQL 저장/조회 경로에서도 이 값이 유지되는지 통합 테스트로 확인해야 한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] PostgreSQL draft 저장 통합 테스트가 `TrackOpens` 저장/복원을 검증한다.
- [x] PostgreSQL draft 저장 통합 테스트가 `ScheduledAt` 저장/복원을 검증한다.
- [x] draft-send attachment handoff 기존 통합 검증은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-119: 사용자 웹메일 베타 안정화 — draft-send 프론트 API 타입/응답 계약 보강

---

## ✅ TASK-119: 사용자 웹메일 베타 안정화 — draft-send 프론트 API 타입/응답 계약 보강

**STATUS: COMPLETE**

### 배경

백엔드 draft-send HTTP 응답은 `message` envelope 안에 정규화된 전송/배송/반송 상태를 제공한다.
프론트 API 타입이 이 응답을 최소 `{ id }`로만 표현하면 상태 필드 계약이 사라지고, 이후 UI에서 배송 상태를 활용할 때 타입 안전성이 약해진다.

### 구현 대상

- `apps/webmail/src/lib/api.ts`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 프론트 API가 공용 send result 타입을 정의한다.
- [x] `sendDraft` 응답 타입이 `send_status`, `delivery_status`, `bounce_status`, `message_id`를 표현한다.
- [x] `sendMessage` 응답 타입도 같은 send result envelope를 표현한다.
- [x] 기존 호출부 동작은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-120: 사용자 웹메일 베타 안정화 — 전송 성공 후 배송 상태 표시 준비

---

## ✅ TASK-120: 사용자 웹메일 베타 안정화 — 전송 성공 후 배송 상태 표시 준비

**STATUS: COMPLETE**

### 배경

TASK-119에서 프론트 API는 백엔드 send result envelope를 타입으로 표현하게 됐다.
다음 단계는 작성창이 전송 성공 응답의 초기 큐/배송/반송 상태를 버리지 않고 보존해, 베타에서 사용자가 “전송됨” 이후 상태를 이해할 수 있게 준비하는 것이다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 작성창이 전송 성공 응답의 send result를 상태로 보존한다.
- [x] 즉시/예약/undo-countdown 전송 성공 경로가 같은 result 저장 경로를 사용한다.
- [x] 전송 성공 UI가 초기 전송 상태를 간결하게 표시한다.
- [x] 기존 닫힘 타이밍과 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-121: 사용자 웹메일 베타 안정화 — 전송 결과 기반 최근 수신자 저장 계약 정리

---

## ✅ TASK-121: 사용자 웹메일 베타 안정화 — 전송 결과 기반 최근 수신자 저장 계약 정리

**STATUS: COMPLETE**

### 배경

전송 성공 후 최근 수신자/follow-up local state 갱신은 현재 undo-countdown 성공 경로에만 있다.
즉시 전송이나 예약 전송도 같은 성공 처리 계약을 가져야 사용자 경험이 일관된다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 성공 후 최근 수신자 저장이 공통 헬퍼로 분리된다.
- [x] undo-countdown 성공 경로가 공통 헬퍼를 사용한다.
- [x] 즉시 전송 성공 경로가 공통 헬퍼를 사용한다.
- [x] 예약 전송 성공 경로가 공통 헬퍼를 사용한다.
- [x] send result 저장/초안 정리/닫힘 타이밍은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-122: 사용자 웹메일 베타 안정화 — 전송 성공 처리 헬퍼 통합

---

## ✅ TASK-122: 사용자 웹메일 베타 안정화 — 전송 성공 처리 헬퍼 통합

**STATUS: COMPLETE**

### 배경

TASK-120/TASK-121에서 전송 성공 응답 보존과 최근 수신자/follow-up 갱신을 보강했다.
하지만 undo-countdown, 즉시 전송, 예약 전송 성공 경로가 여전히 같은 후처리를 반복하므로 이후 배송 상태 UI나 성공 토스트를 붙일 때 drift가 생길 수 있다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 성공 후처리가 단일 헬퍼로 통합된다.
- [x] undo-countdown 성공 경로가 통합 헬퍼를 사용한다.
- [x] 즉시 전송 성공 경로가 통합 헬퍼를 사용한다.
- [x] 예약 전송 성공 경로가 통합 헬퍼를 사용한다.
- [x] send result 저장, 최근 수신자/follow-up 갱신, 초안 정리, 닫힘 타이밍은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-123: 사용자 웹메일 베타 안정화 — 전송 실패 처리 헬퍼 통합

---

## ✅ TASK-123: 사용자 웹메일 베타 안정화 — 전송 실패 처리 헬퍼 통합

**STATUS: COMPLETE**

### 배경

TASK-122에서 전송 성공 후처리를 단일 헬퍼로 통합했다.
실패 경로도 undo-countdown, 즉시 전송, 예약 전송에서 같은 메시지/상태 정리 계약을 유지해야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 실패 후처리가 단일 헬퍼로 통합된다.
- [x] undo-countdown 실패 경로가 통합 헬퍼를 사용한다.
- [x] 즉시 전송 실패 경로가 통합 헬퍼를 사용한다.
- [x] 예약 전송 실패 경로가 통합 헬퍼를 사용한다.
- [x] pending draft-send 상태와 countdown 상태 정리 계약이 유지된다.
- [x] 사용자에게 초안 보존/재시도 가능성이 계속 안내된다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-124: 사용자 웹메일 베타 안정화 — 전송 준비 실패 안내 정리

---

## ✅ TASK-124: 사용자 웹메일 베타 안정화 — 전송 준비 실패 안내 정리

**STATUS: COMPLETE**

### 배경

전송 준비 단계는 최신 작성 상태를 draft로 저장한 뒤 draft-send를 수행한다.
이 단계에서 저장/업데이트가 실패하면 실제 전송은 시작되지 않았으므로, 사용자는 “전송 실패”가 아니라 “전송 준비 실패”와 초안 상태를 명확히 알아야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 준비 실패 후처리가 단일 헬퍼로 분리된다.
- [x] 전송 준비 실패 시 pending message/draft-send 상태가 정리된다.
- [x] 전송 준비 실패 시 사용자가 초안 저장을 다시 시도해야 함을 안내한다.
- [x] 일반 전송 실패 처리와 메시지가 구분된다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-125: 사용자 웹메일 베타 안정화 — 작성창 전송 헬퍼 명명/가독성 정리

---

## ✅ TASK-125: 사용자 웹메일 베타 안정화 — 작성창 전송 헬퍼 명명/가독성 정리

**STATUS: COMPLETE**

### 배경

TASK-124까지 전송 성공/실패/준비 실패 후처리가 각각 헬퍼로 정리됐다.
남은 반복은 undo-countdown, 예약 전송, 즉시 전송 경로가 모두 직접 draft-send/direct-send 분기식을 갖고 있다는 점이다.
다음 기능 고도화에서 계약을 덜 흔들기 위해, 전송 준비가 끝난 메시지를 실제로 dispatch하는 의도를 이름으로 드러낸다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] draft-send 사용 가능 여부 판단이 명명된 헬퍼를 통한다.
- [x] 준비된 메시지 dispatch가 명명된 헬퍼를 통한다.
- [x] undo-countdown 전송 경로가 동일 헬퍼를 사용한다.
- [x] 예약 전송 경로가 동일 헬퍼를 사용한다.
- [x] 즉시 전송 경로가 동일 헬퍼를 사용한다.
- [x] 기존 전송 성공/실패/초안 보존 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-126: 사용자 웹메일 베타 안정화 — 작성창 전송 상태 사용자 안내 고도화

---

## ✅ TASK-126: 사용자 웹메일 베타 안정화 — 작성창 전송 상태 사용자 안내 고도화

**STATUS: COMPLETE**

### 배경

TASK-120에서 백엔드 전송 결과를 작성창 성공 상태에 표시하기 시작했다.
다만 `queued`, `pending` 같은 내부 상태값을 그대로 노출하면 베타 사용자는 실제로 무엇이 일어났는지 이해하기 어렵다.
계약 값은 유지하되 작성창의 작은 상태 안내를 사용자 언어로 변환한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 상태값이 사용자 친화적인 한국어 문구로 표시된다.
- [x] 배송 상태값이 사용자 친화적인 한국어 문구로 표시된다.
- [x] 반송/신고 상태가 있을 때만 추가 안내로 표시된다.
- [x] 백엔드 응답 계약과 기존 성공/닫힘 타이밍은 변경하지 않는다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-127: 사용자 웹메일 베타 안정화 — 작성창 전송 결과 표시 로직 추출

---

## ✅ TASK-127: 사용자 웹메일 베타 안정화 — 작성창 전송 결과 표시 로직 추출

**STATUS: COMPLETE**

### 배경

TASK-126에서 전송 결과 상태값을 사용자 친화적인 문구로 바꿨다.
이 로직을 작성창 렌더 본문에 그대로 두면 이후 상태값이 늘어날 때 컴포넌트가 더 복잡해진다.
표시 로직을 순수 함수로 추출해 UI 상태 전이와 문구 포맷 책임을 분리한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 상태 label 매핑이 순수 함수로 분리된다.
- [x] 배송 상태 label 매핑이 순수 함수로 분리된다.
- [x] 반송 상태 label 매핑이 순수 함수로 분리된다.
- [x] 최종 전송 결과 label 조립이 순수 함수로 분리된다.
- [x] 작성창 렌더 본문은 포맷 함수를 호출하는 형태로 단순화된다.
- [x] 기존 문구와 표시 조건은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-128: 사용자 웹메일 베타 안정화 — 작성창 전송 결과 타입/표시 테스트 보강

---

## ✅ TASK-128: 사용자 웹메일 베타 안정화 — 작성창 전송 결과 타입/표시 테스트 보강

**STATUS: COMPLETE**

### 배경

TASK-127에서 전송 결과 표시 로직을 순수 함수로 추출했다.
웹메일 앱은 아직 별도 단위 테스트 러너를 갖고 있지 않으므로, 새 테스트 도구를 성급하게 도입하지 않고 `tsc` 검증에 포함되는 계약 fixture를 먼저 둔다.
이후 Vitest/Jest 도입 시 같은 모듈을 바로 런타임 테스트 대상으로 사용할 수 있어야 한다.

### 구현 대상

- `apps/webmail/src/lib/sendResultLabel.ts`
- `apps/webmail/src/lib/sendResultLabel.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 결과 label 포맷터가 React 컴포넌트 밖의 순수 모듈에 위치한다.
- [x] 작성창은 순수 포맷터를 import해 사용한다.
- [x] `SendMessageResult` 타입을 만족하는 계약 fixture가 타입체크에 포함된다.
- [x] queued/pending, bounced, unknown fallback, null 결과 샘플이 계약 fixture에 포함된다.
- [x] 새 테스트 러너를 추가하지 않고 기존 검증 루프를 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-129: 사용자 웹메일 베타 안정화 — 작성창 예약 전송 성공 안내 분리

---

## ✅ TASK-129: 사용자 웹메일 베타 안정화 — 작성창 예약 전송 성공 안내 분리

**STATUS: COMPLETE**

### 배경

예약 전송은 즉시 발송과 다른 사용자 기대를 만든다.
기존 작성창 버튼은 예약 전송 성공 직후에도 `전송됨`으로 표시될 수 있어, 사용자가 즉시 발송 완료로 오해할 수 있다.
전송 계약과 디자인은 유지하면서 성공 상태의 버튼 문구만 예약 전송과 즉시 전송으로 분리한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 예약 전송 성공 상태는 `예약됨`으로 표시된다.
- [x] 즉시 전송 성공 상태는 기존 `전송됨` 표시를 유지한다.
- [x] 예약 전송 시작 전 버튼 문구 `예약 전송`은 유지한다.
- [x] 기존 전송/예약 API 계약과 성공 후 닫힘 타이밍은 변경하지 않는다.
- [x] 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-130: 사용자 웹메일 베타 안정화 — 작성창 전송 버튼 라벨 계산 정리

---

## ✅ TASK-130: 사용자 웹메일 베타 안정화 — 작성창 전송 버튼 라벨 계산 정리

**STATUS: COMPLETE**

### 배경

TASK-129에서 예약 전송 성공 문구를 분리하면서 작성창 전송 버튼 label 조건이 더 복잡해졌다.
버튼의 시각 디자인은 유지하되, 상태별 label 계산을 순수 함수로 분리해 JSX를 단순하게 유지한다.

### 구현 대상

- `apps/webmail/src/lib/composeSendButtonLabel.ts`
- `apps/webmail/src/lib/composeSendButtonLabel.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 버튼 label 계산이 순수 함수로 분리된다.
- [x] 전송 중, 즉시 전송 성공, 예약 전송 성공, 업로드 중, 예약 대기, 기본 상태 fixture가 타입체크에 포함된다.
- [x] 작성창 JSX는 계산된 label/disabled 상태를 사용한다.
- [x] 기존 버튼 디자인과 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-131: 사용자 웹메일 베타 안정화 — 작성창 전송 버튼 접근성 상태 보강

---

## ✅ TASK-131: 사용자 웹메일 베타 안정화 — 작성창 전송 버튼 접근성 상태 보강

**STATUS: COMPLETE**

### 배경

전송 버튼은 시각적으로 전송 중, 업로드 중, 전송 완료, 예약 완료 상태를 표현한다.
베타 품질에서는 보조기술도 이 상태 변화를 이해할 수 있어야 한다.
디자인을 변경하지 않고 버튼과 상태 메시지의 접근성 속성을 보강한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 버튼이 현재 label을 접근성 이름으로 제공한다.
- [x] 전송/업로드 진행 상태가 `aria-busy`로 전달된다.
- [x] 전송 결과 상태 메시지가 `role=status`와 `aria-live=polite`로 노출된다.
- [x] 저장 상태 메시지가 live status로 노출된다.
- [x] 전송 옵션 버튼이 menu popup/expanded 상태를 제공한다.
- [x] 기존 디자인과 전송 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-132: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 항목 접근성 보강

---

## ✅ TASK-132: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 항목 접근성 보강

**STATUS: COMPLETE**

### 배경

TASK-131에서 전송 옵션 trigger가 menu popup/expanded 상태를 제공하도록 보강했다.
이제 열린 예약 전송 메뉴 내부 항목도 menu item 구조와 읽기 좋은 접근성 이름을 제공해야 한다.
시각 디자인과 클릭 동작은 그대로 유지한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 옵션 trigger가 열린 메뉴를 `aria-controls`로 연결한다.
- [x] 전송 옵션 메뉴가 안정적인 id를 가진다.
- [x] 예약 옵션 버튼이 `role=menuitem`을 가진다.
- [x] 예약 옵션 버튼이 날짜/시간 정보를 포함한 접근성 이름을 가진다.
- [x] 보내고 보관/사용자 지정 날짜 항목도 menu item으로 노출된다.
- [x] 기존 디자인과 클릭 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-133: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 키보드 닫기 보강

---

## ✅ TASK-133: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 키보드 닫기 보강

**STATUS: COMPLETE**

### 배경

TASK-132에서 예약 전송 메뉴의 ARIA 구조를 보강했다.
키보드 사용자에게는 열린 메뉴를 `Escape`로 닫을 수 있는 동작도 필요하다.
메뉴 항목 선택 동작과 디자인은 유지하면서 닫기 키보드 동작을 추가한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 열린 전송 옵션 메뉴에서 `Escape` 입력 시 메뉴가 닫힌다.
- [x] `Escape` 처리는 상위 작성창 이벤트로 전파되지 않는다.
- [x] 기존 메뉴 항목 클릭 동작은 유지한다.
- [x] 기존 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-134: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 외부 클릭 닫기 보강

---

## ✅ TASK-134: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 외부 클릭 닫기 보강

**STATUS: COMPLETE**

### 배경

예약 전송 메뉴는 `Escape`로 닫을 수 있게 됐다.
일반적인 popover/menu 기대에 맞추려면 메뉴 바깥을 클릭했을 때도 닫혀야 한다.
기존 메뉴 선택 동작과 디자인은 유지한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 옵션 wrapper가 안정적인 ref를 가진다.
- [x] 전송 옵션 메뉴가 열린 동안 문서 `mousedown` 외부 클릭을 감지한다.
- [x] 메뉴 바깥 클릭 시 전송 옵션 메뉴가 닫힌다.
- [x] 메뉴 내부 클릭은 기존 선택 동작을 유지한다.
- [x] 이벤트 리스너는 메뉴가 닫히거나 컴포넌트가 정리될 때 제거된다.
- [x] 기존 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-135: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 닫기 로직 통합

---

## ✅ TASK-135: 사용자 웹메일 베타 안정화 — 작성창 예약 메뉴 닫기 로직 통합

**STATUS: COMPLETE**

### 배경

TASK-133과 TASK-134로 전송 옵션 메뉴 닫기 경로가 늘어났다.
외부 클릭, Escape, 메뉴 항목 선택이 모두 직접 상태 setter를 호출하면 다음 확장에서 정리 비용이 커진다.
메뉴 닫기 의도를 명명된 헬퍼로 통합한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 전송 옵션 메뉴 닫기 동작이 `closeSendDropdown` 헬퍼로 명명된다.
- [x] 외부 클릭 닫기 경로가 공통 헬퍼를 사용한다.
- [x] Escape 닫기 경로가 공통 헬퍼를 사용한다.
- [x] 예약 옵션/보내고 보관/사용자 지정 날짜 선택 경로가 공통 헬퍼를 사용한다.
- [x] 기존 메뉴 열기 toggle 동작은 유지한다.
- [x] 기존 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-136: 사용자 웹메일 베타 안정화 — 작성창 사용자 지정 예약 시간 안내 보강

---

## ✅ TASK-136: 사용자 웹메일 베타 안정화 — 작성창 사용자 지정 예약 시간 안내 보강

**STATUS: COMPLETE**

### 배경

예약 전송 시간 검증은 전송 시점에 이미 수행된다.
하지만 사용자 지정 예약 시간을 고를 때 현재 시각 이후만 가능하다는 기준을 미리 알려주면 실패 경험을 줄일 수 있다.
디자인 톤을 유지하면서 작은 안내와 접근성 설명을 추가한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 사용자 지정 예약 시간 입력에 접근성 label이 제공된다.
- [x] 예약 시간 입력이 안내 문구와 `aria-describedby`로 연결된다.
- [x] 현재 시각 이후만 선택 가능하다는 안내가 표시된다.
- [x] 기존 min 검증과 전송 시 검증은 유지한다.
- [x] 기존 디자인 톤은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-137: 사용자 웹메일 베타 안정화 — 작성창 예약 시간 안내 상수화

---

## ✅ TASK-137: 사용자 웹메일 베타 안정화 — 작성창 예약 시간 안내 상수화

**STATUS: COMPLETE**

### 배경

TASK-136에서 사용자 지정 예약 시간 안내 문구를 추가했다.
문구가 렌더마다 새로 선언될 필요는 없고, 이후 테스트/재사용을 위해 컴포넌트 상수로 두는 것이 더 명확하다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 예약 시간 안내 문구가 컴포넌트 내부 렌더 계산에서 분리된다.
- [x] 안내 문구가 module-level 상수로 명명된다.
- [x] 기존 표시 문구와 접근성 연결은 유지한다.
- [x] 기존 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-138: 사용자 웹메일 베타 안정화 — 작성창 예약 시간 min 계산 명명

---

## ✅ TASK-138: 사용자 웹메일 베타 안정화 — 작성창 예약 시간 min 계산 명명

**STATUS: COMPLETE**

### 배경

예약 전송 입력은 HTML `datetime-local` 값을 사용한다.
기존 inline 계산은 UTC 기반 `toISOString().slice(0, 16)`을 사용하고 있어, 로컬 시간 입력과 시간대 기준이 어긋날 수 있다.
예약 시간 표시/기본값/min 값을 로컬 datetime-local 포맷 헬퍼로 통일한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `datetime-local` 값을 만드는 로컬 시간 포맷 헬퍼가 추가된다.
- [x] preset 예약 옵션이 로컬 datetime-local 포맷을 사용한다.
- [x] 사용자 지정 예약 기본값이 로컬 datetime-local 포맷을 사용한다.
- [x] 사용자 지정 예약 입력 `min` 값이 명명된 계산값을 사용한다.
- [x] 전송 payload의 ISO 직렬화는 기존 계약을 유지한다.
- [x] 기존 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-139: 사용자 웹메일 베타 안정화 — 예약 시간 포맷 헬퍼 타입 fixture 추가

---

## ✅ TASK-139: 사용자 웹메일 베타 안정화 — 예약 시간 포맷 헬퍼 타입 fixture 추가

**STATUS: COMPLETE**

### 배경

TASK-138에서 HTML `datetime-local`에 맞는 로컬 시간 포맷 헬퍼를 추가했다.
시간 포맷은 다른 예약 UI에서도 재사용될 가능성이 높고, 컴포넌트 내부에 묶어두기보다 순수 모듈과 타입 fixture로 관리하는 편이 안전하다.

### 구현 대상

- `apps/webmail/src/lib/dateTimeLocal.ts`
- `apps/webmail/src/lib/dateTimeLocal.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 로컬 datetime-local 포맷 헬퍼가 React 컴포넌트 밖의 순수 모듈로 이동한다.
- [x] 작성창은 공유 헬퍼를 import해 사용한다.
- [x] 한 자리 월/일/시/분 padding fixture가 타입체크에 포함된다.
- [x] 두 자리 월/일/시/분 fixture가 타입체크에 포함된다.
- [x] 기존 예약 시간 표시/입력 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-140: 사용자 웹메일 베타 안정화 — 예약 시간 포맷 런타임 검증 스크립트 추가

---

## ✅ TASK-140: 사용자 웹메일 베타 안정화 — 예약 시간 포맷 런타임 검증 스크립트 추가

**STATUS: COMPLETE**

### 배경

TASK-139에서 예약 시간 포맷 헬퍼의 타입 fixture를 추가했다.
타입 fixture는 계약 shape를 지키는 데 유용하지만, 실제 문자열 출력도 런타임에서 확인할 수 있어야 한다.
새 테스트 러너를 도입하지 않고 Node의 TypeScript strip 실행으로 작은 검증 스크립트를 추가한다.

### 구현 대상

- `apps/webmail/scripts/check-date-time-local.mjs`
- `apps/webmail/package.json`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 예약 시간 포맷 헬퍼를 실행하는 런타임 검증 스크립트가 있다.
- [x] 한 자리 월/일/시/분 padding 출력이 런타임에서 검증된다.
- [x] 두 자리 월/일/시/분 출력이 런타임에서 검증된다.
- [x] 웹메일 package script로 검증을 실행할 수 있다.
- [x] 새 테스트 러너를 추가하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 예약 시간 포맷 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:datetime-local` in `apps/webmail` 통과

### 다음 태스크

TASK-141: 사용자 웹메일 베타 안정화 — 작성창 닫기 전 예약 설정 보존 점검

---

## ✅ TASK-141: 사용자 웹메일 베타 안정화 — 작성창 닫기 전 예약 설정 보존 점검

**STATUS: COMPLETE**

### 배경

닫기 전 임시저장 경로는 `buildDraftData`를 사용하므로 예약 설정을 draft payload에 포함한다.
다만 사용자에게는 닫기 확인 문구가 일반 임시저장처럼 보이므로, 예약 설정도 같이 보존된다는 점을 더 명확히 안내한다.
기존 저장 계약과 디자인은 유지한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 예약 전송 설정이 있는 상태의 닫기 확인 문구가 예약 설정 보존을 안내한다.
- [x] 예약 전송 설정이 없는 상태의 기존 닫기 확인 문구는 유지한다.
- [x] 닫기 전 임시저장은 기존 `buildDraftData` 경로를 계속 사용한다.
- [x] 기존 draft 저장 계약과 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-142: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 문구 계산 정리

---

## ✅ TASK-142: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 문구 계산 정리

**STATUS: COMPLETE**

### 배경

TASK-141에서 예약 설정 여부에 따라 닫기 확인 문구가 달라지도록 했다.
상태 기반 문구 계산은 작성창 JSX 안에 남겨두기보다 순수 helper와 타입 fixture로 관리하는 것이 다음 확장에 안전하다.

### 구현 대상

- `apps/webmail/src/lib/composeCloseSavePrompt.ts`
- `apps/webmail/src/lib/composeCloseSavePrompt.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 확인 문구 계산이 순수 helper로 분리된다.
- [x] 예약/일반 닫기 확인 문구 fixture가 타입체크에 포함된다.
- [x] 작성창은 helper를 import해 사용한다.
- [x] 기존 문구와 저장 동작은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-143: 사용자 웹메일 베타 안정화 — 작성창 예약/닫기 문구 런타임 검증 확장

---

## ✅ TASK-143: 사용자 웹메일 베타 안정화 — 작성창 예약/닫기 문구 런타임 검증 확장

**STATUS: COMPLETE**

### 배경

TASK-140에서 예약 시간 포맷 런타임 검증을 추가했다.
이후 전송 버튼 label과 닫기 확인 문구가 순수 helper로 분리됐으므로, 같은 가벼운 Node 검증 루프에서 실제 출력 문구까지 확인한다.

### 구현 대상

- `apps/webmail/scripts/check-date-time-local.mjs`
- `apps/webmail/package.json`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 확인 일반/예약 문구가 런타임 검증에 포함된다.
- [x] 예약 전송 버튼 준비/성공 문구가 런타임 검증에 포함된다.
- [x] 기존 datetime-local 런타임 검증은 유지한다.
- [x] 더 넓은 helper 검증용 package script가 제공된다.
- [x] 기존 `test:datetime-local` 진입점은 호환 alias로 유지된다.
- [x] 새 테스트 러너를 추가하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-144: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 스크립트 명명 정리

---

## ✅ TASK-144: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 스크립트 명명 정리

**STATUS: COMPLETE**

### 배경

TASK-143에서 기존 datetime-local 전용 스크립트가 작성창 helper 전반을 검증하도록 확장됐다.
파일명이 여전히 `check-date-time-local.mjs`이면 역할을 오해하기 쉬우므로, 실제 책임에 맞게 이름을 정리한다.

### 구현 대상

- `apps/webmail/scripts/check-compose-helpers.mjs`
- `apps/webmail/scripts/check-date-time-local.mjs`
- `apps/webmail/package.json`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 런타임 검증 스크립트 파일명이 작성창 helper 검증 책임을 드러낸다.
- [x] `test:compose-helpers` package script가 새 파일명을 사용한다.
- [x] `test:datetime-local` 호환 alias는 유지한다.
- [x] 기존 런타임 검증 내용은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-145: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 출력 문구 정리

---

## ✅ TASK-145: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 출력 문구 정리

**STATUS: COMPLETE**

### 배경

TASK-144에서 런타임 검증 스크립트명을 정리했다.
스크립트 출력도 실제 검증 범위인 datetime, 전송 버튼, 닫기 저장 helper를 드러내면 자동화 로그에서 맥락을 더 빨리 파악할 수 있다.

### 구현 대상

- `apps/webmail/scripts/check-compose-helpers.mjs`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 런타임 검증 성공 출력이 실제 검증 범위를 드러낸다.
- [x] 기존 검증 assert 내용은 유지한다.
- [x] 새 테스트 러너를 추가하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-146: 사용자 웹메일 베타 안정화 — 작성창 helper 검증을 개발문서에 명시

---

## ✅ TASK-146: 사용자 웹메일 베타 안정화 — 작성창 helper 검증을 개발문서에 명시

**STATUS: COMPLETE**

### 배경

작성창 helper 런타임 검증이 추가됐으므로, 앞으로 같은 범위의 변경에서 이 검증이 누락되지 않도록 개발문서에 명시해야 한다.
코드 변경 없이 웹메일 개발 검증 루프를 문서화한다.

### 구현 대상

- `docs/WEBMAIL_ROADMAP.md`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 웹메일 개발 검증 루프에 `go test ./...`가 명시된다.
- [x] 웹메일 개발 검증 루프에 `pnpm type-check`가 명시된다.
- [x] 작성창 helper 관련 변경 시 `pnpm test:compose-helpers` 실행 기준이 명시된다.
- [x] 코드 변경 없이 개발문서만 갱신한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-147: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 alias 문서화

---

## ✅ TASK-147: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 alias 문서화

**STATUS: COMPLETE**

### 배경

`pnpm test:compose-helpers`가 작성창 helper 런타임 검증의 표준 명령이 됐다.
다만 기존 `pnpm test:datetime-local`도 호환 alias로 유지되므로, 왜 alias가 남아 있는지 문서에 남긴다.

### 구현 대상

- `docs/WEBMAIL_ROADMAP.md`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `test:datetime-local`이 호환 alias임을 문서화한다.
- [x] alias가 같은 compose-helper 런타임 검증을 실행함을 문서화한다.
- [x] alias 유지 이유가 datetime-local formatter 최초 검증에서 출발했기 때문임을 문서화한다.
- [x] 코드 변경 없이 개발문서만 갱신한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-148: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 alias 실행 확인

---

## ✅ TASK-148: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 alias 실행 확인

**STATUS: COMPLETE**

### 배경

TASK-147에서 `pnpm test:datetime-local`이 `pnpm test:compose-helpers`의 호환 alias임을 문서화했다.
문서 설명이 실제 package script 동작과 맞는지 확인하고, 검증 결과를 기록한다.

### 구현 대상

- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `pnpm test:datetime-local`이 실행된다.
- [x] alias가 `pnpm test:compose-helpers`를 호출하는 것이 command output으로 확인된다.
- [x] compose helper 런타임 검증이 alias 경로에서도 통과한다.
- [x] 코드 변경 없이 검증 결과만 개발문서에 기록한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:datetime-local` in `apps/webmail` 통과

### 다음 태스크

TASK-149: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 command 명명 일관성 점검

---

## ✅ TASK-149: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 command 명명 일관성 점검

**STATUS: COMPLETE**

### 배경

작성창 helper 런타임 검증에는 표준 명령과 호환 alias가 함께 존재한다.
미래 작업자가 어떤 명령을 기준으로 삼아야 하는지 명확히 하기 위해 canonical command를 개발문서에 명시한다.

### 구현 대상

- `docs/WEBMAIL_ROADMAP.md`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `pnpm test:compose-helpers`가 canonical command임을 문서화한다.
- [x] `pnpm test:datetime-local`은 compatibility alias임을 유지해서 설명한다.
- [x] 기존 package script 동작은 변경하지 않는다.
- [x] 코드 변경 없이 개발문서만 갱신한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-150: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 범위 주석 추가

---

## ✅ TASK-150: 사용자 웹메일 베타 안정화 — 작성창 helper 검증 범위 주석 추가

**STATUS: COMPLETE**

### 배경

작성창 helper 런타임 검증은 datetime-local 포맷, 전송 버튼 문구, 닫기 저장 문구를 함께 확인한다.
스크립트만 봐도 왜 이 assert들이 묶여 있는지 이해할 수 있도록 범위 주석을 추가한다.

### 구현 대상

- `apps/webmail/scripts/check-compose-helpers.mjs`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 런타임 검증 스크립트에 검증 범위 주석이 있다.
- [x] 주석은 순수 compose helper와 문구/datetime formatting 회귀 방지 목적을 설명한다.
- [x] 기존 assert와 package script 동작은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-151: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 접근성 상태 보강

---

## ✅ TASK-151: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 접근성 상태 보강

**STATUS: COMPLETE**

### 배경

닫기 확인 패널은 저장/버리기/취소라는 중요한 선택을 요구한다.
시각적으로는 기존 디자인을 유지하되, 보조기술이 확인 UI로 인식할 수 있도록 역할과 label 연결을 보강한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 확인 패널이 `role=alertdialog`로 노출된다.
- [x] 닫기 확인 패널이 표시 문구와 `aria-labelledby`로 연결된다.
- [x] inline confirmation이므로 `aria-modal=false`로 명시된다.
- [x] 기존 저장/버리기/취소 동작과 디자인은 변경하지 않는다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-152: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 버튼 접근성 라벨 보강

---

## ✅ TASK-152: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 버튼 접근성 라벨 보강

**STATUS: COMPLETE**

### 배경

닫기 확인 패널은 `임시저장`, `버리기`, `취소` 세 버튼을 제공한다.
짧은 시각 텍스트는 유지하되, 보조기술에는 각 버튼이 작성창 닫기 흐름에서 어떤 결과를 만드는지 더 명확히 전달한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 임시저장 버튼이 “저장 후 닫기” 맥락의 접근성 이름을 가진다.
- [x] 버리기 버튼이 “저장하지 않고 닫기” 맥락의 접근성 이름을 가진다.
- [x] 취소 버튼이 “작성 계속하기” 맥락의 접근성 이름을 가진다.
- [x] 기존 버튼 텍스트, 동작, 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-153: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 Escape 취소 지원

---

## ✅ TASK-153: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 Escape 취소 지원

**STATUS: COMPLETE**

### 배경

닫기 확인 패널은 안전한 취소 경로가 명확해야 한다.
키보드 사용자가 `Escape`로 패널을 닫으면 작성창 자체를 닫거나 버리지 않고, 취소 버튼과 같은 “작성 계속” 상태로 돌아가야 한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 확인 패널에서 `Escape` 입력 시 확인 패널이 닫힌다.
- [x] `Escape`는 작성창 닫기/버리기/저장을 실행하지 않는다.
- [x] `Escape` 처리는 상위 작성창 이벤트로 전파되지 않는다.
- [x] 기존 버튼 동작과 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-154: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 취소 헬퍼 통합

---

## ✅ TASK-154: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 취소 헬퍼 통합

**STATUS: COMPLETE**

### 배경

닫기 확인 패널 취소는 버튼과 Escape에서 같은 의미를 가진다.
직접 state setter를 반복하지 않고, “닫기 확인 취소” 의도를 helper로 명명해 유지보수성을 높인다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 확인 취소 동작이 `cancelCloseConfirmation` helper로 명명된다.
- [x] Escape 취소 경로가 공통 helper를 사용한다.
- [x] 취소 버튼이 공통 helper를 사용한다.
- [x] 기존 저장/버리기/취소 동작과 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-155: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 헬퍼 분리

---

## ✅ TASK-155: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 헬퍼 분리

**STATUS: COMPLETE**

### 배경

닫기 확인 패널의 `임시저장` 버튼은 draft 저장 후 작성창을 닫는 중요한 경로다.
inline async handler에 저장 계약이 길게 남아 있으면 이후 예약/첨부/추적 상태 보존 변경 시 실수하기 쉽다.
저장 후 닫기 의도를 helper로 분리하되 기존 best-effort 저장 동작은 유지한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 전 임시저장 후 닫기 동작이 `saveDraftAndClose` helper로 분리된다.
- [x] helper는 기존 `buildDraftData` 경로를 계속 사용한다.
- [x] 저장 실패가 작성창 닫기를 막지 않는 기존 best-effort 동작을 유지한다.
- [x] 임시저장 버튼은 helper를 호출하는 형태로 단순화된다.
- [x] 기존 버튼 텍스트, 접근성 label, 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-156: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 폐기 헬퍼 분리

---

## ✅ TASK-156: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 폐기 헬퍼 분리

**STATUS: COMPLETE**

### 배경

닫기 확인 패널의 `버리기` 버튼은 저장하지 않고 작성창을 닫는 명시적인 폐기 경로다.
저장/취소 경로처럼 폐기 의도도 helper로 명명해 버튼 handler를 단순화한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 저장하지 않고 닫기 동작이 `discardDraftAndClose` helper로 명명된다.
- [x] 버리기 버튼이 공통 helper를 호출한다.
- [x] 기존 폐기 동작은 `onClose` 호출 그대로 유지한다.
- [x] 기존 버튼 텍스트, 접근성 label, 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-157: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 액션 명명 일관성 점검

---

## ✅ TASK-157: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 액션 명명 일관성 점검

**STATUS: COMPLETE**

### 배경

TASK-154부터 TASK-156까지 닫기 확인 패널의 세 액션이 helper로 정리됐다.
저장 후 닫기, 저장하지 않고 닫기, 닫기 취소가 각각 명명된 상태임을 문서화해 이후 close-confirm 흐름을 확장할 기준으로 삼는다.

### 구현 대상

- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 저장 후 닫기 helper 이름이 문서화된다.
- [x] 저장하지 않고 닫기 helper 이름이 문서화된다.
- [x] 닫기 취소 helper 이름이 문서화된다.
- [x] 코드 변경 없이 개발문서만 갱신한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-158: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 중복 클릭 방지

---

## ✅ TASK-158: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 중복 클릭 방지

**STATUS: COMPLETE**

### 배경

닫기 확인의 임시저장은 네트워크 저장 후 작성창을 닫는 경로다.
사용자가 저장 버튼을 반복 클릭하면 중복 draft 저장/닫기 시도가 생길 수 있으므로, 저장 진행 상태를 명시적으로 관리한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 전 임시저장 진행 상태가 state로 관리된다.
- [x] 저장 진행 중 `saveDraftAndClose` 중복 실행이 차단된다.
- [x] 저장 진행 중 닫기 확인 패널이 `aria-busy` 상태를 노출한다.
- [x] 저장 진행 중 확인 액션 버튼들이 비활성화된다.
- [x] 저장 중 버튼 문구가 사용자에게 진행 상태를 알린다.
- [x] 기존 best-effort 저장 후 닫기 계약은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-159: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 상태 라벨 계산 분리

---

## ✅ TASK-159: 사용자 웹메일 베타 안정화 — 작성창 닫기 확인 저장 상태 라벨 계산 분리

**STATUS: COMPLETE**

### 배경

TASK-158에서 닫기 전 저장 진행 중 버튼 문구가 `저장 중...`으로 바뀌도록 했다.
상태 기반 문구 계산은 전송 버튼과 동일하게 순수 helper로 분리해 일관성을 맞춘다.

### 구현 대상

- `apps/webmail/src/lib/composeCloseSaveButtonLabel.ts`
- `apps/webmail/src/lib/composeCloseSaveButtonLabel.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `apps/webmail/scripts/check-compose-helpers.mjs`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 저장 버튼 문구 계산이 순수 helper로 분리된다.
- [x] idle/saving 상태 fixture가 타입체크에 포함된다.
- [x] 작성창은 helper를 import해 사용한다.
- [x] 작성창 helper 런타임 검증에 idle/saving 문구가 포함된다.
- [x] 기존 버튼 동작과 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-160: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 버튼 접근성 라벨 상태화

---

## ✅ TASK-160: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 버튼 접근성 라벨 상태화

**STATUS: COMPLETE**

### 배경

TASK-159에서 닫기 저장 버튼의 시각 문구가 상태 helper를 통해 계산되도록 했다.
저장 진행 중에는 접근성 label도 같은 상태를 반영해야 보조기술 사용자가 버튼의 현재 의미를 이해할 수 있다.

### 구현 대상

- `apps/webmail/src/lib/composeCloseSaveButtonAriaLabel.ts`
- `apps/webmail/src/lib/composeCloseSaveButtonAriaLabel.contract.ts`
- `apps/webmail/src/components/ComposeModal.tsx`
- `apps/webmail/scripts/check-compose-helpers.mjs`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 닫기 저장 버튼 접근성 label 계산이 순수 helper로 분리된다.
- [x] idle/saving 접근성 label fixture가 타입체크에 포함된다.
- [x] 작성창 저장 버튼이 상태 기반 접근성 label을 사용한다.
- [x] 작성창 helper 런타임 검증에 idle/saving 접근성 label이 포함된다.
- [x] 기존 저장 동작과 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-161: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 접근성 helper 범위 문서화

---

## ✅ TASK-161: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 접근성 helper 범위 문서화

**STATUS: COMPLETE**

### 배경

TASK-160에서 닫기 저장 버튼 접근성 label helper와 런타임 검증이 추가됐다.
웹메일 검증 루프 문서도 이 helper 범위를 포함하도록 갱신한다.

### 구현 대상

- `docs/WEBMAIL_ROADMAP.md`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] compose-helper 검증 범위에 close-save button accessibility label이 포함된다.
- [x] canonical command 문서 기준은 유지한다.
- [x] 코드 변경 없이 개발문서만 갱신한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 작성창 helper 런타임 검증 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- `pnpm test:compose-helpers` in `apps/webmail` 통과

### 다음 태스크

TASK-162: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 진행 중 Escape 처리 고정

---

## ✅ TASK-162: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 진행 중 Escape 처리 고정

**STATUS: COMPLETE**

### 배경

TASK-158에서 닫기 전 저장 진행 중에는 확인 액션 버튼을 비활성화했다.
하지만 키보드 Escape 경로도 같은 저장 진행 상태를 존중해야 한다.
저장 중 Escape는 상위 단축키로 전파하지 않되, 확인 패널을 닫지도 않도록 고정한다.

### 구현 대상

- `apps/webmail/src/components/ComposeModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 저장 진행 중 Escape 입력은 확인 패널을 닫지 않는다.
- [x] 저장 진행 중 Escape 입력은 상위 작성창 이벤트로 전파되지 않는다.
- [x] 저장 진행 중이 아닐 때 Escape 취소 동작은 유지한다.
- [x] 기존 버튼 동작과 디자인은 유지한다.
- [x] `go test ./...` 통과.
- [x] 웹메일 타입 체크 통과.
- [x] 기능 단위 커밋 후 push.

### 검증

- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과

### 다음 태스크

TASK-163: 사용자 웹메일 베타 안정화 — 작성창 닫기 저장 상태 helper 문서화

---

## ⏹️ TASK-096: 웹메일 성능 최적화 + 번들 크기 감소 (Blocked on UI rendering issue)

**STATUS: BLOCKED**
**Issue**: Hierarchical org chart data loaded in DB but not rendering in UI despite API path fix

**자세한 내용 (완료되지 않음)**

### 목표

webmail Phase 3이 완료되고 E2E 테스트가 준비되었으므로, 성능 최적화로 사용자 경험을 개선한다.
번들 크기 감소, 렌더링 최적화, 메모리 사용량 개선을 통해 제품 수준의 성능을 달성한다.

### 구현 대상

1. 번들 크기 분석 및 최적화
   - `next/dynamic` import로 큰 컴포넌트 코드 분할 (ComposeModal, OrgPickerModal, etc.)
   - 불필요한 의존성 제거 또는 경량화
   - Tree-shaking 확인 (unused exports 제거)

2. 렌더링 최적화
   - MessageList: 가상화 (virtualization) 구현으로 큰 목록 성능 개선
   - RecipientChips: useMemo/useCallback으로 불필요한 리렌더링 방지
   - ReadingPane: 이미지 lazy loading, intersection observer
   - ComposeModal: editor 초기화 최적화, 언마운트 시 cleanup

3. 메모리 최적화
   - Message 캐시 크기 제한 (최근 50개만 유지)
   - 큰 메일 본문 텍스트 제한 (1MB max)
   - Draft autosave 간격 조정 (3s → 5s)

4. 네트워크 최적화
   - API 응답 캐싱 (@tanstack/react-query stale/fresh times)
   - Prefetch: 메일 목록 보이면 다음 페이지 프리페치
   - 이미지 프록시 최적화 (resize, format conversion)

### 완료 조건

- [ ] `pnpm build` 번들 크기 측정 및 기록
- [ ] Dynamic import로 코드 분할 (최소 3개 컴포넌트)
- [ ] MessageList 가상화 구현 및 성능 테스트
- [ ] RecipientChips 메모이제이션 적용
- [ ] ReadingPane 이미지 lazy loading
- [ ] Draft autosave 간격 조정
- [ ] 메모리 캐시 제한 구현
- [ ] E2E 테스트 여전히 통과
- [ ] 성능 메트릭 개선 확인 (lighthouse)
- [ ] docs/CURRENT_STATUS.md 갱신

### 다음 태스크

TASK-097: 백엔드 Phase 5 (Mail Security & Milter 프로토콜) 또는 모바일 반응형 강화

---

## ✅ TASK-095: 웹메일 E2E 테스트 및 통합 테스트 커버리지

**STATUS: COMPLETE**

### 완료 (2026-05-12)

- `playwright.config.ts`: Chromium 브라우저, baseURL=http://localhost:3003, HTML 리포트
- `package.json`: "test:e2e", "test:e2e:ui" npm 스크립트 추가
- `e2e/auth.spec.ts`: 로그인, 리다이렉트 흐름 (3 tests)
- `e2e/mail-list.spec.ts`: 메일 목록, 네비게이션, 사이드바 (3 tests)
- `e2e/compose.spec.ts`: 모달, 수신자 입력, 제목 입력 (3 tests)
- `e2e/search.spec.ts`: 검색 필드 입력, 초기화 (3 tests)
- `e2e/message-view.spec.ts`: 메시지 클릭, 읽기 창, 폴더 (3 tests)
- `e2e/responsive.spec.ts`: 데스크톱/태블릿/모바일, 리사이즈 (4 tests)
- `e2e/features.spec.ts`: 캘린더, 조직도, 드라이브, 설정 (6 tests)
- `e2e/README.md`: 실행, 작성, CI, 문제 해결 가이드
- **총 25개 E2E 테스트 케이스** 완료

---

## ✅ TASK-094: 조직도 수신자 피커 + 그룹 자동완성

**STATUS: COMPLETE (2026-05-12 개선)**

### 최근 개선 사항 (Hierarchical Tree Implementation)

사용자 피드백: "조직도의 깊이가 표현되어 있지 않아" → 해결됨

- **OrgPickerModal 계층적 트리 렌더링 구현**
  - RenderOrgTree 컴포넌트로 재귀적 부모-자식 트리 구조 렌더링
  - 확장/축소 기능 (▸/▼ 인디케이터)
  - 루트 조직 자동 확장으로 초기 계층 구조 시각화
  - 검색 모드에서는 플랫 리스트 유지
  
- **시각적 계층 표현**
  - 깊이별 들여쓰기 (padding): `12px + depth * 16px`
  - 깊이별 스타일 분화 (폰트 크기, 색상, 배경색)
  - 확장 가능 항목 명시 (children count에 따라)

- **테스트 결과**
  - E2E 테스트: 24/25 통과 (1개 사전 존재하는 auth 테스트만 실패)
  - 모든 기능 작동 확인: 조직 선택, 멤버 표시, 검색, 주소록 탭

**STATUS: COMPLETE**

---

## ✅ TASK-163: 사용자 웹메일 베타 검증 — 개발 시드 bootstrap 복구

**STATUS: COMPLETE**

### 배경

개발 환경에서 백엔드 컨테이너와 PostgreSQL은 정상 기동되었지만, fresh DB에 `scripts/seed_dev_beta.sh`를 적용하면 베타 도메인과 로그인 사용자가 먼저 생성되지 않아 사용자/주소/조직/메시지 시드가 FK 오류로 중단됐다.
사용자 웹메일과 관리자 콘솔 기능 테스트를 빠르게 반복하려면 개발 시드가 테넌트, 도메인, 베타 로그인 사용자, 기본 폴더를 스스로 보장해야 한다.

### 구현 대상

- `scripts/seed_dev_data.sql`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] `고구마컴퍼니` 회사 시드가 사용자/조직 시드보다 먼저 보장된다.
- [x] `parkjw.org` 도메인 시드가 사용자/주소 시드보다 먼저 보장된다.
- [x] 웹메일 베타 로그인 사용자 `pjw@parkjw.org / pass1234`가 seed 내에서 생성된다.
- [x] 웹메일 메시지 시드가 참조하는 `pjw` Inbox 폴더가 seed 내에서 생성된다.
- [x] 추가 베타 사용자 16명 모두 기본 시스템 폴더를 갖는다.
- [x] 개발 문서를 함께 갱신한다.

### 검증

- `./scripts/seed_dev_beta.sh` 통과
- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- 웹메일 프론트엔드 smoke test 예정

### 다음 태스크

TASK-164: 사용자 웹메일 베타 검증 — 백엔드/프론트엔드 smoke test

---

## ✅ TASK-164: 사용자 웹메일 베타 검증 — 시드 조직도/주소록 및 smoke 결함 보강

**STATUS: COMPLETE**

### 배경

개발 환경 smoke test에서 메일 폴더/읽음 처리/조직도/주소록 fixture가 베타 검증 기준으로 충분히 단단하지 않다는 점이 확인됐다.
시드는 백엔드의 표준 폴더 생성 규칙과 충돌하지 않아야 하며, 조직도와 주소록은 관리자 콘솔/사용자 웹메일 양쪽 테스트에 바로 사용할 수 있을 만큼 풍부해야 한다.

### 구현 대상

- `scripts/seed_dev_data.sql`
- `internal/maildb/messages.go`
- `apps/webmail/src/app/mail/page.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] seed가 백엔드 자동 생성 표준 시스템 폴더(`/Inbox` 등)를 우선 사용한다.
- [x] 이전 고정 ID 시스템 폴더와 메시지 참조를 표준 폴더로 정리한다.
- [x] 모든 내부 베타 사용자 21명이 조직 조회 fixture에 포함된다.
- [x] pjw 주소록에 내부 직원 20명 전체가 들어간다.
- [x] 외부 협력사 주소록과 풍부한 vCard 연락처를 추가한다.
- [x] 사라진 폴더 ID를 들고 있는 웹메일 세션은 유효한 Inbox로 복구된다.
- [x] 단건/대량/스레드 flag 업데이트 SQL이 PostgreSQL 타입 추론 오류 없이 동작한다.
- [x] 개발 문서를 함께 갱신한다.

### 검증

- `./scripts/seed_dev_beta.sh` 2회 연속 통과: users 21, org_members 21, contacts 24, inbox_msgs 7
- `go test ./...` 통과
- `pnpm type-check` in `apps/webmail` 통과
- 브라우저 smoke test: 표준 Inbox 복구 및 메일 리스트 표시 확인
- directory/users dev user_id fallback 확인: 21명 반환

### 다음 태스크

TASK-165: 사용자 웹메일 베타 검증 — 작성/전송/주소록 연동 smoke test

---

## ✅ TASK-165: 사용자 웹메일 베타 — 조직/주소록 발송 토큰 확장

**STATUS: COMPLETE**

### 배경

조직도/주소록 선택은 단순 사용자 선택 UI가 아니라 조직 발송 기능의 시작점이다.
조직 또는 주소록을 To/Cc/Bcc에 넣으면 실제 발송 시 해당 조직/주소록 소속 사용자가 수신자로 확장되어야 하며, 조직은 하위 조직 포함 여부를 명시적으로 제어할 수 있어야 한다.

### 구현 대상

- `apps/webmail/src/components/OrgPickerModal.tsx`
- `apps/webmail/src/components/ComposeModal.tsx`
- `internal/mailservice/service.go`
- `internal/mailservice/compose_contract.go`
- `internal/maildb/recipient_groups.go`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 조직도 선택 모달의 2번째 패널에 선택 조직, 하위 조직, 구성원을 함께 표시한다.
- [x] 조직 선택 시 하위 조직 포함 여부를 체크박스로 제어한다.
- [x] 조직 토큰을 To/Cc/Bcc에 추가할 수 있다.
- [x] 주소록 탭에서 주소록 전체를 To/Cc/Bcc에 추가할 수 있다.
- [x] 발송 직전 백엔드가 조직 토큰을 실제 사용자 이메일 목록으로 확장한다.
- [x] 발송 직전 백엔드가 주소록 토큰을 실제 연락처 이메일 목록으로 확장한다.
- [x] 초안 저장/초안 발송 경로에서도 토큰이 보존되고 발송 시 확장된다.
- [x] 확장된 수신자는 To → Cc → Bcc 순서로 중복 제거된다.
- [x] 개발 문서를 함께 갱신한다.

### 검증

- `go test ./...` 통과
- 조직 토큰 smoke: 재귀 CTE 보강
- 주소록 토큰 smoke: local mailstore root 자동 생성 보강
- `pnpm --dir apps/webmail type-check` 통과
- API smoke: 조직 토큰 발송 202 Accepted, 실제 To 9명 확장 확인
- API smoke: 주소록 토큰 발송 202 Accepted, 실제 To 4명 확장 확인

### 다음 태스크

TASK-166: 사용자 웹메일 베타 — 조직/주소록 발송 UX polish 및 전송 결과 표시

---

## ✅ TASK-166: 사용자 웹메일 베타 — 조직/주소록 수신자 표시 문구 정리

**STATUS: COMPLETE**

### 배경

조직/주소록 발송 토큰은 기능적으로 동작하지만, 수신자 선택 UI에 `[조직]`, `[하위 조직]`, `+ 하위 조직`, `[주소록]` 같은 시스템성 문구가 노출되면 실제 제품 사용감이 거칠어진다.
디자인 톤은 유지하면서 사용자가 선택하는 이름은 자연스러운 조직명/주소록명으로 보여야 한다.

### 구현 대상

- `apps/webmail/src/components/OrgPickerModal.tsx`
- `docs/CURRENT_STATUS.md`
- `docs/ACTIVE_TASK.md`

### 완료 조건

- [x] 조직 토큰 표시명에서 `[조직]` 접두어를 제거한다.
- [x] 하위 조직 행 표시명에서 `[하위 조직]` 접두어를 제거한다.
- [x] 하위 조직 포함 토큰 표시명에서 `+ 하위 조직` 접미어를 제거한다.
- [x] 주소록 토큰 표시명에서 `[주소록]` 접두어를 제거한다.
- [x] 내부 토큰과 백엔드 확장 동작은 유지한다.
- [x] 개발 문서를 함께 갱신한다.

### 검증

- `pnpm --dir apps/webmail type-check` 통과

### 다음 태스크

TASK-167: 사용자 웹메일 베타 — 조직/주소록 수신자 칩 UX polish
