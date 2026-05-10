# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ⏳ TASK-087: Admin Console Frontend (Phase 3)

**STATUS: IN_PROGRESS**

### 배경

Admin Console 완성 단계:
- TASK-085 (Dashboard, Audit, Org, Reports, Roles) ✅
- TASK-086 (API Keys, Security, SSO, Domains, Compliance) ✅  
- 마지막 Phase: 통합, 네비게이션, E2E 테스트

### 구현 대상

#### 1. Navigation & Sidebar
- 상황에 따른 메뉴 표시 (Company Admin/Super Admin)
- 권한별 페이지 접근 제한
- Active 페이지 하이라이트
- 모바일 반응형 네비게이션

#### 2. 통합 & 레이아웃
- 일관된 헤더 구성 (로고, 사용자 정보, 알림)
- 공통 푸터 (버전, 문서 링크)
- 권한 기반 페이지 표시
- 로딩 및 에러 상태 처리

#### 3. 성능 최적화
- Code splitting & lazy loading
- 번들 크기 최적화
- 이미지 최적화
- 네트워크 요청 캐싱

#### 4. E2E 테스트 & 검증
- 전체 워크플로우 E2E 테스트
  * 로그인 → 대시보드 → 각 기능 페이지
  * 데이터 생성/편집/삭제
  * 권한 검증
- 스크린샷 캡처
- xlsx 결과 문서 작성 (한글)

#### 5. 접근성 & 성능
- WCAG 2.1 AA 준수
- 성능 지표 (LCP, FID, CLS)
- SEO 기본 설정

### 완료 조건

- [ ] Sidebar 네비게이션 구현
- [ ] 권한 기반 메뉴 필터링
- [ ] 공통 헤더/푸터 구현
- [ ] 로딩 상태 처리
- [ ] 에러 처리 및 표시
- [ ] Code splitting 설정
- [ ] 성능 최적화
- [ ] WCAG 검사 통과
- [ ] E2E 테스트 실행
- [ ] 스크린샷 캡처
- [ ] xlsx 테스트 결과 문서
- [ ] docs/CURRENT_STATUS.md 최종 갱신
- [ ] git commit + push

### 다음 단계

TASK-088: 웹페이지 실제 E2E 테스트 및 검증 (모든 기능)

### 루프 절차

```
1. 이 파일 읽기
2. Navigation & Sidebar 구현
3. Layout 통합
4. 성능 최적화
5. 접근성 검사
6. E2E 테스트 작성 및 실행
7. 스크린샷 캡처
8. xlsx 결과 문서 작성
9. docs 최종 업데이트
10. git commit + push
11. 백엔드+프론트엔드 모두 띄워서 웹에서 직접 전체 기능 E2E 테스트 진행
```

### 다음 태스크
TASK-088: Full System E2E Testing (웹페이지 직접 테스트)
