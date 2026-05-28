import { useState, useEffect } from 'react';
import { getPreferences, setPreferences, getUserProfile, getFolderStats, exportFolderEml, exportFolderZip, type FolderStats, type WebmailPreferences } from '@/lib/api';
import { ReadMark, ExternalImages, SendDelay, Theme, FontSize, FilterRule, migrateFilterRule, loadFilterRules, saveFilterRules } from '@/lib/settings/settingsUtils';
import { loadWmSettings, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';
import { normalizeEmailTemplates, saveLocalEmailTemplates, loadLocalEmailTemplates } from '@/lib/emailTemplates';
import { setWebmailAvatar } from '@/lib/webmailAvatar';
import { type BackupState } from '@/components/settings-view/SettingsStorageSection';
import { useSettingsAccount } from '@/components/settings-view/useSettingsAccount';
import { useSettingsNotifications } from '@/components/settings-view/useSettingsNotifications';
import { useSettingsTemplates } from '@/components/settings-view/useSettingsTemplates';

const BADGE_COUNT_MODE_KEY = 'webmail_badge_count_mode';
const BROWSER_NOTIF_ENABLED_KEY = 'webmail_browser_notifications_enabled';
type ContactsSort = 'name' | 'email' | 'company';
type ContactsDensity = 'comfortable' | 'compact';
type DriveSort = 'typeName' | 'name' | 'updated' | 'size';

export interface UseSettingsPrefsParams {
  userEmail?: string;
  userName?: string;
  activeSection?: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
  router: { push: (href: string) => void };
}

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
export function useSettingsPrefs({ userEmail: _userEmail, userName, activeSection, t, router }: UseSettingsPrefsParams) {
  // Sub-hooks
  const account = useSettingsAccount({ t, router });
  const notifications = useSettingsNotifications({ t });
  const tplState = useSettingsTemplates();

  // Inbox
  const [convMode, setConvMode] = useState(true);
  const [compact, setCompact] = useState(false);
  const [showPreview, setShowPreview] = useState(true);
  const [refreshInterval, setRefreshInterval] = useState<30 | 60 | 300>(30);
  const [importanceMarkers, setImportanceMarkers] = useState(true);
  const [groupByDate, setGroupByDate] = useState(true);
  const [focusMode, setFocusMode] = useState(false);
  const [swipeLeft, setSwipeLeft] = useState<'archive' | 'delete' | 'snooze' | 'star'>('archive');
  const [swipeRight, setSwipeRight] = useState<'archive' | 'delete' | 'snooze' | 'star'>('star');

  // Contacts
  const [contactsSort, setContactsSort] = useState<ContactsSort>('name');
  const [contactsDensity, setContactsDensity] = useState<ContactsDensity>('comfortable');
  const [contactsShowCompany, setContactsShowCompany] = useState(true);

  // Drive
  const [driveSort, setDriveSort] = useState<DriveSort>('typeName');

  // Reading
  const [readMark, setReadMark] = useState<ReadMark>('instant');
  const [externalImages, setExternalImages] = useState<ExternalImages>('ask');
  const [inlineImagePreview, setInlineImagePreview] = useState(true);
  const [smartReplySuggestions, setSmartReplySuggestions] = useState(true);
  const [showReadingTime, setShowReadingTime] = useState(true);
  const [readingPanePosition, setReadingPanePosition] = useState<'right' | 'bottom' | 'hidden'>('right');

  // Compose
  const [sendDelay, setSendDelay] = useState<SendDelay>(0);
  const [quoteOnReply, setQuoteOnReply] = useState(true);
  const [fontSize, setFontSize] = useState<FontSize>('medium');
  const [ccSelf, setCcSelf] = useState(false);
  const [defaultBcc, setDefaultBcc] = useState('');
  const [confirmBeforeSend, setConfirmBeforeSend] = useState(false);
  const [spellCheck, setSpellCheck] = useState(true);

  // Appearance
  const [theme, setTheme] = useState<Theme>('light');
  const [accent, setAccent] = useState('#2563eb');
  const [customAccent, setCustomAccent] = useState('');

  // Filters
  const [filterRules, setFilterRules] = useState<FilterRule[]>([]);

  // Privacy
  const [blockTrackingPixels, setBlockTrackingPixels] = useState(true);
  const [requestReadReceipt, setRequestReadReceipt] = useState(false);
  const [linkPreview, setLinkPreview] = useState(true);
  const [followUpDays, setFollowUpDays] = useState<0 | 1 | 3 | 7>(0);

  // Blocked senders / Spam settings
  const [blockedSenders, setBlockedSenders] = useState<string[]>([]);
  const [blockedMeta, setBlockedMeta] = useState<Record<string, string>>({});
  const [newBlockedInput, setNewBlockedInput] = useState('');
  const [blockedSearch, setBlockedSearch] = useState('');
  const [blockedPage, setBlockedPage] = useState(0);
  const [spamAutoDeleteDays, setSpamAutoDeleteDays] = useState<number>(30);
  const [spamAutoBlock, setSpamAutoBlock] = useState(true);
  // Allowed senders (allowlist)
  const [allowedSenders, setAllowedSenders] = useState<string[]>([]);
  const [allowedMeta, setAllowedMeta] = useState<Record<string, string>>({});
  const [newAllowedInput, setNewAllowedInput] = useState('');
  const [allowedSearch, setAllowedSearch] = useState('');
  const [allowedPage, setAllowedPage] = useState(0);

  // Vacation responder
  const [vacEnabled, setVacEnabled] = useState(false);
  const [vacStartDate, setVacStartDate] = useState('');
  const [vacEndDate, setVacEndDate] = useState('');
  const [vacSubject, setVacSubject] = useState('');
  const [vacBody, setVacBody] = useState('');
  const [vacSaved, setVacSaved] = useState(false);

  // Accessibility
  const [reducedMotion, setReducedMotion] = useState(false);
  const [highContrast, setHighContrast] = useState(false);
  const [largerClickTargets, setLargerClickTargets] = useState(false);
  const [screenReaderMode, setScreenReaderMode] = useState(false);
  const [fontFamily, setFontFamily] = useState<'system' | 'serif' | 'mono'>('system');
  const [colorBlindMode, setColorBlindMode] = useState<'none' | 'deuteranopia' | 'protanopia' | 'tritanopia'>('none');
  const [alwaysFocusRing, setAlwaysFocusRing] = useState(false);
  const [underlineLinks, setUnderlineLinks] = useState(false);
  const [dyslexiaMode, setDyslexiaMode] = useState(false);
  const [uiFontSize, setUiFontSize] = useState<'sm' | 'md' | 'lg' | 'xl'>('md');
  const [lineSpacing, setLineSpacing] = useState<'normal' | 'relaxed' | 'loose'>('normal');
  const [letterSpacing, setLetterSpacing] = useState<'normal' | 'wide'>('normal');

  // Timezone
  const [timezone, setTimezone] = useState<string>(() => {
    try { return localStorage.getItem('webmail_timezone') || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'; } catch { return 'UTC'; }
  });

  // Storage / Backup
  const [folderStats, setFolderStats] = useState<FolderStats[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  const [backupStates, setBackupStates] = useState<Record<string, BackupState>>({});

  // Server-side preferences sync
  const [prefsLoaded, setPrefsLoaded] = useState(false);

  // ── Load server preferences (overlay over localStorage on mount) ──────────────
  useEffect(() => {
    getUserProfile().then((p) => {
      if (p) {
        account.setProfile(p);
        account.setRecoveryEmail(p.recovery_email ?? '');
        account.setAvatarUrl(p.avatar_url ?? '');
        setWebmailAvatar(p.avatar_url ?? '');
      }
    }).catch(() => {});
    getPreferences().then((prefs: WebmailPreferences) => {
      try {
        if (prefs.settings) {
          const s = prefs.settings;
          if (s.readMark) setReadMark(s.readMark as ReadMark);
          if (s.externalImages) setExternalImages(s.externalImages as ExternalImages);
          if (s.sendDelay !== undefined) setSendDelay(s.sendDelay as SendDelay);
          if (s.showPreview !== undefined) setShowPreview(s.showPreview as boolean);
          if (s.compact !== undefined) setCompact(s.compact as boolean);
          if (s.convMode !== undefined) setConvMode(s.convMode as boolean);
          if (s.quoteOnReply !== undefined) setQuoteOnReply(s.quoteOnReply as boolean);
          if (s.fontSize) setFontSize(s.fontSize as FontSize);
          if (s.inlineImagePreview !== undefined) setInlineImagePreview(s.inlineImagePreview as boolean);
          if (s.blockTrackingPixels !== undefined) setBlockTrackingPixels(s.blockTrackingPixels as boolean);
          if (s.requestReadReceipt !== undefined) setRequestReadReceipt(s.requestReadReceipt as boolean);
          if (s.linkPreview !== undefined) setLinkPreview(s.linkPreview as boolean);
          if (s.focusMode !== undefined) setFocusMode(s.focusMode as boolean);
          if (s.swipeLeft) setSwipeLeft(s.swipeLeft as typeof swipeLeft);
          if (s.swipeRight) setSwipeRight(s.swipeRight as typeof swipeRight);
          if (s.refreshInterval) setRefreshInterval(s.refreshInterval as 30 | 60 | 300);
          if (s.contactsSort) setContactsSort(s.contactsSort as ContactsSort);
          if (s.contactsDensity) setContactsDensity(s.contactsDensity as ContactsDensity);
          if (s.contactsShowCompany !== undefined) setContactsShowCompany(s.contactsShowCompany as boolean);
          if (s.driveSort) setDriveSort(s.driveSort as DriveSort);
          if (s.browserNotificationsEnabled !== undefined) {
            const enabled = s.browserNotificationsEnabled as boolean;
            notifications.setBrowserNotificationsEnabled(enabled);
            try { localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, enabled ? 'true' : 'false'); } catch { /* ignore */ }
          }
          if (s.notifSound !== undefined) notifications.setNotifSound(s.notifSound as boolean);
          if (s.notifDetail) notifications.setNotifDetail(s.notifDetail as 'sender' | 'subject' | 'preview');
          if (s.badgeCountMode === 'all' || s.badgeCountMode === 'none' || s.badgeCountMode === 'unread') {
            notifications.setBadgeCountMode(s.badgeCountMode);
            try { localStorage.setItem(BADGE_COUNT_MODE_KEY, s.badgeCountMode); } catch { /* ignore */ }
          }
          if (s.dndEnabled !== undefined) notifications.setDndEnabled(s.dndEnabled as boolean);
          if (s.dndStart) notifications.setDndStart(s.dndStart as string);
          if (s.dndEnd) notifications.setDndEnd(s.dndEnd as string);
        }
        if (prefs.filter_rules) {
          const serverRules = (prefs.filter_rules as Record<string, unknown>[]).map(migrateFilterRule);
          setFilterRules(serverRules);
          saveFilterRules(serverRules);
        }
        if (prefs.blocked_senders) {
          setBlockedSenders(prefs.blocked_senders);
          // Backfill meta timestamps for entries that have no recorded date
          try {
            const meta = JSON.parse(localStorage.getItem('webmail_blocked_meta') ?? '{}') as Record<string, string>;
            let changed = false;
            prefs.blocked_senders.forEach((addr) => {
              if (!meta[addr]) { meta[addr] = new Date().toISOString(); changed = true; }
            });
            if (changed) { localStorage.setItem('webmail_blocked_meta', JSON.stringify(meta)); setBlockedMeta(meta); }
          } catch { /* */ }
        }
        if (prefs.allowed_senders) {
          setAllowedSenders(prefs.allowed_senders);
          try {
            const meta = JSON.parse(localStorage.getItem('webmail_allowed_meta') ?? '{}') as Record<string, string>;
            let changed = false;
            prefs.allowed_senders.forEach((addr) => {
              if (!meta[addr]) { meta[addr] = new Date().toISOString(); changed = true; }
            });
            if (changed) { localStorage.setItem('webmail_allowed_meta', JSON.stringify(meta)); setAllowedMeta(meta); }
          } catch { /* */ }
        }
        if (prefs.vacation) {
          const v = prefs.vacation;
          if (v.enabled !== undefined) setVacEnabled(v.enabled as boolean);
          if (v.startDate !== undefined) setVacStartDate(v.startDate as string);
          if (v.endDate !== undefined) setVacEndDate(v.endDate as string);
          if (v.subject) setVacSubject(v.subject as string);
          if (v.body !== undefined) setVacBody(v.body as string);
        }
        if (prefs.signatures && typeof prefs.signatures['default'] === 'string') {
          account.setSignature(prefs.signatures['default']);
          try { localStorage.setItem('webmail_signature', prefs.signatures['default']); } catch { /* ignore */ }
        }
        if (prefs.templates) {
          const serverTemplates = normalizeEmailTemplates(prefs.templates);
          tplState.setTemplates(serverTemplates);
          saveLocalEmailTemplates(serverTemplates);
        }
      } catch { /* ignore */ }
      setPrefsLoaded(true);
    }).catch(() => setPrefsLoaded(true));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Debounced server save (2s after any setting change) ───────────────────────
  useEffect(() => {
    if (!prefsLoaded) return;
    const timer = setTimeout(() => {
      const prefs: WebmailPreferences = {
        settings: {
          readMark, externalImages, sendDelay, showPreview, compact, convMode,
          quoteOnReply, fontSize, inlineImagePreview, blockTrackingPixels,
          requestReadReceipt, linkPreview, followUpDays, focusMode,
          swipeLeft, swipeRight, refreshInterval, importanceMarkers, groupByDate,
          contactsSort, contactsDensity, contactsShowCompany, driveSort,
          browserNotificationsEnabled: notifications.browserNotificationsEnabled,
          notifSound: notifications.notifSound,
          notifDetail: notifications.notifDetail,
          badgeCountMode: notifications.badgeCountMode,
          dndEnabled: notifications.dndEnabled,
          dndStart: notifications.dndStart,
          dndEnd: notifications.dndEnd,
        },
        filter_rules: filterRules as unknown[],
        blocked_senders: blockedSenders,
        allowed_senders: allowedSenders,
        vacation: { enabled: vacEnabled, startDate: vacStartDate, endDate: vacEndDate, subject: vacSubject, body: vacBody },
        templates: tplState.templates,
      };
      setPreferences(prefs).catch(() => {});
    }, 2000);
    return () => clearTimeout(timer);
  }, [
    prefsLoaded,
    readMark, externalImages, sendDelay, showPreview, compact, convMode,
    quoteOnReply, fontSize, inlineImagePreview, blockTrackingPixels,
    requestReadReceipt, linkPreview, followUpDays, focusMode,
    swipeLeft, swipeRight, refreshInterval, importanceMarkers, groupByDate,
    contactsSort, contactsDensity, contactsShowCompany, driveSort,
    notifications.browserNotificationsEnabled, notifications.notifSound,
    notifications.notifDetail, notifications.badgeCountMode,
    notifications.dndEnabled, notifications.dndStart, notifications.dndEnd,
    filterRules, blockedSenders, allowedSenders, tplState.templates,
    vacEnabled, vacStartDate, vacEndDate, vacSubject, vacBody,
  ]);

  // ── Load from storage ─────────────────────────────────────────────────────────
  useEffect(() => {
    try {
      account.setDisplayName(localStorage.getItem('webmail_display_name') ?? userName ?? '');
      account.setSignature(localStorage.getItem('webmail_signature') ?? '');
      setTheme((localStorage.getItem('webmail_theme') as Theme) ?? 'light');
      setAccent(localStorage.getItem('webmail_accent') ?? '#2563eb');
      setCompact(localStorage.getItem('webmail_compact') === '1');
      setConvMode(localStorage.getItem('webmail_conv_mode') !== '0');
      setRefreshInterval((Number(localStorage.getItem('webmail_refresh_interval') ?? 30)) as 30 | 60 | 300);
      const wm = loadWmSettings();
      setReadMark((wm.readMark as ReadMark) ?? 'instant');
      setShowPreview((wm.showPreview as boolean) !== false);
      setExternalImages((wm.externalImages as ExternalImages) ?? 'ask');
      setSendDelay((wm.sendDelay as SendDelay) ?? 0);
      setQuoteOnReply((wm.quoteOnReply as boolean) !== false);
      setFontSize((wm.fontSize as FontSize) ?? 'medium');
      setInlineImagePreview((wm.inlineImagePreview as boolean) !== false);
      notifications.setNotifSound(localStorage.getItem('webmail_notif_sound') === '1');
      notifications.setBrowserNotificationsEnabled(localStorage.getItem(BROWSER_NOTIF_ENABLED_KEY) !== 'false');
      notifications.setNotifDetail((localStorage.getItem('webmail_notif_detail') as 'sender' | 'subject' | 'preview') ?? 'subject');
      const storedBadgeMode = localStorage.getItem(BADGE_COUNT_MODE_KEY);
      notifications.setBadgeCountMode(storedBadgeMode === 'all' || storedBadgeMode === 'none' ? storedBadgeMode : 'unread');
      tplState.setTemplates(loadLocalEmailTemplates());
      setFilterRules(loadFilterRules());
      setBlockedSenders(JSON.parse(localStorage.getItem('webmail_blocked_senders') ?? '[]') as string[]);
      setBlockedMeta(JSON.parse(localStorage.getItem('webmail_blocked_meta') ?? '{}') as Record<string, string>);
      setAllowedSenders(JSON.parse(localStorage.getItem('webmail_allowed_senders') ?? '[]') as string[]);
      setAllowedMeta(JSON.parse(localStorage.getItem('webmail_allowed_meta') ?? '{}') as Record<string, string>);
      const spamDays = parseInt(localStorage.getItem('webmail_spam_autodelete_days') ?? '30', 10);
      setSpamAutoDeleteDays([14, 30, 60, 90, 0].includes(spamDays) ? spamDays : 30);
      setSpamAutoBlock(localStorage.getItem('webmail_spam_auto_block') !== 'false');
      const priv = loadWmSettings();
      setBlockTrackingPixels((priv.blockTrackingPixels as boolean) !== false);
      setRequestReadReceipt((priv.requestReadReceipt as boolean) === true);
      setLinkPreview((priv.linkPreview as boolean) !== false);
      setFollowUpDays(((priv.followUpDays as number) ?? 0) as 0 | 1 | 3 | 7);
      setContactsSort((wm.contactsSort as ContactsSort) ?? 'name');
      setContactsDensity((wm.contactsDensity as ContactsDensity) ?? 'comfortable');
      setContactsShowCompany((wm.contactsShowCompany as boolean) !== false);
      setDriveSort((wm.driveSort as DriveSort) ?? 'typeName');
      const vac = JSON.parse(localStorage.getItem('webmail_vacation') ?? '{}') as Record<string, unknown>;
      setVacEnabled(vac.enabled === true);
      setVacStartDate((vac.startDate as string) ?? '');
      setVacEndDate((vac.endDate as string) ?? '');
      setVacSubject((vac.subject as string) ?? t('vacSubjectDefault'));
      setVacBody((vac.body as string) ?? '');
      // Accessibility
      setHighContrast(localStorage.getItem('webmail_high_contrast') === '1');
      setReducedMotion(localStorage.getItem('webmail_reduced_motion') === '1');
      setLargerClickTargets(localStorage.getItem('webmail_larger_targets') === '1');
      setScreenReaderMode(localStorage.getItem('webmail_screen_reader') === '1');
      setAlwaysFocusRing(localStorage.getItem('webmail_always_focus_ring') === '1');
      setUnderlineLinks(localStorage.getItem('webmail_underline_links') === '1');
      setDyslexiaMode(localStorage.getItem('webmail_dyslexia') === '1');
      const storedFf = localStorage.getItem('webmail_font_family');
      if (storedFf === 'serif' || storedFf === 'mono') setFontFamily(storedFf);
      const storedCb = localStorage.getItem('webmail_colorblind');
      if (storedCb === 'deuteranopia' || storedCb === 'protanopia' || storedCb === 'tritanopia') setColorBlindMode(storedCb);
      const storedUfs = localStorage.getItem('webmail_ui_font_size');
      if (storedUfs === 'sm' || storedUfs === 'md' || storedUfs === 'lg' || storedUfs === 'xl') setUiFontSize(storedUfs);
      const storedLs = localStorage.getItem('webmail_line_spacing');
      if (storedLs === 'relaxed' || storedLs === 'loose') setLineSpacing(storedLs);
      const storedLts = localStorage.getItem('webmail_letter_spacing');
      if (storedLts === 'wide') setLetterSpacing('wide');
    } catch { /* ignore */ }
    if (typeof Notification !== 'undefined') notifications.setNotifPerm(Notification.permission);
  }, [userName, t]); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-load folder stats when storage section becomes active
  useEffect(() => {
    if (activeSection !== 'storage') return;
    if (folderStats.length > 0 || statsLoading) return; // already loaded
    setStatsLoading(true);
    getFolderStats().then((stats) => {
      const seen = new Set<string>();
      setFolderStats(stats.filter((f) => { if (seen.has(f.id)) return false; seen.add(f.id); return true; }));
    }).catch(() => {}).finally(() => setStatsLoading(false));
  }, [activeSection]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Handlers ──────────────────────────────────────────────────────────────────

  function applyTheme(th: Theme) {
    setTheme(th);
    try { localStorage.setItem('webmail_theme', th); } catch { /* ignore */ }
    if (th === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.setAttribute('data-theme', th);
    }
  }

  function applyAccent(color: string) {
    setAccent(color);
    try { localStorage.setItem('webmail_accent', color); } catch { /* ignore */ }
    document.documentElement.style.setProperty('--color-accent', color);
    const hex = color.replace('#', '');
    const r = parseInt(hex.slice(0, 2), 16);
    const g = parseInt(hex.slice(2, 4), 16);
    const b = parseInt(hex.slice(4, 6), 16);
    document.documentElement.style.setProperty('--color-accent-subtle', `rgba(${r},${g},${b},0.1)`);
    document.documentElement.style.setProperty('--color-accent-hover', color);
  }

  function applyFontSize(fs: FontSize) {
    setFontSize(fs);
    saveWmSetting('fontSize', fs);
    const map: Record<FontSize, string> = { small: '13px', medium: '14px', large: '15px' };
    document.documentElement.style.setProperty('--font-size-base', map[fs]);
  }

  function applyColorBlindMode(mode: 'none' | 'deuteranopia' | 'protanopia' | 'tritanopia') {
    const el = document.documentElement;
    el.classList.remove('colorblind-deuteranopia', 'colorblind-protanopia', 'colorblind-tritanopia');
    if (mode !== 'none') el.classList.add(`colorblind-${mode}`);
    setColorBlindMode(mode);
    try { localStorage.setItem('webmail_colorblind', mode); } catch { /* */ }
  }

  function applyUiFontSize(size: 'sm' | 'md' | 'lg' | 'xl') {
    const el = document.documentElement;
    el.classList.remove('ui-font-size-sm', 'ui-font-size-md', 'ui-font-size-lg', 'ui-font-size-xl');
    el.classList.add(`ui-font-size-${size}`);
    setUiFontSize(size);
    try { localStorage.setItem('webmail_ui_font_size', size); } catch { /* */ }
  }

  function applyLineSpacing(spacing: 'normal' | 'relaxed' | 'loose') {
    const el = document.documentElement;
    el.classList.remove('line-spacing-relaxed', 'line-spacing-loose');
    if (spacing !== 'normal') el.classList.add(`line-spacing-${spacing}`);
    setLineSpacing(spacing);
    try { localStorage.setItem('webmail_line_spacing', spacing); } catch { /* */ }
  }

  function applyLetterSpacing(spacing: 'normal' | 'wide') {
    document.documentElement.classList.toggle('letter-spacing-wide', spacing === 'wide');
    setLetterSpacing(spacing);
    try { localStorage.setItem('webmail_letter_spacing', spacing); } catch { /* */ }
  }

  async function startBackup(folderId: string, folderName: string, format: 'eml' | 'zip') {
    const key = `${folderId}-${format}`;
    setBackupStates((p) => ({ ...p, [key]: { status: 'running', fetched: 0, total: 0 } }));
    try {
      const onProgress = (fetched: number, total: number) =>
        setBackupStates((p) => ({ ...p, [key]: { status: 'running', fetched, total } }));
      if (format === 'eml') await exportFolderEml(folderId, folderName, onProgress);
      else await exportFolderZip(folderId, folderName, onProgress);
      setBackupStates((p) => ({ ...p, [key]: { status: 'done', fetched: 0, total: 0 } }));
      setTimeout(() => setBackupStates((p) => ({ ...p, [key]: { status: 'idle', fetched: 0, total: 0 } })), 3000);
    } catch (e) {
      setBackupStates((p) => ({ ...p, [key]: { status: 'error', fetched: 0, total: 0, error: String(e) } }));
    }
  }

  return {
    // Account (from sub-hook)
    ...account,
    // Notifications (from sub-hook)
    ...notifications,
    // Templates (from sub-hook)
    ...tplState,
    // Inbox
    convMode, setConvMode,
    compact, setCompact,
    showPreview, setShowPreview,
    refreshInterval, setRefreshInterval,
    importanceMarkers, setImportanceMarkers,
    groupByDate, setGroupByDate,
    focusMode, setFocusMode,
    swipeLeft, setSwipeLeft,
    swipeRight, setSwipeRight,
    // Contacts
    contactsSort, setContactsSort,
    contactsDensity, setContactsDensity,
    contactsShowCompany, setContactsShowCompany,
    // Drive
    driveSort, setDriveSort,
    // Reading
    readMark, setReadMark,
    externalImages, setExternalImages,
    inlineImagePreview, setInlineImagePreview,
    smartReplySuggestions, setSmartReplySuggestions,
    showReadingTime, setShowReadingTime,
    readingPanePosition, setReadingPanePosition,
    // Compose
    sendDelay, setSendDelay,
    quoteOnReply, setQuoteOnReply,
    fontSize, setFontSize,
    ccSelf, setCcSelf,
    defaultBcc, setDefaultBcc,
    confirmBeforeSend, setConfirmBeforeSend,
    spellCheck, setSpellCheck,
    // Appearance
    theme, setTheme,
    accent, setAccent,
    customAccent, setCustomAccent,
    // Filters
    filterRules, setFilterRules,
    // Privacy
    blockTrackingPixels, setBlockTrackingPixels,
    requestReadReceipt, setRequestReadReceipt,
    linkPreview, setLinkPreview,
    followUpDays, setFollowUpDays,
    // Blocked / Spam
    blockedSenders, setBlockedSenders,
    blockedMeta, setBlockedMeta,
    newBlockedInput, setNewBlockedInput,
    blockedSearch, setBlockedSearch,
    blockedPage, setBlockedPage,
    spamAutoDeleteDays, setSpamAutoDeleteDays,
    spamAutoBlock, setSpamAutoBlock,
    // Allowed senders
    allowedSenders, setAllowedSenders,
    allowedMeta, setAllowedMeta,
    newAllowedInput, setNewAllowedInput,
    allowedSearch, setAllowedSearch,
    allowedPage, setAllowedPage,
    // Vacation
    vacEnabled, setVacEnabled,
    vacStartDate, setVacStartDate,
    vacEndDate, setVacEndDate,
    vacSubject, setVacSubject,
    vacBody, setVacBody,
    vacSaved, setVacSaved,
    // Accessibility
    reducedMotion, setReducedMotion,
    highContrast, setHighContrast,
    largerClickTargets, setLargerClickTargets,
    screenReaderMode, setScreenReaderMode,
    fontFamily, setFontFamily,
    colorBlindMode, setColorBlindMode,
    alwaysFocusRing, setAlwaysFocusRing,
    underlineLinks, setUnderlineLinks,
    dyslexiaMode, setDyslexiaMode,
    uiFontSize, setUiFontSize,
    lineSpacing, setLineSpacing,
    letterSpacing, setLetterSpacing,
    // Timezone
    timezone, setTimezone,
    // Storage
    folderStats, setFolderStats,
    statsLoading, setStatsLoading,
    backupStates, setBackupStates,
    // Prefs loaded flag
    prefsLoaded, setPrefsLoaded,
    // Handlers
    applyTheme,
    applyAccent,
    applyFontSize,
    applyColorBlindMode,
    applyUiFontSize,
    applyLineSpacing,
    applyLetterSpacing,
    startBackup,
  };
}
