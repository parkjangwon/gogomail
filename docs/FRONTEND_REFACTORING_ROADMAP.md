# Frontend Code Refactoring Roadmap

## Goal
Prevent webmail and console frontends from becoming bloated by systematically separating concerns into focused, reusable modules. All existing functionality must continue working.

## Pattern Established (Calendar Refactoring)

### Phase 1: Extract Utilities
- Create focused utility modules in `lib/` directory
- Each module contains related functions (date, parsing, formatting, etc.)
- Utilities are reusable across components and testable

**Completed:**
- `apps/webmail/src/lib/calendar/dateUtils.ts` (55 lines, 9 functions)
- `apps/webmail/src/lib/calendar/eventParser.ts` (94 lines, 2 types + 4 functions)

### Phase 2: Extract Sub-Components
- Move discrete UI components to feature directories
- Each component handles a specific, focused responsibility
- Import utilities instead of duplicating logic

**Completed:**
- `apps/webmail/src/components/calendar/MiniCalendar.tsx` (101 lines)
- `apps/webmail/src/components/calendar/QuickCreatePopover.tsx` (104 lines)
- `apps/webmail/src/components/calendar/EventPopover.tsx` (64 lines)

### Phase 3: Organize by Feature
- Create directories for each major feature (calendar, mail, compose, drive, contacts, settings)
- Group related components and utilities together
- Simplify imports with barrel exports (index.ts)

## Webmail Refactoring Status

### CalendarView.tsx Refactoring
- **Original**: 1887 lines
- **Current**: 1502 lines (-385 lines, -20.4%)
- **Remaining Components to Extract**:
  - MonthView (129 lines)
  - WeekView (157 lines)
  - DayView (100+ lines)
  - Subscription event parser (to eventParser.ts)
- **Target**: 700-800 lines (feature orchestrator only)

### Other Large Components (Priority Order)
1. **ComposeModal.tsx** (현재 1,429줄, 원본 1,795줄)
   - ✅ Extracted: `compose/ComposeEditorToolbar.tsx` (92줄) — TipTap 툴바
   - ✅ Extracted: `compose/ComposeAttachmentPanel.tsx` (84줄) — 첨부파일 패널
   - ✅ Extracted: `compose/toolbarBtnStyle.ts` — 툴바 버튼 스타일 공유 유틸
   - Remaining: RichTextEditor sub-component, RecipientInput component, composerUtils.ts

**Completed Extractions:**
- **DMPanel.tsx** (원본 약 900줄 → 현재 235줄)
  - ✅ Extracted: `dm/DMRoomList.tsx` (272줄) — 룸 목록 + 새 채팅
  - ✅ Extracted: `dm/DMMessageList.tsx` (204줄) — 메시지 목록
  - ✅ Extracted: `dm/DMComposer.tsx` (88줄) — 작성 영역
  - ✅ Extracted: `dm/DMDetailsPanel.tsx` (166줄) — 대화 상세
  - ✅ Extracted: `dm/DMOverlays.tsx` (94줄) — 이미지/붙여넣기 오버레이
  - ✅ Extracted: `dm/useDMPanel.ts` — 전체 DM 상태 관리 커스텀 훅
  - ✅ Extracted: `dm/types.ts` — `DMTFunction` 타입 정의

2. **ReadingPane.tsx** (1756 lines)
   - Extract: MessageHeader component
   - Extract: MessageBody component
   - Extract: MailActions component
   - Extract: messageUtils.ts (parsing, formatting helpers)

3. **SettingsView.tsx** (1691 lines)
   - Extract: SettingsForm component
   - Extract: PreferencePanel components
   - Extract: settingsUtils.ts (validation, serialization)

4. **DriveView.tsx** (1284 lines)
   - Extract: FileGrid/FileList components
   - Extract: FileUpload component
   - Extract: driveUtils.ts (sorting, filtering)

5. **MessageList.tsx** (1267 lines)
   - Extract: MessageRow component
   - Extract: ThreadViewer component
   - Extract: messageListUtils.ts (sorting, filtering, thread logic)

6. **FolderTree.tsx**, **ContactList.tsx**, etc. (smaller components)

### Organization Structure (Target)
```
apps/webmail/src/
├── components/
│   ├── mail/
│   │   ├── MessageList.tsx → 400 lines
│   │   ├── MessageRow.tsx → 200 lines
│   │   ├── ReadingPane.tsx → 600 lines
│   │   ├── MessageHeader.tsx → 150 lines
│   │   ├── MessageBody.tsx → 250 lines
│   │   ├── MailActions.tsx → 120 lines
│   │   └── index.ts
│   ├── compose/
│   │   ├── ComposeModal.tsx → 400 lines
│   │   ├── RichTextEditor.tsx → 600 lines
│   │   ├── RecipientInput.tsx → 200 lines
│   │   ├── AttachmentList.tsx → 150 lines
│   │   └── index.ts
│   ├── calendar/
│   │   ├── CalendarView.tsx → 700 lines (orchestrator)
│   │   ├── MonthView.tsx → 129 lines
│   │   ├── WeekView.tsx → 157 lines
│   │   ├── DayView.tsx → 100 lines
│   │   ├── MiniCalendar.tsx → 101 lines
│   │   ├── QuickCreatePopover.tsx → 104 lines
│   │   ├── EventPopover.tsx → 64 lines
│   │   └── index.ts
│   ├── drive/
│   │   ├── DriveView.tsx → 400 lines
│   │   ├── FileGrid.tsx → 300 lines
│   │   ├── FileUpload.tsx → 200 lines
│   │   └── index.ts
│   ├── contacts/
│   ├── settings/
│   ├── ui/ (shared UI components)
│   └── AppLayout.tsx
├── lib/
│   ├── calendar/
│   │   ├── dateUtils.ts → 9 functions
│   │   ├── eventParser.ts → 2 types + 4 functions
│   │   └── index.ts
│   ├── mail/
│   │   ├── messageUtils.ts
│   │   ├── threadUtils.ts
│   │   └── index.ts
│   ├── compose/
│   │   ├── composerUtils.ts
│   │   ├── richTextUtils.ts
│   │   └── index.ts
│   └── ... (other feature utilities)
└── hooks/
    ├── useCalendar.ts
    ├── useMail.ts
    └── ... (feature hooks)
```

## Console Refactoring Roadmap

- **Current State**: 106 files, ~1.4M total
- **Approach**: Apply same pattern as webmail
- **Priority Components**:
  1. Admin dashboard (largest single file)
  2. Domain settings (settings management)
  3. User management (user listing, creation, updates)
  4. Audit logs (log viewing, filtering)
  5. Other modules

## Implementation Strategy

### For Each Large Component:
1. **Create utilities module** (`lib/<feature>/<topic>Utils.ts`)
   - Extract helper functions
   - Extract constants and types
   - Test utilities independently

2. **Create sub-components**
   - Identify logical sections
   - Extract each section to separate file
   - Use utilities from lib/

3. **Update main component**
   - Import sub-components
   - Import utilities
   - Reduce to 400-600 lines (orchestrator)

4. **Create index.ts barrel export**
   - Export main and sub-components
   - Simplify imports in parent components

5. **Test**
   - TypeScript compilation: `npx tsc --noEmit`
   - Verify functionality with browser
   - All existing features must work

### Estimated Effort
- **CalendarView completion** (3 remaining components): 2-3 commits
- **Other webmail components** (ComposeModal, ReadingPane, SettingsView, DriveView, MessageList): 10-15 commits
- **Console refactoring**: 15-20 commits
- **Total**: 30-40 commits to complete refactoring

## Success Criteria
- [ ] All webmail components < 800 lines (ComposeModal 1,429줄 — 진행 중)
- [ ] All console components < 1000 lines
- [x] DM components: DMPanel 235줄, 모든 서브컴포넌트 300줄 이하
- [ ] Utilities organized by feature in `lib/`
- [ ] Components organized by feature directories
- [ ] All functionality preserved and tested
- [ ] TypeScript checks pass
- [ ] Existing tests pass

## Progress Tracking
See `CURRENT_STATUS.md` for detailed progress updates.
