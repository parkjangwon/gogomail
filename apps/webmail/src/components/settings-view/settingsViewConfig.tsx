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
  CommandLineIcon,
  ShieldExclamationIcon,
  LockClosedIcon,
  EyeIcon,
  CircleStackIcon,
  FolderIcon,
  UserGroupIcon,
} from '@heroicons/react/24/outline';

export type SectionId =
  | 'account'
  | 'inbox'
  | 'reading'
  | 'compose'
  | 'contacts'
  | 'drive'
  | 'filters'
  | 'storage'
  | 'blocked'
  | 'vacation'
  | 'privacy'
  | 'appearance'
  | 'notifications'
  | 'shortcuts'
  | 'security'
  | 'mcp'
  | 'accessibility'
  | 'about';

// labelKey references a key in the `settingsView` translation namespace.
export const NAV_ITEMS: { id: SectionId; labelKey: string; icon: ReactNode }[] = [
  { id: 'account', labelKey: 'navAccount', icon: <UserCircleIcon style={{ width: 16, height: 16 }} /> },
  { id: 'compose', labelKey: 'navCompose', icon: <PencilSquareIcon style={{ width: 16, height: 16 }} /> },
  { id: 'reading', labelKey: 'navReading', icon: <BookOpenIcon style={{ width: 16, height: 16 }} /> },
  { id: 'inbox', labelKey: 'navInbox', icon: <InboxIcon style={{ width: 16, height: 16 }} /> },
  { id: 'contacts', labelKey: 'navContacts', icon: <UserGroupIcon style={{ width: 16, height: 16 }} /> },
  { id: 'drive', labelKey: 'navDrive', icon: <FolderIcon style={{ width: 16, height: 16 }} /> },
  { id: 'filters', labelKey: 'navFilters', icon: <FunnelIcon style={{ width: 16, height: 16 }} /> },
  { id: 'vacation', labelKey: 'navVacation', icon: <CalendarDaysIcon style={{ width: 16, height: 16 }} /> },
  { id: 'blocked', labelKey: 'navSpam', icon: <ShieldExclamationIcon style={{ width: 16, height: 16 }} /> },
  { id: 'storage', labelKey: 'navStorage', icon: <CircleStackIcon style={{ width: 16, height: 16 }} /> },
  { id: 'notifications', labelKey: 'navNotifications', icon: <BellIcon style={{ width: 16, height: 16 }} /> },
  { id: 'appearance', labelKey: 'navAppearance', icon: <SwatchIcon style={{ width: 16, height: 16 }} /> },
  { id: 'privacy', labelKey: 'navPrivacy', icon: <LockClosedIcon style={{ width: 16, height: 16 }} /> },
  { id: 'security', labelKey: 'navSecurity', icon: <ShieldCheckIcon style={{ width: 16, height: 16 }} /> },
  { id: 'mcp', labelKey: 'navMcp', icon: <CommandLineIcon style={{ width: 16, height: 16 }} /> },
  { id: 'accessibility', labelKey: 'navAccessibility', icon: <EyeIcon style={{ width: 16, height: 16 }} /> },
  { id: 'shortcuts', labelKey: 'navShortcuts', icon: <KeyIcon style={{ width: 16, height: 16 }} /> },
  { id: 'about', labelKey: 'navAbout', icon: <InformationCircleIcon style={{ width: 16, height: 16 }} /> },
];

// titleKey and descKey reference keys in the `settingsView` translation namespace.
export const SHORTCUT_GROUPS: { titleKey: string; items: [string, string][] }[] = [
  { titleKey: 'shortcutGlobal', items: [['?', 'scShortcutHelp'], ['Cmd+K / Ctrl+K', 'scSpotlightSearch'], ['[', 'scSidebarToggle'], ['b', 'scNotificationCenter'], ['`', 'scDM']] },
  { titleKey: 'shortcutAppSwitch', items: [['g  m', 'scMail'], ['g  c', 'scCalendar'], ['g  a', 'scContacts'], ['g  d', 'scDrive'], ['g  ,', 'scSettings']] },
  { titleKey: 'shortcutMailNav', items: [['j / k', 'scNextPrev'], ['↑ / ↓', 'scListMove'], ['o', 'scOpenSelected'], ['Space', 'scCheckboxSelect'], ['Home / End', 'scFirstLast'], ['Ctrl+A', 'scSelectAll'], ['Esc', 'scCloseUnset']] },
  { titleKey: 'shortcutMailAction', items: [['r', 'scReply'], ['a', 'scReplyAll'], ['f', 'scForward'], ['e', 'scArchive'], ['v', 'scMoveToFolder'], ['#', 'scDelete'], ['m', 'scMarkRead'], ['Shift+M', 'scMarkUnread'], ['z', 'scSnooze1h'], ['l', 'scLabelCycle'], ['!', 'scSpam']] },
  { titleKey: 'shortcutMailbox', items: [['g  i', 'scInbox'], ['g  s', 'scSent'], ['g  t', 'scTrash'], ['g  p', 'scSpamFolder']] },
  { titleKey: 'shortcutCompose', items: [['c', 'scNewMail'], ['Ctrl+Enter', 'scSend'], ['Ctrl+S', 'scSaveDraft'], ['Esc', 'scClose']] },
  { titleKey: 'shortcutCalendar', items: [['d', 'scCalendarDay'], ['w', 'scCalendarWeek'], ['m', 'scCalendarMonth'], ['t', 'scCalendarToday'], ['← / →', 'scCalendarPrevNext']] },
  { titleKey: 'shortcutContacts', items: [['j / k', 'scContactNextPrev'], ['c', 'scContactCompose'], ['Del / Backspace', 'scContactDelete'], ['Ctrl+A / Cmd+A', 'scContactSelectAll'], ['Esc', 'scContactClearSelection']] },
];
