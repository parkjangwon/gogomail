# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ⏳ TASK-086: Admin Console Frontend (Phase 2)

**STATUS: IN_PROGRESS**

### 배경

Phase 8-D (UI/UX & Settings)의 심화 단계로 Admin Console의 고급 기능 구현:
- TASK-085 (Dashboard, Audit Logs, Organization, Reports, Roles) 완료
- 이제 정책, 보안, 통합 관리 기능 추가
- API Key 관리, MFA 정책, SSO 설정 페이지 개발

### 구현 대상

#### 1. API Key Management 페이지
- API 키 생성/삭제
- 키 로테이션
- CIDR 허용 목록 설정
- 사용 통계 (요청 수, 마지막 사용)
- 만료 날짜 설정

#### 2. MFA & 보안 정책
- MFA 모드 설정 (disabled/optional/required)
- TOTP 설정 강제 (Grace period 설정)
- Recovery codes 관리
- 세션 타임아웃 정책

#### 3. SSO & Identity Provider 관리
- SSO 제공자 설정 (LDAP, OIDC)
- 프로비저닝 옵션 설정
- 속성 매핑 (email, name, groups)
- 테스트 연결 기능

#### 4. 도메인 & 테넌트 관리
- 도메인 추가/삭제/편집
- 기본 도메인 설정
- 도메인별 설정 상속
- 도메인 상태 모니터링

#### 5. 정책 & 준수
- 암호 정책 (길이, 복잡성, 만료)
- 로그인 정책 (IP 제한, 로그인 시간)
- 감사 정책 레벨 설정
- 데이터 보호 규정 (GDPR, HIPAA)

### 완료 조건

- [ ] API Key Management 페이지 구현
- [ ] MFA & Security Policy 페이지 구현
- [ ] SSO & Identity Provider 설정 페이지
- [ ] Domain & Tenant Management 페이지
- [ ] Policy & Compliance 페이지
- [ ] React Query 훅 구현 및 테스트
- [ ] API 엔드포인트 연동 검증
- [ ] 폼 검증 및 에러 처리
- [ ] 접근성 검사 (WCAG 2.1 AA)
- [ ] 성능 최적화
- [ ] E2E 테스트 (주요 워크플로우)
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] git commit + push

### 루프 절차

```
1. 이 파일 읽기
2. 각 페이지별 훅 작성
3. UI 구현 및 스타일링
4. API 연동 테스트
5. 검증 및 에러 처리
6. 접근성 & 성능 검증
7. E2E 테스트 작성
8. docs 업데이트
9. git commit + push
10. TASK-087로 이동
```

### 다음 태스크
TASK-087: Admin Console Frontend (Phase 3)
