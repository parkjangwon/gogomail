# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## 현재 태스크

- **ID**: TASK-002
- **제목**: Phase 2-A — Runtime Config Store (PostgreSQL JSONB + LISTEN/NOTIFY)
- **배경**: 회사(Company) → 도메인(Domain) → 사용자(User) 3단 계층으로 설정이 상속·독립 운영된다.
  설정 변경이 재배포나 스키마 마이그레이션 없이 모든 프로세스에 즉시 반영된다.
- **구현 대상**: migration, internal/configstore, Admin API, propgate API

### 완료 조건

- [ ] Migration 0073: `runtime_config` 테이블, `companies.parent_id` 추가
- [ ] `internal/configstore.ConfigStore` 인터페이스 + `PostgresConfigStore` 구현 (LISTEN/NOTIFY)
- [ ] Admin API CRUD: `GET/POST/PUT/DELETE /admin/v1/companies/{id}/config/{key}`
- [ ] Admin API CRUD: `GET/POST/PUT/DELETE /admin/v1/domains/{id}/config/{key}`
- [ ] Admin API CRUD: `GET/POST/PUT/DELETE /admin/v1/users/{id}/config/{key}`
- [ ] Propagate API: `POST /admin/v1/companies/{id}/config/propagate?scope=subtree|children|domains`
- [ ] 생성 훅: 자회사/도메인 생성 시 직속 부모 설정 자동 복사
- [ ] 테스트: 트리 해결 순서, locked 차단, propagate 전파 범위, 생성 복사
- [ ] docs/CURRENT_STATUS.md 갱신

### 커밋 후 다음 태스크

`docs/BACKLOG.md`의 첫 번째 미완료 항목( `[ ]` )을 꺼낸다.
현재 다음 태스크: **TASK-002 — Phase 2-A Runtime Config Store**

---

## 루프 절차 (매 태스크마다 반복)

```
1. 이 파일 읽기
2. 실패하는 테스트 먼저 작성
3. 테스트 통과하도록 구현
4. go test ./... 실행
5. docs 업데이트
6. 위 체크리스트 전부 체크
7. git add (코드 + docs), git commit
8. go test ./... 통과 확인 후 git push origin main
9. 다음 태스크로 이 파일 교체
```
