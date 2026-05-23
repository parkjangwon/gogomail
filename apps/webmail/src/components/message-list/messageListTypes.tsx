import { MessageSummary, Folder } from '@/lib/api';
import { ReactNode } from 'react';

export type FilterMode = 'all' | 'unread' | 'read' | 'starred' | 'unstarred' | 'attachment' | 'noattachment';

export const AVATAR_COLORS = ['#2F6EE0', '#0D9488', '#7C3AED', '#EA580C', '#DB2777', '#059669', '#D97706', '#DC2626'];

export type CategoryTab = 'all' | '알림' | '뉴스레터' | '주문' | '청구서';

export const CATEGORY_TABS: { id: CategoryTab; label: string }[] = [
  { id: 'all', label: '전체' },
  { id: '알림', label: '알림' },
  { id: '뉴스레터', label: '뉴스레터' },
  { id: '주문', label: '주문' },
  { id: '청구서', label: '청구서' },
];

export function avatarColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) & 0xffffff;
  return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

export function highlight(text: string, query: string): ReactNode {
  if (!query.trim() || !text) return text;
  const words = query.trim().split(/\s+/).filter(Boolean);
  const pattern = new RegExp(`(${words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
  const parts = text.split(pattern);
  return parts.map((part, i) => (
    <mark key={i} style={{ background: '#fef08a', color: 'inherit', borderRadius: '2px', padding: '0 1px' }}>
      {part}
    </mark>
  ));
}

export function readingTimeLabel(preview: string, t?: (key: string, values?: Record<string, unknown>) => string): string | null {
  if (!preview) return null;
  const words = Math.round((preview.length / 5) * 8);
  const mins = Math.max(1, Math.round(words / 200));
  if (t) {
    return mins === 1 ? t('misc.messageListTypes.readingApprox1') : t('misc.messageListTypes.readingApproxN', { n: mins });
  }
  return mins === 1 ? '~1분' : `~${mins}분`;
}

export function getAutoCategory(fromAddr: string, subject: string, t?: (key: string) => string): { label: string; color: string } | null {
  const from = (fromAddr ?? '').toLowerCase();
  const subj = (subject ?? '').toLowerCase();
  const tr = (k: string, fallback: string) => (t ? t(k) : fallback);
  if (/no.?reply|noreply|automated?@|do.not.reply|donotreply/.test(from)) return { label: tr('misc.messageListTypes.noticeCategory', '알림'), color: '#6b7280' };
  if (/newsletter|hello@|hi@|info@|updates?@|news@|digest@/.test(from)) return { label: tr('misc.messageListTypes.newsletterCategory', '뉴스레터'), color: '#3b82f6' };
  if (/주문|order|purchase|receipt|배송|shipped|delivered|tracking/.test(subj)) return { label: tr('misc.messageListTypes.orderCategory', '주문'), color: '#16a34a' };
  if (/invoice|청구|영수증|payment|결제|billing/.test(subj)) return { label: tr('misc.messageListTypes.invoiceCategory', '청구서'), color: '#f97316' };
  return null;
}

export function formatDate(receivedAt: string, t?: (key: string, values?: Record<string, unknown>) => string): string {
  const date = new Date(receivedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return t ? t('misc.messageListTypes.justNow') : '방금 전';
  if (diffMins < 60) return t ? t('misc.messageListTypes.minutesAgo', { n: diffMins }) : `${diffMins}분 전`;
  if (diffHours < 12 && date.getDate() === now.getDate()) return t ? t('misc.messageListTypes.hoursAgo', { n: diffHours }) : `${diffHours}시간 전`;
  if (diffDays === 0) {
    return new Intl.DateTimeFormat('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false }).format(date);
  }
  if (diffDays < 7) {
    return new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(date);
  }
  if (date.getFullYear() === now.getFullYear()) {
    return new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric' }).format(date);
  }
  return new Intl.DateTimeFormat('ko-KR', { year: 'numeric', month: 'numeric', day: 'numeric' }).format(date);
}

export interface MessageListProps {
  messages: MessageSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  loading?: boolean;
  emptyLabel?: string;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  onStar?: (id: string, starred: boolean) => void;
  onBulkDelete?: (ids: string[]) => void;
  onDeleteMessage?: (id: string) => void;
  onBulkMarkRead?: (ids: string[]) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
  isMobile?: boolean;
  onOpenSidebar?: () => void;
  paneWidth?: number;
  fullWidth?: boolean;
  bottomLayout?: boolean;
  onContextMenuMessage?: (id: string, x: number, y: number) => void;
  onMarkAllRead?: () => void;
  emptyFolderLabel?: string;
  onEmptyFolder?: () => void;
  folders?: Folder[];
  onBulkMove?: (ids: string[], folderId: string) => void;
  searchQuery?: string;
  onBulkRestore?: (ids: string[]) => void;
  onBulkLabel?: (ids: string[], color: string | null) => void;
  onBulkStar?: (ids: string[], starred: boolean) => void;
  onArchiveMessage?: (id: string) => void;
  onToggleReadMessage?: (id: string, read: boolean) => void;
  onSnoozeMessage?: (id: string, until: Date) => void;
  onPinMessage?: (id: string) => void;
  pinnedIds?: Set<string>;
  importantIds?: Set<string>;
  messageLabels?: Record<string, string>;
  userEmail?: string;
  showPreview?: boolean;
  showCategoryTabs?: boolean;
}

export interface MessageRowProps {
  message: MessageSummary;
  isSelected: boolean;
  isBulkChecked: boolean;
  onSelect: (id: string) => void;
  onStar?: (id: string, starred: boolean) => void;
  onToggleBulk: (id: string, shiftKey?: boolean) => void;
  onContextMenu?: (id: string, x: number, y: number) => void;
  searchQuery?: string;
  compact?: boolean;
  onDelete?: (id: string) => void;
  onArchiveRow?: (id: string) => void;
  onHoverDelete?: (id: string) => void;
  onHoverArchive?: (id: string) => void;
  onHoverToggleRead?: (id: string, read: boolean) => void;
  onHoverSnooze?: (id: string, until: Date) => void;
  onHoverPin?: (id: string) => void;
  isPinned?: boolean;
  threadCount?: number;
  labelColor?: string;
  userEmail?: string;
  showPreview?: boolean;
  hasNote?: boolean;
  isImportant?: boolean;
  folderLabel?: string;
  onAvatarEnter?: (name: string, addr: string, rect: DOMRect) => void;
  onAvatarLeave?: () => void;
  onHoverChange?: (id: string | null) => void;
}
