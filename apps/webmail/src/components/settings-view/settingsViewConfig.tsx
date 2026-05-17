import type { ReactNode } from 'react';
import {
  UserCircleIcon,
  SwatchIcon,
  BellIcon,
  ShieldCheckIcon,
  InformationCircleIcon,
  InboxIcon,
  BookOpenIcon,
  PencilSquareIcon,
  KeyIcon,
  FunnelIcon,
  CalendarDaysIcon,
  NoSymbolIcon,
  LockClosedIcon,
  EyeIcon,
  CircleStackIcon,
} from '@heroicons/react/24/outline';

export type SectionId =
  | 'account'
  | 'inbox'
  | 'reading'
  | 'compose'
  | 'filters'
  | 'storage'
  | 'blocked'
  | 'vacation'
  | 'privacy'
  | 'appearance'
  | 'notifications'
  | 'shortcuts'
  | 'security'
  | 'accessibility'
  | 'about';

export const NAV_ITEMS: { id: SectionId; label: string; icon: ReactNode }[] = [
  { id: 'account', label: '계정', icon: <UserCircleIcon style={{ width: 16, height: 16 }} /> },
  { id: 'inbox', label: '받은편지함', icon: <InboxIcon style={{ width: 16, height: 16 }} /> },
  { id: 'reading', label: '읽기', icon: <BookOpenIcon style={{ width: 16, height: 16 }} /> },
  { id: 'compose', label: '작성', icon: <PencilSquareIcon style={{ width: 16, height: 16 }} /> },
  { id: 'filters', label: '필터', icon: <FunnelIcon style={{ width: 16, height: 16 }} /> },
  { id: 'storage', label: '용량/백업', icon: <CircleStackIcon style={{ width: 16, height: 16 }} /> },
  { id: 'blocked', label: '차단 목록', icon: <NoSymbolIcon style={{ width: 16, height: 16 }} /> },
  { id: 'vacation', label: '자동 응답', icon: <CalendarDaysIcon style={{ width: 16, height: 16 }} /> },
  { id: 'privacy', label: '개인정보 보호', icon: <LockClosedIcon style={{ width: 16, height: 16 }} /> },
  { id: 'appearance', label: '외관', icon: <SwatchIcon style={{ width: 16, height: 16 }} /> },
  { id: 'notifications', label: '알림', icon: <BellIcon style={{ width: 16, height: 16 }} /> },
  { id: 'shortcuts', label: '단축키', icon: <KeyIcon style={{ width: 16, height: 16 }} /> },
  { id: 'security', label: '보안', icon: <ShieldCheckIcon style={{ width: 16, height: 16 }} /> },
  { id: 'accessibility', label: '접근성', icon: <EyeIcon style={{ width: 16, height: 16 }} /> },
  { id: 'about', label: '정보', icon: <InformationCircleIcon style={{ width: 16, height: 16 }} /> },
];

export const SHORTCUT_GROUPS = [
  { title: '전역', items: [['?', '단축키 도움말'], ['Cmd+K / Ctrl+K', '스팟라이트 검색'], ['/', '스팟라이트 열기'], ['[', '사이드바 접기/펼치기']] },
  { title: '앱 전환', items: [['g  m', '메일'], ['g  c', '캘린더'], ['g  a', '연락처'], ['g  d', '드라이브'], ['g  ,', '설정']] },
  { title: '메일 탐색', items: [['j / k', '다음/이전 메일'], ['↑ / ↓', '목록 이동'], ['Enter / o', '선택 메일 열기'], ['Space', '체크박스 선택'], ['Home / End', '첫/마지막 메일'], ['Ctrl+A', '전체 선택'], ['Esc', '닫기 / 해제']] },
  { title: '메일 동작', items: [['r', '회신'], ['a', '전체 회신'], ['f', '전달'], ['e', '보관'], ['v', '편지함으로 이동'], ['#', '삭제'], ['s', '별표'], ['m', '읽음 표시'], ['Shift+M', '읽지 않음'], ['z', '1시간 스누즈'], ['l', '라벨 순환'], ['!', '스팸']] },
  { title: '편지함 이동', items: [['g  i', '받은 편지함'], ['g  s', '보낸 편지함'], ['g  t', '휴지통'], ['g  p', '스팸 편지함']] },
  { title: '작성', items: [['c', '새 메일'], ['Ctrl+Enter', '전송'], ['Ctrl+S', '임시저장'], ['Esc', '닫기']] },
];
