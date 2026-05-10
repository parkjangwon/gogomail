# ACTIVE_TASK

> 에이전트는 이 파일만 읽고 구현을 시작한다.
> 완료 후 docs/NEXT_STEPS.md 백로그에서 다음 항목을 이 파일로 가져온다.

---

## ✅ TASK-085: Admin Console Frontend (Phase 1)

**STATUS: READY_TO_START**

### 배경

Phase 8-D (UI/UX & Settings)의 최종 단계로 Admin Console의 프론트엔드 완성:
- TASK-084 (Alerts & Notifications) 완료로 백엔드 모든 기능 구현 완료
- 이제 admin console의 2차, 3차 기능 페이지 개발 시작
- React Query 훅과 Cloudscape Design System 활용

### 구현 대상

#### 1. Dashboard & Analytics 페이지
- 전체 사용자 수, 도메인 수 통계
- 최근 7일 활동 차트
- API 사용률 그래프
- 보안 이벤트 요약

#### 2. Advanced Audit Logs UI
- 필터링 (기간, 사용자, 작업 유형)
- 페이지네이션 (50/100/200 행 선택)
- 내보내기 (CSV, JSON)
- 상세 보기 모달

#### 3. Organization Structure 시각화
- 조직 계층도 (트리 뷰)
- 사용자/그룹 관계도
- Drag-and-drop 계층 변경
- 벌크 작업 지원

#### 4. Export & Reports
- 정기 리포트 설정 (일일, 주간, 월간)
- 리포트 템플릿 커스터마이징
- 수신자 그룹 관리
- 리포트 다운로드 히스토리

#### 5. Role Management Advanced
- 커스텀 권한 조합 UI
- 권한 매트릭스 (리소스 × 액션)
- 역할 복제 및 버전 관리
- 권한 영향도 분석

### 완료 조건

- [ ] Dashboard 페이지 구현 및 스타일링
- [ ] Audit Logs 고급 필터링 UI
- [ ] Organization Structure 시각화 컴포넌트
- [ ] Export & Reports 설정 페이지
- [ ] Role Management 커스텀 권한 UI
- [ ] React Query 훅 모든 API 연동 테스트
- [ ] Cloudscape 컴포넌트 일관성 검증
- [ ] 접근성 검사 (WCAG 2.1 AA)
- [ ] 반응형 레이아웃 (모바일 미지원 확인)
- [ ] 성능 최적화 (번들 크기, 로딩 시간)
- [ ] E2E 테스트 작성 (주요 워크플로우)
- [ ] docs/CURRENT_STATUS.md 갱신
- [ ] git add + commit + push

### 루프 절차

```
1. 이 파일 읽기
2. 각 페이지별 컴포넌트 설계
3. React Query 훅 작성
4. UI 구현 및 스타일링
5. API 연동 테스트
6. 접근성 및 성능 검증
7. E2E 테스트 작성
8. docs 업데이트
9. git commit + push
10. TASK-086으로 이동
```

### 다음 태스크
TASK-086: Admin Console Frontend (Phase 2)
