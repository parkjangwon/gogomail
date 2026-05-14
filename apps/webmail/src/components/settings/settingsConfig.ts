export interface WebmailSettings {
  readMark: 'instant' | '2s' | 'manual';
  listDensity: 'default' | 'compact';
  defaultSort: 'newest' | 'oldest';
  quoteOnReply: boolean;
  signature: string;
  theme: 'light' | 'dark' | 'system';
  notifications: boolean;
  accentColor: string;
  locale: string;
  autoSaveDraft: boolean;
  sendDelay: 0 | 5 | 10 | 30;
  threadView: boolean;
  showPreview: boolean;
  inlineImages: boolean;
  externalImages: 'always' | 'ask' | 'never';
  fontSize: 'small' | 'medium' | 'large';
}

export const DEFAULT_SETTINGS: WebmailSettings = {
  readMark: 'instant',
  listDensity: 'default',
  defaultSort: 'newest',
  quoteOnReply: true,
  signature: '',
  theme: 'system',
  notifications: false,
  accentColor: '#2F6EE0',
  locale: 'ko',
  autoSaveDraft: true,
  sendDelay: 0,
  threadView: true,
  showPreview: true,
  inlineImages: true,
  externalImages: 'ask',
  fontSize: 'medium',
};

export type Category = 'mailbox' | 'compose' | 'theme' | 'notifications' | 'account' | 'security' | 'shortcuts' | 'advanced' | 'filters';

export type AccentPreset = {
  name: string;
  swatch: string;
  light: { accent: string; hover: string; subtle: string };
  dark: { accent: string; hover: string; subtle: string };
};

export const ACCENT_PRESETS: AccentPreset[] = [
  { name: '블루', swatch: '#2F6EE0', light: { accent: '#2F6EE0', hover: '#2560C8', subtle: '#EBF1FD' }, dark: { accent: '#5B8EF0', hover: '#6B9AF4', subtle: '#1E2B45' } },
  { name: '틸', swatch: '#0D9488', light: { accent: '#0D9488', hover: '#0F766E', subtle: '#CCFBF1' }, dark: { accent: '#14B8A6', hover: '#2DD4BF', subtle: '#1A3D38' } },
  { name: '보라', swatch: '#7C3AED', light: { accent: '#7C3AED', hover: '#6D28D9', subtle: '#EDE9FE' }, dark: { accent: '#A78BFA', hover: '#C4B5FD', subtle: '#2E1B4E' } },
  { name: '주황', swatch: '#EA580C', light: { accent: '#EA580C', hover: '#C2410C', subtle: '#FFEDD5' }, dark: { accent: '#FB923C', hover: '#FDBA74', subtle: '#451A03' } },
  { name: '핑크', swatch: '#DB2777', light: { accent: '#DB2777', hover: '#BE185D', subtle: '#FCE7F3' }, dark: { accent: '#F472B6', hover: '#FBCFE8', subtle: '#4A1032' } },
  { name: '그린', swatch: '#059669', light: { accent: '#059669', hover: '#047857', subtle: '#D1FAE5' }, dark: { accent: '#34D399', hover: '#6EE7B7', subtle: '#1A3B30' } },
];

export function applyAccent(swatch: string) {
  const preset = ACCENT_PRESETS.find((p) => p.swatch === swatch) ?? ACCENT_PRESETS[0];
  const id = 'webmail-accent-override';
  let el = document.getElementById(id) as HTMLStyleElement | null;
  if (!el) { el = document.createElement('style'); el.id = id; document.head.appendChild(el); }
  el.textContent = `:root { --color-accent: ${preset.light.accent}; --color-accent-hover: ${preset.light.hover}; --color-accent-subtle: ${preset.light.subtle}; } [data-theme="dark"] { --color-accent: ${preset.dark.accent}; --color-accent-hover: ${preset.dark.hover}; --color-accent-subtle: ${preset.dark.subtle}; }`;
  try { localStorage.setItem('webmail_accent', swatch); } catch { /* */ }
}

export function getInitialLocale(): string {
  if (typeof document === 'undefined') return 'ko';
  const match = document.cookie.match(/(?:^|;\s*)webmail_locale=([^;]+)/);
  return match?.[1] ?? 'ko';
}

export function loadSettings(): WebmailSettings {
  try {
    const raw = localStorage.getItem('webmail_settings');
    return { ...DEFAULT_SETTINGS, locale: getInitialLocale(), ...(raw ? JSON.parse(raw) : {}) };
  } catch { /* */ }
  return { ...DEFAULT_SETTINGS, locale: getInitialLocale() };
}

export function saveSettings(s: WebmailSettings) {
  try {
    localStorage.setItem('webmail_settings', JSON.stringify(s));
  } catch { /* */ }
}

export interface FilterCondition {
  field: 'from' | 'to' | 'cc' | 'subject' | 'body' | 'has_attachment' | 'is_unread' | 'size_larger' | 'size_smaller';
  matchType: 'contains' | 'not_contains' | 'equals' | 'starts_with' | 'ends_with' | 'regex';
  value: string;
}

export interface FilterAction {
  labelColor?: string;
  moveToFolder?: string;
  markRead?: boolean;
  markUnread?: boolean;
  markStarred?: boolean;
  markImportant?: boolean;
  skipInbox?: boolean;
  deleteMsg?: boolean;
  forwardTo?: string;
}

export interface FilterRule {
  id: string;
  name: string;
  enabled: boolean;
  logic: 'and' | 'or';
  conditions: FilterCondition[];
  action: FilterAction;
  stopProcessing?: boolean;
}

export const FILTER_RULES_KEY = 'webmail_filter_rules';

export function migrateRule(r: Record<string, unknown>): FilterRule {
  if (Array.isArray(r.conditions)) return { enabled: true, ...r } as unknown as FilterRule;
  return {
    id: (r.id as string) ?? Math.random().toString(36).slice(2),
    name: (r.name as string) ?? '',
    enabled: true,
    logic: 'and',
    conditions: [{ field: (r.field === 'any' ? 'from' : (r.field as FilterCondition['field'])) ?? 'from', matchType: 'contains', value: (r.value as string) ?? '' }],
    action: { labelColor: r.labelColor as string | undefined },
  };
}

export function loadFilterRules(): FilterRule[] {
  try { return (JSON.parse(localStorage.getItem(FILTER_RULES_KEY) ?? '[]') as Record<string, unknown>[]).map(migrateRule); } catch { return []; }
}

export function saveFilterRules(rules: FilterRule[]) {
  try { localStorage.setItem(FILTER_RULES_KEY, JSON.stringify(rules)); } catch { /* */ }
}

export const CATEGORIES: { id: Category; label: string }[] = [
  { id: 'mailbox', label: '메일함' },
  { id: 'compose', label: '메일 쓰기' },
  { id: 'filters', label: '필터' },
  { id: 'theme', label: '테마' },
  { id: 'notifications', label: '알림' },
  { id: 'account', label: '계정' },
  { id: 'security', label: '보안' },
  { id: 'shortcuts', label: '단축키' },
  { id: 'advanced', label: '고급' },
];

export const LABEL_COLORS = ['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#8b5cf6', '#ec4899', '#6b7280'];

export const createEmptyRule = (): Omit<FilterRule, 'id'> => ({ name: '', enabled: true, logic: 'and', conditions: [{ field: 'from', matchType: 'contains', value: '' }], action: {} });
