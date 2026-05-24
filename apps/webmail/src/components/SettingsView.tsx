'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { CheckIcon, ExclamationTriangleIcon, NoSymbolIcon, ArrowDownTrayIcon, GlobeAltIcon, MagnifyingGlassIcon, CheckCircleIcon } from '@heroicons/react/24/outline';
import { revokeAllSessions, getFolderStats, exportFolderEml, exportFolderZip, getPreferences, setPreferences, getUserProfile, updateUserProfile, uploadUserAvatar, deleteUserAvatar, changePassword, registerWebPushDevice, getNotificationPreferences, setNotificationPreferences, getFolders, type FolderStats, type WebmailPreferences, type UserProfile, type NotificationPreferences, type FolderNotificationOverride, type Folder } from '@/lib/api';
import { ReadMark, ExternalImages, SendDelay, Theme, FontSize, ACCENT_COLORS, FilterRule, migrateFilterRule, loadFilterRules, saveFilterRules } from '@/lib/settings/settingsUtils';
import { NAV_ITEMS, SHORTCUT_GROUPS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { Kbd, MiniEditor, Row, SectionCard, SectionHeader, Segment, Toggle, loadWmSettings, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';
import { FilterRulesSection } from '@/components/settings-view/FilterRulesSection';
import { TimezoneSelect } from '@/components/settings-view/TimezoneSelect';
import { SettingsAboutSection } from '@/components/settings-view/SettingsAboutSection';
import { SettingsStorageSection, type BackupState } from '@/components/settings-view/SettingsStorageSection';
import { SettingsPrivacySection } from '@/components/settings-view/SettingsPrivacySection';
import { SettingsNotificationsSection } from '@/components/settings-view/SettingsNotificationsSection';
import { SettingsSecuritySection } from '@/components/settings-view/SettingsSecuritySection';
import { SettingsMCPSection } from '@/components/settings-view/SettingsMCPSection';
import { handleVerticalNavKeyDown } from '@/lib/navKeyboard';
import { webPushPublicKeyToUint8Array } from '@/lib/webpush';
import { loadLocalEmailTemplates, normalizeEmailTemplates, saveLocalEmailTemplates, type StoredEmailTemplate } from '@/lib/emailTemplates';
import { stableId } from '@/lib/stableId';
import { setWebmailAvatar } from '@/lib/webmailAvatar';

export interface SettingsViewProps {
  userEmail?: string;
  userName?: string;
  initialSection?: SectionId;
}

// ─── Main component ────────────────────────────────────────────────────────────

function currentTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

function quietHoursPreferences(
  base: NotificationPreferences | null,
  folderOverrides: Record<string, FolderNotificationOverride>,
  enabled: boolean,
  start: string,
  end: string,
): NotificationPreferences {
  return {
    global_dnd_enabled: enabled,
    global_dnd_schedule: {
      weekdays: enabled ? [0, 1, 2, 3, 4, 5, 6] : [],
      time_ranges: enabled ? [{ start, end }] : [],
      timezone: base?.global_dnd_schedule?.timezone || currentTimeZone(),
    },
    folder_overrides: folderOverrides ?? base?.folder_overrides ?? {},
    thread_overrides: base?.thread_overrides ?? {},
  };
}

const NOTIFICATION_FOLDER_OVERRIDES_KEY = 'webmail_notification_folder_overrides';
const BADGE_COUNT_MODE_KEY = 'webmail_badge_count_mode';
const BROWSER_NOTIF_ENABLED_KEY = 'webmail_browser_notifications_enabled';
type BadgeCountMode = 'unread' | 'all' | 'none';
type ContactsSort = 'name' | 'email' | 'company';
type ContactsDensity = 'comfortable' | 'compact';
type DriveSort = 'typeName' | 'name' | 'updated' | 'size';

function emptyDNDSchedule() {
  return { weekdays: [], time_ranges: [], timezone: '' };
}

export function SettingsView({ userEmail, userName, initialSection }: SettingsViewProps) {
  const router = useRouter();
  const t = useTranslations('settingsView');
  const [activeSection, setActiveSection] = useState<SectionId>(initialSection ?? 'account');
  const contentRef = useRef<HTMLDivElement>(null);

  // Allow external navigation (e.g. Spotlight) to change the active section
  // even after initial mount.
  useEffect(() => {
    if (initialSection) setActiveSection(initialSection);
  }, [initialSection]);

  // Account
  const [displayName, setDisplayName] = useState('');
  const [nameSaved, setNameSaved] = useState(false);
  const [recoveryEmail, setRecoveryEmail] = useState('');
  const [recoverySaved, setRecoverySaved] = useState(false);
  const [recoveryError, setRecoveryError] = useState('');
  const [signature, setSignature] = useState('');
  const [sigSaved, setSigSaved] = useState(false);
  const [avatarUrl, setAvatarUrl] = useState('');
  const [avatarSaving, setAvatarSaving] = useState(false);
  const [avatarError, setAvatarError] = useState('');

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

  // Notifications
  const [notifPerm, setNotifPerm] = useState<NotificationPermission>('default');
  const [notifSyncError, setNotifSyncError] = useState('');
  const [browserNotificationsEnabled, setBrowserNotificationsEnabled] = useState(true);
  const [notifSound, setNotifSound] = useState(false);
  const [notifDetail, setNotifDetail] = useState<'sender' | 'subject' | 'preview'>('subject');
  const [badgeCountMode, setBadgeCountMode] = useState<BadgeCountMode>('unread');
  const [dndEnabled, setDndEnabled] = useState(false);
  const [dndStart, setDndStart] = useState('22:00');
  const [dndEnd, setDndEnd] = useState('08:00');
  const [webPushEnabled, setWebPushEnabled] = useState<boolean>(() => {
    try { return localStorage.getItem('webmail_webpush_enabled') === 'true'; } catch { return false; }
  });
  const [webPushSupported] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false;
    return 'serviceWorker' in navigator && 'PushManager' in window;
  });
  const [notificationPrefsLoaded, setNotificationPrefsLoaded] = useState(false);
  const notificationPrefsBaseRef = useRef<NotificationPreferences | null>(null);
  const skipNotificationPrefsInitialSaveRef = useRef(true);
  const [notificationFolderOverrides, setNotificationFolderOverrides] = useState<Record<string, FolderNotificationOverride>>({});
  const [notificationFolders, setNotificationFolders] = useState<Folder[]>([]);

  // Templates
  const [templates, setTemplates] = useState<StoredEmailTemplate[]>([]);
  const [newTplName, setNewTplName] = useState('');
  const [newTplSubject, setNewTplSubject] = useState('');
  const [newTplBody, setNewTplBody] = useState('');
  const [showNewTpl, setShowNewTpl] = useState(false);

  // Filters
  const [filterRules, setFilterRules] = useState<FilterRule[]>([]);

  // Privacy
  const [blockTrackingPixels, setBlockTrackingPixels] = useState(true);
  const [requestReadReceipt, setRequestReadReceipt] = useState(false);
  const [linkPreview, setLinkPreview] = useState(true);
  const [followUpDays, setFollowUpDays] = useState<0 | 1 | 3 | 7>(0);

  // Blocked senders / Spam settings
  const [blockedSenders, setBlockedSenders] = useState<string[]>([]);
  const [blockedMeta, setBlockedMeta] = useState<Record<string, string>>({}); // addr → ISO date
  const [newBlockedInput, setNewBlockedInput] = useState('');
  const [blockedSearch, setBlockedSearch] = useState('');
  const [blockedPage, setBlockedPage] = useState(0);
  const [spamAutoDeleteDays, setSpamAutoDeleteDays] = useState<number>(30);
  const [spamAutoBlock, setSpamAutoBlock] = useState(true);
  // Allowed senders (allowlist)
  const [allowedSenders, setAllowedSenders] = useState<string[]>([]);
  const [allowedMeta, setAllowedMeta] = useState<Record<string, string>>({}); // addr → ISO date
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

  // Security
  const [revokingAll, setRevokingAll] = useState(false);
  const [revokeAllError, setRevokeAllError] = useState('');

  // Storage / Backup
  const [folderStats, setFolderStats] = useState<FolderStats[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  const [backupStates, setBackupStates] = useState<Record<string, BackupState>>({});

  // User profile
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [pwCurrent, setPwCurrent] = useState('');
  const [pwNew, setPwNew] = useState('');
  const [pwConfirm, setPwConfirm] = useState('');
  const [pwError, setPwError] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwSaved, setPwSaved] = useState(false);

  // Server-side preferences sync
  const [prefsLoaded, setPrefsLoaded] = useState(false);

  // ── Load server preferences (overlay over localStorage on mount) ──────────────
  useEffect(() => {
    getUserProfile().then((p) => {
      if (p) {
        setProfile(p);
        setRecoveryEmail(p.recovery_email ?? '');
        setAvatarUrl(p.avatar_url ?? '');
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
            setBrowserNotificationsEnabled(enabled);
            try { localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, enabled ? 'true' : 'false'); } catch { /* ignore */ }
          }
          if (s.notifSound !== undefined) setNotifSound(s.notifSound as boolean);
          if (s.notifDetail) setNotifDetail(s.notifDetail as typeof notifDetail);
          if (s.badgeCountMode === 'all' || s.badgeCountMode === 'none' || s.badgeCountMode === 'unread') {
            setBadgeCountMode(s.badgeCountMode);
            try { localStorage.setItem(BADGE_COUNT_MODE_KEY, s.badgeCountMode); } catch { /* ignore */ }
          }
          if (s.dndEnabled !== undefined) setDndEnabled(s.dndEnabled as boolean);
          if (s.dndStart) setDndStart(s.dndStart as string);
          if (s.dndEnd) setDndEnd(s.dndEnd as string);
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
          setSignature(prefs.signatures['default']);
          try { localStorage.setItem('webmail_signature', prefs.signatures['default']); } catch { /* ignore */ }
        }
        if (prefs.templates) {
          const serverTemplates = normalizeEmailTemplates(prefs.templates);
          setTemplates(serverTemplates);
          saveLocalEmailTemplates(serverTemplates);
        }
      } catch { /* ignore */ }
      setPrefsLoaded(true);
    }).catch(() => setPrefsLoaded(true));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    getFolders()
      .then((data) => setNotificationFolders(data.folders ?? []))
      .catch(() => setNotificationFolders([]));
  }, []);

  useEffect(() => {
    let cancelled = false;
    getNotificationPreferences()
      .then((prefs) => {
        if (cancelled) return;
        notificationPrefsBaseRef.current = prefs;
        setNotificationFolderOverrides(prefs.folder_overrides ?? {});
        setDndEnabled(prefs.global_dnd_enabled);
        const firstRange = prefs.global_dnd_schedule?.time_ranges?.[0];
        if (firstRange?.start) setDndStart(firstRange.start);
        if (firstRange?.end) setDndEnd(firstRange.end);
        try {
          localStorage.setItem('webmail_dnd', prefs.global_dnd_enabled ? '1' : '0');
          localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(prefs.folder_overrides ?? {}));
          if (firstRange?.start) localStorage.setItem('webmail_dnd_start', firstRange.start);
          if (firstRange?.end) localStorage.setItem('webmail_dnd_end', firstRange.end);
        } catch {
          // local settings cache is best-effort
        }
      })
      .catch(() => {
        // Older backends may not expose server-side notification preferences.
      })
      .finally(() => {
        if (!cancelled) setNotificationPrefsLoaded(true);
      });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!notificationPrefsLoaded) return;
    if (skipNotificationPrefsInitialSaveRef.current) {
      skipNotificationPrefsInitialSaveRef.current = false;
      return;
    }
    const timer = setTimeout(() => {
      const next = quietHoursPreferences(notificationPrefsBaseRef.current, notificationFolderOverrides, dndEnabled, dndStart, dndEnd);
      setNotificationPreferences(next)
        .then((saved) => {
          notificationPrefsBaseRef.current = saved;
          try {
            localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(saved.folder_overrides ?? {}));
          } catch {
            // local settings cache is best-effort
          }
        })
        .catch(() => {});
    }, 800);
    return () => clearTimeout(timer);
  }, [notificationPrefsLoaded, notificationFolderOverrides, dndEnabled, dndStart, dndEnd]);

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
          browserNotificationsEnabled, notifSound, notifDetail, badgeCountMode, dndEnabled, dndStart, dndEnd,
        },
        filter_rules: filterRules as unknown[],
        blocked_senders: blockedSenders,
        allowed_senders: allowedSenders,
        vacation: { enabled: vacEnabled, startDate: vacStartDate, endDate: vacEndDate, subject: vacSubject, body: vacBody },
        templates,
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
    browserNotificationsEnabled, notifSound, notifDetail, badgeCountMode, dndEnabled, dndStart, dndEnd,
    filterRules, blockedSenders, allowedSenders, templates,
    vacEnabled, vacStartDate, vacEndDate, vacSubject, vacBody,
  ]);

  // ── Load from storage ─────────────────────────────────────────────────────────
  useEffect(() => {
    try {
      setDisplayName(localStorage.getItem('webmail_display_name') ?? userName ?? '');
      setSignature(localStorage.getItem('webmail_signature') ?? '');
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
      setNotifSound(localStorage.getItem('webmail_notif_sound') === '1');
      setBrowserNotificationsEnabled(localStorage.getItem(BROWSER_NOTIF_ENABLED_KEY) !== 'false');
      setNotifDetail((localStorage.getItem('webmail_notif_detail') as 'sender' | 'subject' | 'preview') ?? 'subject');
      const storedBadgeMode = localStorage.getItem(BADGE_COUNT_MODE_KEY);
      setBadgeCountMode(storedBadgeMode === 'all' || storedBadgeMode === 'none' ? storedBadgeMode : 'unread');
      setTemplates(loadLocalEmailTemplates());
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
      if (storedUfs === 'sm' || storedUfs === 'lg' || storedUfs === 'xl') setUiFontSize(storedUfs);
      const storedLs = localStorage.getItem('webmail_line_spacing');
      if (storedLs === 'relaxed' || storedLs === 'loose') setLineSpacing(storedLs);
      const storedLts = localStorage.getItem('webmail_letter_spacing');
      if (storedLts === 'wide') setLetterSpacing('wide');
    } catch { /* ignore */ }
    if (typeof Notification !== 'undefined') setNotifPerm(Notification.permission);
  }, [userName, t]);

  // ── Handlers ──────────────────────────────────────────────────────────────────

  function applyTheme(t: Theme) {
    setTheme(t);
    try { localStorage.setItem('webmail_theme', t); } catch { /* ignore */ }
    if (t === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.setAttribute('data-theme', t);
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


  async function handleAvatarUpload(file: File | undefined) {
    if (!file) return;
    setAvatarError('');
    setAvatarSaving(true);
    try {
      const url = await uploadUserAvatar(file);
      setAvatarUrl(url);
      setWebmailAvatar(url);
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : t('avatarUploadError'));
    } finally {
      setAvatarSaving(false);
    }
  }

  async function handleAvatarRemove() {
    setAvatarError('');
    setAvatarSaving(true);
    try {
      await deleteUserAvatar();
      setAvatarUrl('');
      setWebmailAvatar('');
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : t('avatarRemoveError'));
    } finally {
      setAvatarSaving(false);
    }
  }

  function saveDisplayName() {
    try { localStorage.setItem('webmail_display_name', displayName); } catch { /* ignore */ }
    updateUserProfile({ display_name: displayName }).catch(() => {});
    setNameSaved(true);
    setTimeout(() => setNameSaved(false), 2000);
  }

  async function saveRecoveryEmail() {
    setRecoveryError('');
    try {
      await updateUserProfile({ recovery_email: recoveryEmail.trim() });
      setRecoverySaved(true);
      setTimeout(() => setRecoverySaved(false), 2000);
    } catch (err) {
      setRecoveryError(err instanceof Error ? err.message : t('recoverySaveError'));
    }
  }

  function saveSignature() {
    try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ }
    setPreferences({ signatures: { default: signature } }).catch(() => {});
    setSigSaved(true);
    setTimeout(() => setSigSaved(false), 2000);
  }

  async function handleChangePassword() {
    setPwError('');
    if (!pwCurrent || !pwNew || !pwConfirm) { setPwError(t('pwAllRequired')); return; }
    if (pwNew.length < 8) { setPwError(t('pwMinLength')); return; }
    if (pwNew !== pwConfirm) { setPwError(t('pwMismatch')); return; }
    setPwSaving(true);
    try {
      await changePassword(pwCurrent, pwNew);
      setPwCurrent(''); setPwNew(''); setPwConfirm('');
      setPwSaved(true);
      setTimeout(() => setPwSaved(false), 3000);
    } catch (err) {
      setPwError(err instanceof Error ? err.message : t('pwChangeFailed'));
    } finally {
      setPwSaving(false);
    }
  }

  async function handleRevokeAll() {
    if (!window.confirm(t('revokeAllConfirm'))) return;
    setRevokeAllError('');
    setRevokingAll(true);
    const ok = await revokeAllSessions();
    if (ok) {
      try { localStorage.removeItem('webmail_token'); localStorage.removeItem('webmail_email'); } catch { /* ignore */ }
      router.push('/login');
    } else {
      setRevokingAll(false);
      setRevokeAllError(t('revokeFailed'));
    }
  }

  async function requestNotif() {
    if (typeof Notification === 'undefined') return;
    const p = await Notification.requestPermission();
    setNotifPerm(p);
    setNotifSyncError('');
    if (p === 'granted') {
      setBrowserNotificationsEnabled(true);
      try {
        localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, 'true');
        window.dispatchEvent(new StorageEvent('storage', { key: BROWSER_NOTIF_ENABLED_KEY, newValue: 'true' }));
      } catch {
        // local settings cache is best-effort
      }
    }
    if (p === 'granted' && 'serviceWorker' in navigator && 'PushManager' in window) {
      try {
        const reg = await navigator.serviceWorker.register('/sw.js');
        const vapidKey = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
        if (vapidKey) {
          const sub = await reg.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: webPushPublicKeyToUint8Array(vapidKey),
          });
          await registerWebPushDevice(sub);
        }
      } catch {
        setNotifSyncError(t('pushRegisterFailed'));
      }
    }
  }

  // ─── Render ──────────────────────────────────────────────────────────────────

  function renderContent() {
    switch (activeSection) {

      case 'account':
        return (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: '16px', padding: '20px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '10px', marginBottom: '20px' }}>
              <div style={{ width: '52px', height: '52px', borderRadius: '50%', background: avatarUrl ? 'transparent' : 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px', fontWeight: 700, flexShrink: 0, overflow: 'hidden' }}>
                {avatarUrl ? <img src={avatarUrl} alt={t('profilePhotoAlt')} style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : (displayName || userEmail || '?')[0].toUpperCase()}
              </div>
              <div>
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{displayName || userName || t('nameEmpty')}</div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '3px' }}>{userEmail}</div>
              </div>
            </div>
            <SectionCard>
              <SectionHeader>{t('sectionProfile')}</SectionHeader>

              <Row label={t('profilePhoto')} description={t('profilePhotoDesc')}>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                  <label style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '12px', fontWeight: 600, cursor: avatarSaving ? 'not-allowed' : 'pointer', whiteSpace: 'nowrap', opacity: avatarSaving ? 0.6 : 1 }}>
                    {avatarSaving ? t('profilePhotoSaving') : t('profilePhotoUpload')}
                    <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" disabled={avatarSaving} style={{ display: 'none' }} onChange={(e) => { void handleAvatarUpload(e.target.files?.[0]); e.currentTarget.value = ''; }} />
                  </label>
                  {avatarUrl && (
                    <button onClick={handleAvatarRemove} disabled={avatarSaving} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 600, cursor: avatarSaving ? 'not-allowed' : 'pointer', whiteSpace: 'nowrap' }}>
                      {t('profilePhotoRemove')}
                    </button>
                  )}
                  {avatarError && <span style={{ fontSize: '12px', color: 'var(--color-danger, #dc2626)', width: '100%', textAlign: 'right' }}>{avatarError}</span>}
                </div>
              </Row>
              <Row label={t('displayName')} description={t('displayNameDesc')}>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder={t('namePlaceholder')} style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '170px', outline: 'none' }} />
                  <button onClick={saveDisplayName} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
                    {nameSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('save')}
                  </button>
                </div>
              </Row>
              <Row label={t('emailAddress')} description={t('emailAddressDesc')}>
                <span style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>{userEmail}</span>
              </Row>
              <Row label={t('recoveryEmail')} description={t('recoveryEmailDesc')}>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                  <input
                    type="email"
                    value={recoveryEmail}
                    onChange={(e) => setRecoveryEmail(e.target.value)}
                    placeholder="personal@example.com"
                    style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '220px', outline: 'none' }}
                  />
                  <button onClick={saveRecoveryEmail} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
                    {recoverySaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('save')}
                  </button>
                  {recoveryError && <span style={{ fontSize: '12px', color: 'var(--color-danger, #dc2626)', width: '100%', textAlign: 'right' }}>{recoveryError}</span>}
                </div>
              </Row>
              <Row label={t('timezone')} description={t('timezoneDesc')} last>
                <TimezoneSelect
                  value={timezone}
                  onChange={(v) => {
                    setTimezone(v);
                    try { localStorage.setItem('webmail_timezone', v); } catch { /* */ }
                  }}
                  placeholder={t('timezonePlaceholder')}
                />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>{t('sectionSignature')}</SectionHeader>
              <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '10px' }}>{t('signatureAutoAdd')}</div>
                <MiniEditor
                  value={signature}
                  onChange={(html) => { setSignature(html); }}
                  placeholder={t('signaturePlaceholder')}
                />
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '10px' }}>
                  <button onClick={saveSignature} style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '5px' }}>
                    {sigSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('saveSignature')}
                  </button>
                </div>
              </div>
            </SectionCard>
            {profile && (
              <SectionCard>
                <SectionHeader>{t('sectionQuota')}</SectionHeader>
                <div style={{ padding: '16px 20px' }}>
                  {(() => {
                    const used = profile.quota_used;
                    const limit = profile.quota_limit;
                    const pct = limit && limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : null;
                    const fmt = (b: number) => b >= 1073741824 ? `${(b / 1073741824).toFixed(1)} GB` : b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.round(b / 1024)} KB`;
                    return (
                      <>
                        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '8px' }}>
                          <span>{fmt(used)} {t('quotaUsed')}{limit ? ` / ${fmt(limit)}` : ''}</span>
                          {pct !== null && <span>{pct}%</span>}
                        </div>
                        {pct !== null && (
                          <div style={{ height: '6px', borderRadius: '3px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                            <div style={{ height: '100%', width: `${pct}%`, background: pct > 90 ? 'var(--color-destructive)' : pct > 75 ? '#f59e0b' : 'var(--color-accent)', borderRadius: '3px', transition: 'width 400ms ease' }} />
                          </div>
                        )}
                      </>
                    );
                  })()}
                </div>
              </SectionCard>
            )}
            <SectionCard>
              <SectionHeader>{t('sectionChangePassword')}</SectionHeader>
              <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                {([t('currentPassword'), t('newPassword'), t('confirmNewPassword')] as const).map((label, i) => (
                  <div key={label} style={{ display: 'flex', flexDirection: 'column', gap: '5px' }}>
                    <label style={{ fontSize: '12px', color: 'var(--color-text-secondary)', fontWeight: 500 }}>{label}</label>
                    <input
                      type="password"
                      value={[pwCurrent, pwNew, pwConfirm][i]}
                      onChange={(e) => [setPwCurrent, setPwNew, setPwConfirm][i](e.target.value)}
                      style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }}
                    />
                  </div>
                ))}
                {pwError && <div style={{ fontSize: '12px', color: 'var(--color-destructive)', padding: '6px 10px', background: 'rgba(217,79,61,0.08)', borderRadius: '5px' }}>{pwError}</div>}
                {pwSaved && <div style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', padding: '6px 10px', background: 'rgba(34,197,94,0.08)', borderRadius: '5px' }}>{t('pwChanged')}</div>}
                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button onClick={handleChangePassword} disabled={pwSaving} style={{ padding: '7px 18px', borderRadius: '6px', border: 'none', background: pwSaving ? 'var(--color-bg-tertiary)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: pwSaving ? 'not-allowed' : 'pointer' }}>
                    {pwSaving ? t('pwChanging') : t('pwChange')}
                  </button>
                </div>
              </div>
            </SectionCard>
          </>
        );

      case 'inbox':
        return (
          <SectionCard>
            <SectionHeader>{t('sectionInboxSettings')}</SectionHeader>
            <Row label={t('convMode')} description={t('convModeDesc')}>
              <Toggle value={convMode} onChange={(v) => { setConvMode(v); try { localStorage.setItem('webmail_conv_mode', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('compactView')} description={t('compactViewDesc')}>
              <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('previewText')} description={t('previewTextDesc')}>
              <Toggle value={showPreview} onChange={(v) => { setShowPreview(v); saveWmSetting('showPreview', v); }} />
            </Row>
            <Row label={t('autoRefresh')} description={t('autoRefreshDesc')}>
              <Segment
                options={[{ value: 30 as 30, label: t('sec30') }, { value: 60 as 60, label: t('min1') }, { value: 300 as 300, label: t('min5') }]}
                value={refreshInterval}
                onChange={(v) => {
                  setRefreshInterval(v);
                  try {
                    localStorage.setItem('webmail_refresh_interval', String(v));
                    window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_refresh_interval', newValue: String(v) }));
                  } catch { /* */ }
                }}
              />
            </Row>
            <Row label={t('groupByDate')} description={t('groupByDateDesc')}>
              <Toggle value={groupByDate} onChange={(v) => { setGroupByDate(v); try { localStorage.setItem('webmail_group_by_date', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('importanceMarkers')} description={t('importanceMarkersDesc')}>
              <Toggle value={importanceMarkers} onChange={(v) => { setImportanceMarkers(v); try { localStorage.setItem('webmail_importance_markers', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('focusMode')} description={t('focusModeDesc')}>
              <Toggle value={focusMode} onChange={(v) => { setFocusMode(v); try { localStorage.setItem('webmail_focus_mode', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('swipeLeft')} description={t('swipeLeftDesc')}>
              <Segment
                options={[{ value: 'archive' as const, label: t('swipeArchive') }, { value: 'delete' as const, label: t('swipeDelete') }, { value: 'snooze' as const, label: t('swipeSnooze') }, { value: 'star' as const, label: t('swipeStar') }]}
                value={swipeLeft}
                onChange={(v) => { setSwipeLeft(v); try { localStorage.setItem('webmail_swipe_left', v); } catch { /* */ } }}
              />
            </Row>
            <Row label={t('swipeRight')} description={t('swipeRightDesc')} last>
              <Segment
                options={[{ value: 'archive' as const, label: t('swipeArchive') }, { value: 'delete' as const, label: t('swipeDelete') }, { value: 'snooze' as const, label: t('swipeSnooze') }, { value: 'star' as const, label: t('swipeStar') }]}
                value={swipeRight}
                onChange={(v) => { setSwipeRight(v); try { localStorage.setItem('webmail_swipe_right', v); } catch { /* */ } }}
              />
            </Row>
          </SectionCard>
        );

      case 'contacts':
        return (
          <SectionCard>
            <SectionHeader>{t('sectionContactsSettings')}</SectionHeader>
            <Row label={t('contactsSort')} description={t('contactsSortDesc')}>
              <Segment
                options={[{ value: 'name' as ContactsSort, label: t('contactsSortName') }, { value: 'email' as ContactsSort, label: t('contactsSortEmail') }, { value: 'company' as ContactsSort, label: t('contactsSortCompany') }]}
                value={contactsSort}
                onChange={(v) => { setContactsSort(v); saveWmSetting('contactsSort', v); }}
              />
            </Row>
            <Row label={t('contactsDensity')} description={t('contactsDensityDesc')}>
              <Segment
                options={[{ value: 'comfortable' as ContactsDensity, label: t('densityComfortable') }, { value: 'compact' as ContactsDensity, label: t('densityCompact') }]}
                value={contactsDensity}
                onChange={(v) => { setContactsDensity(v); saveWmSetting('contactsDensity', v); }}
              />
            </Row>
            <Row label={t('contactsShowCompany')} description={t('contactsShowCompanyDesc')} last>
              <Toggle value={contactsShowCompany} onChange={(v) => { setContactsShowCompany(v); saveWmSetting('contactsShowCompany', v); }} />
            </Row>
          </SectionCard>
        );

      case 'drive':
        return (
          <SectionCard>
            <SectionHeader>{t('sectionDriveSettings')}</SectionHeader>
            <Row label={t('driveSort')} description={t('driveSortDesc')} last>
              <Segment
                options={[{ value: 'typeName' as DriveSort, label: t('driveSortTypeName') }, { value: 'name' as DriveSort, label: t('driveSortName') }, { value: 'updated' as DriveSort, label: t('driveSortUpdated') }, { value: 'size' as DriveSort, label: t('driveSortSize') }]}
                value={driveSort}
                onChange={(v) => { setDriveSort(v); saveWmSetting('driveSort', v); }}
              />
            </Row>
          </SectionCard>
        );

      case 'reading':
        return (
          <SectionCard>
            <SectionHeader>{t('sectionReadingSettings')}</SectionHeader>
            <Row label={t('readMark')} description={t('readMarkDesc')}>
              <Segment
                options={[{ value: 'instant' as ReadMark, label: t('readMarkInstant') }, { value: '2s' as ReadMark, label: t('readMark2s') }, { value: 'manual' as ReadMark, label: t('readMarkManual') }]}
                value={readMark}
                onChange={(v) => { setReadMark(v); saveWmSetting('readMark', v); }}
              />
            </Row>
            <Row label={t('externalImages')} description={t('externalImagesDesc')}>
              <Segment
                options={[{ value: 'always' as ExternalImages, label: t('externalImagesAlways') }, { value: 'ask' as ExternalImages, label: t('externalImagesAsk') }, { value: 'never' as ExternalImages, label: t('externalImagesNever') }]}
                value={externalImages}
                onChange={(v) => { setExternalImages(v); saveWmSetting('externalImages', v); }}
              />
            </Row>
            <Row label={t('inlineImagePreview')} description={t('inlineImagePreviewDesc')}>
              <Toggle value={inlineImagePreview} onChange={(v) => { setInlineImagePreview(v); saveWmSetting('inlineImagePreview', v); }} />
            </Row>
            <Row label={t('smartReply')} description={t('smartReplyDesc')}>
              <Toggle value={smartReplySuggestions} onChange={(v) => { setSmartReplySuggestions(v); try { localStorage.setItem('webmail_smart_reply', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('readingTime')} description={t('readingTimeDesc')}>
              <Toggle value={showReadingTime} onChange={(v) => { setShowReadingTime(v); try { localStorage.setItem('webmail_reading_time', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label={t('readingPane')} description={t('readingPaneDesc')} last>
              <Segment
                options={[{ value: 'right' as const, label: t('paneRight') }, { value: 'bottom' as const, label: t('paneBottom') }, { value: 'hidden' as const, label: t('paneHidden') }]}
                value={readingPanePosition}
                onChange={(v) => { setReadingPanePosition(v); try { localStorage.setItem('webmail_reading_pane', v); } catch { /* */ } }}
              />
            </Row>
          </SectionCard>
        );

      case 'compose': {
        function saveTpl() {
          if (!newTplName.trim()) return;
          const next = normalizeEmailTemplates([
            ...templates.filter((t) => t.name !== newTplName.trim()),
            { id: stableId('template'), name: newTplName.trim(), subject: newTplSubject.trim(), body: newTplBody.trim() },
          ]);
          setTemplates(next);
          saveLocalEmailTemplates(next);
          setPreferences({ templates: next }).catch(() => {});
          setNewTplName(''); setNewTplSubject(''); setNewTplBody(''); setShowNewTpl(false);
        }
        function deleteTpl(name: string) {
          const next = templates.filter((t) => t.name !== name);
          setTemplates(next);
          saveLocalEmailTemplates(next);
          setPreferences({ templates: next }).catch(() => {});
        }
        return (
          <>
            <SectionCard>
              <SectionHeader>{t('sectionComposeSettings')}</SectionHeader>
              <Row label={t('sendDelay')} description={t('sendDelayDesc')}>
                <Segment
                  options={[{ value: 0 as SendDelay, label: t('sendDelayNone') }, { value: 5 as SendDelay, label: t('sendDelay5s') }, { value: 10 as SendDelay, label: t('sendDelay10s') }, { value: 30 as SendDelay, label: t('sendDelay30s') }]}
                  value={sendDelay}
                  onChange={(v) => { setSendDelay(v); saveWmSetting('sendDelay', v); }}
                />
              </Row>
              <Row label={t('quoteOnReply')} description={t('quoteOnReplyDesc')}>
                <Toggle value={quoteOnReply} onChange={(v) => { setQuoteOnReply(v); saveWmSetting('quoteOnReply', v); }} />
              </Row>
              <Row label={t('fontSizeDefault')} description={t('fontSizeDefaultDesc')}>
                <Segment
                  options={[{ value: 'small' as FontSize, label: t('fontSizeSmall') }, { value: 'medium' as FontSize, label: t('fontSizeMedium') }, { value: 'large' as FontSize, label: t('fontSizeLarge') }]}
                  value={fontSize}
                  onChange={(v) => applyFontSize(v)}
                />
              </Row>
              <Row label={t('confirmBeforeSend')} description={t('confirmBeforeSendDesc')}>
                <Toggle value={confirmBeforeSend} onChange={(v) => { setConfirmBeforeSend(v); try { localStorage.setItem('webmail_confirm_before_send', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label={t('ccSelf')} description={t('ccSelfDesc')}>
                <Toggle value={ccSelf} onChange={(v) => { setCcSelf(v); try { localStorage.setItem('webmail_cc_self', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label={t('spellCheck')} description={t('spellCheckDesc')}>
                <Toggle value={spellCheck} onChange={(v) => { setSpellCheck(v); try { localStorage.setItem('webmail_spell_check', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label={t('defaultBcc')} description={t('defaultBccDesc')} last>
                <input
                  type="email"
                  value={defaultBcc}
                  onChange={(e) => { setDefaultBcc(e.target.value); try { localStorage.setItem('webmail_default_bcc', e.target.value); } catch { /* */ } }}
                  placeholder="bcc@example.com"
                  style={{ width: '200px', padding: '5px 10px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }}
                />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>{t('sectionQuickReplyTemplates')}</SectionHeader>
              {templates.length === 0 && !showNewTpl && (
                <div style={{ padding: '16px 20px', fontSize: '13px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-primary)' }}>
                  {t('noTemplates')}
                </div>
              )}
              {templates.map((tpl, i) => (
                <div key={tpl.name} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '12px 20px', borderBottom: i < templates.length - 1 || showNewTpl ? '1px solid var(--color-border-subtle)' : 'none', background: 'var(--color-bg-primary)' }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{tpl.name}</div>
                    {tpl.subject && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('templateSubjectLabel')}: {tpl.subject}</div>}
                  </div>
                  <button onClick={() => deleteTpl(tpl.name)} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(220,38,38,0.3)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer' }}>{t('delete')}</button>
                </div>
              ))}
              {showNewTpl && (
                <div style={{ padding: '14px 20px', background: 'var(--color-bg-secondary)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  <input value={newTplName} onChange={(e) => setNewTplName(e.target.value)} placeholder={t('tplNamePlaceholder')} style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
                  <input value={newTplSubject} onChange={(e) => setNewTplSubject(e.target.value)} placeholder={t('tplSubjectPlaceholder')} style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
                  <textarea value={newTplBody} onChange={(e) => setNewTplBody(e.target.value)} placeholder={t('tplBodyPlaceholder')} rows={4} style={{ padding: '8px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', fontFamily: 'inherit', outline: 'none' }} />
                  <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                    <button onClick={() => setShowNewTpl(false)} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>{t('cancel')}</button>
                    <button onClick={saveTpl} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer' }}>{t('save')}</button>
                  </div>
                </div>
              )}
              {!showNewTpl && (
                <div style={{ padding: '10px 20px', background: 'var(--color-bg-primary)', borderTop: templates.length > 0 ? '1px solid var(--color-border-subtle)' : 'none' }}>
                  <button onClick={() => setShowNewTpl(true)} style={{ fontSize: '13px', color: 'var(--color-accent)', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 500, padding: 0 }}>{t('newTemplate')}</button>
                </div>
              )}
            </SectionCard>
          </>
        );
      }

      case 'filters': {
        return <FilterRulesSection filterRules={filterRules} setFilterRules={setFilterRules} />;
      }

      case 'blocked': {
        const PAGE_SIZE = 5;
        const q = blockedSearch.trim().toLowerCase();
        const filteredSenders = q ? blockedSenders.filter((a) => a.includes(q)) : blockedSenders;
        const totalPages = Math.ceil(filteredSenders.length / PAGE_SIZE);
        const safePage = Math.min(blockedPage, Math.max(0, totalPages - 1));
        const pageItems = filteredSenders.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

        function saveBlocked(next: string[], meta?: Record<string, string>) {
          try { localStorage.setItem('webmail_blocked_senders', JSON.stringify(next)); } catch { /* ignore */ }
          setBlockedSenders(next);
          if (meta !== undefined) {
            try { localStorage.setItem('webmail_blocked_meta', JSON.stringify(meta)); } catch { /* ignore */ }
            setBlockedMeta(meta);
          }
          void setPreferences({ blocked_senders: next });
        }
        function addBlocked() {
          const val = newBlockedInput.trim().toLowerCase();
          if (!val || blockedSenders.includes(val)) return;
          const now = new Date().toISOString();
          const nextMeta = { ...blockedMeta, [val]: now };
          saveBlocked([...blockedSenders, val], nextMeta);
          setNewBlockedInput('');
          // Jump to last page to show newly added entry
          setBlockedPage(Math.floor(blockedSenders.length / PAGE_SIZE));
        }
        function removeBlocked(addr: string) {
          const next = blockedSenders.filter((a) => a !== addr);
          const nextMeta = { ...blockedMeta };
          delete nextMeta[addr];
          saveBlocked(next, nextMeta);
          // Keep page in range
          const newTotal = Math.ceil(next.length / PAGE_SIZE);
          if (safePage >= newTotal && safePage > 0) setBlockedPage(safePage - 1);
        }
        function formatBlockedDate(addr: string): string {
          const iso = blockedMeta[addr];
          if (!iso) return '—';
          try {
            return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso));
          } catch { return iso.slice(0, 10); }
        }

        const autoDeleteOptions: { value: number; labelKey: string }[] = [
          { value: 14, labelKey: 'spamDelete14' },
          { value: 30, labelKey: 'spamDelete30' },
          { value: 60, labelKey: 'spamDelete60' },
          { value: 90, labelKey: 'spamDelete90' },
          { value: 0, labelKey: 'spamDeleteNever' },
        ];

        const thSt: React.CSSProperties = {
          padding: '8px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 700,
          letterSpacing: '0.06em', textTransform: 'uppercase',
          color: 'var(--color-text-tertiary)',
          borderBottom: '1px solid var(--color-border-default)',
          whiteSpace: 'nowrap', background: 'var(--color-bg-secondary)',
        };
        const tdSt: React.CSSProperties = {
          padding: '9px 14px', fontSize: '13px',
          color: 'var(--color-text-primary)',
          borderBottom: '1px solid var(--color-border-subtle)',
          verticalAlign: 'middle',
        };

        return (
          <>
            {/* ── 스팸 필터 설정 ── */}
            <SectionCard>
              <SectionHeader>{t('sectionSpamFilter')}</SectionHeader>
              <Row label={t('spamAutoDelete')} description={t('spamAutoDeleteDesc')}>
                <Segment
                  value={String(spamAutoDeleteDays)}
                  onChange={(v) => {
                    const days = Number(v);
                    setSpamAutoDeleteDays(days);
                    try { localStorage.setItem('webmail_spam_autodelete_days', String(days)); } catch { /* */ }
                  }}
                  options={autoDeleteOptions.map((o) => ({ value: String(o.value), label: t(o.labelKey) }))}
                />
              </Row>
              <Row label={t('spamAutoBlock')} description={t('spamAutoBlockDesc')}>
                <Toggle
                  value={spamAutoBlock}
                  onChange={(v) => {
                    setSpamAutoBlock(v);
                    try { localStorage.setItem('webmail_spam_auto_block', v ? 'true' : 'false'); } catch { /* */ }
                  }}
                />
              </Row>
            </SectionCard>

            {/* ── 차단된 발신자 목록 (table + pagination) ── */}
            <SectionCard>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '16px 20px 0', flexWrap: 'wrap' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('sectionBlockedSenders')}</div>
                  <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('blockedSendersDesc')}</div>
                </div>
                {/* Search input */}
                {blockedSenders.length > 0 && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                    <div style={{ position: 'relative' }}>
                      <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: 'var(--color-text-tertiary)', pointerEvents: 'none' }} />
                      <input
                        type="text"
                        value={blockedSearch}
                        onChange={(e) => { setBlockedSearch(e.target.value); setBlockedPage(0); }}
                        placeholder={t('blockedSearchPlaceholder')}
                        style={{
                          paddingLeft: 26, paddingRight: 8, paddingTop: 5, paddingBottom: 5,
                          width: 190, fontSize: '12px',
                          border: '1px solid var(--color-border-default)',
                          borderRadius: '6px',
                          background: 'var(--color-bg-secondary)',
                          color: 'var(--color-text-primary)',
                          outline: 'none',
                          fontFamily: 'monospace',
                        }}
                        onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                        onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                      />
                      {blockedSearch && (
                        <button
                          onClick={() => { setBlockedSearch(''); setBlockedPage(0); }}
                          style={{ position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--color-text-tertiary)', lineHeight: 1, fontSize: 14 }}
                          aria-label={t('blockedSearchClear')}
                        >×</button>
                      )}
                    </div>
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                      {q ? t('blockedSearchCount', { found: filteredSenders.length, total: blockedSenders.length }) : t('blockedCount', { count: blockedSenders.length })}
                    </span>
                  </div>
                )}
              </div>

              <div style={{ overflowX: 'auto', margin: '12px 0 0' }}>
                {blockedSenders.length === 0 ? (
                  <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
                    {t('noBlocked')}
                  </div>
                ) : filteredSenders.length === 0 ? (
                  <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
                    {t('blockedSearchEmpty')}
                  </div>
                ) : (
                  <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
                    <colgroup>
                      <col style={{ width: '40px' }} />
                      <col />
                      <col style={{ width: '160px' }} />
                      <col style={{ width: '72px' }} />
                    </colgroup>
                    <thead>
                      <tr>
                        <th style={thSt} />
                        <th style={thSt}>{t('blockedColAddr')}</th>
                        <th style={thSt}>{t('blockedColDate')}</th>
                        <th style={{ ...thSt, textAlign: 'center' }}>{t('blockedColAction')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {pageItems.map((addr) => {
                        const isDomain = addr.startsWith('@');
                        return (
                          <tr key={addr}
                            onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-bg-secondary)'; }}
                            onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                          >
                            <td style={{ ...tdSt, textAlign: 'center' }}>
                              {isDomain
                                ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-warning)', display: 'inline-block' }} />
                                : <NoSymbolIcon style={{ width: 14, height: 14, color: 'var(--color-destructive)', display: 'inline-block' }} />
                              }
                            </td>
                            <td style={{ ...tdSt, fontFamily: 'monospace', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {addr}
                            </td>
                            <td style={{ ...tdSt, fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                              {formatBlockedDate(addr)}
                            </td>
                            <td style={{ ...tdSt, textAlign: 'center' }}>
                              <button
                                onClick={() => removeBlocked(addr)}
                                style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
                                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 10%, transparent)'; }}
                                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                              >{t('unblock')}</button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '6px', padding: '10px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
                  <button
                    onClick={() => setBlockedPage((p) => Math.max(0, p - 1))}
                    disabled={safePage === 0}
                    style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === 0 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === 0 ? 'default' : 'pointer', fontSize: '12px' }}
                  >‹</button>
                  <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '80px', textAlign: 'center' }}>
                    {safePage + 1} / {totalPages}
                  </span>
                  <button
                    onClick={() => setBlockedPage((p) => Math.min(totalPages - 1, p + 1))}
                    disabled={safePage === totalPages - 1}
                    style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === totalPages - 1 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === totalPages - 1 ? 'default' : 'pointer', fontSize: '12px' }}
                  >›</button>
                </div>
              )}
            </SectionCard>

            {/* ── 발신자/도메인 차단 추가 ── */}
            <SectionCard>
              <SectionHeader>{t('sectionAddBlockedSender')}</SectionHeader>
              <div style={{ padding: '4px 20px 8px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                {t('blockedInputHint')}
              </div>
              <div style={{ padding: '0 20px 20px' }}>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
                  <div style={{ flex: 1, position: 'relative' }}>
                    <input
                      value={newBlockedInput}
                      onChange={(e) => setNewBlockedInput(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') addBlocked(); }}
                      placeholder={t('blockedInputPlaceholder')}
                      style={{
                        width: '100%', boxSizing: 'border-box',
                        padding: '9px 12px',
                        border: '1px solid var(--color-border-default)',
                        borderRadius: '7px',
                        background: 'var(--color-bg-primary)',
                        color: 'var(--color-text-primary)',
                        fontSize: '13px', outline: 'none',
                        fontFamily: 'monospace',
                        transition: 'border-color 120ms',
                      }}
                      onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                      onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                    />
                  </div>
                  <button
                    onClick={addBlocked}
                    disabled={!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase())}
                    style={{
                      padding: '9px 20px', borderRadius: '7px', border: 'none',
                      background: 'var(--color-accent)', color: '#fff',
                      fontSize: '13px', fontWeight: 600,
                      cursor: newBlockedInput.trim() && !blockedSenders.includes(newBlockedInput.trim().toLowerCase()) ? 'pointer' : 'default',
                      opacity: newBlockedInput.trim() && !blockedSenders.includes(newBlockedInput.trim().toLowerCase()) ? 1 : 0.4,
                      flexShrink: 0, whiteSpace: 'nowrap',
                      transition: 'opacity 120ms',
                    }}
                    onMouseEnter={(e) => { if (!(!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase()))) (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
                    onMouseLeave={(e) => { if (!(!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase()))) (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
                  >{t('block')}</button>
                </div>
                {newBlockedInput.trim() && blockedSenders.includes(newBlockedInput.trim().toLowerCase()) && (
                  <div style={{ marginTop: '6px', fontSize: '12px', color: 'var(--color-warning)' }}>
                    {t('blockedAlready')}
                  </div>
                )}
              </div>
            </SectionCard>

            {/* ── 허용된 발신자 목록 ── */}
            {(() => {
              const aq = allowedSearch.trim().toLowerCase();
              const filteredAllowed = aq ? allowedSenders.filter((a) => a.includes(aq)) : allowedSenders;
              const allowedTotalPages = Math.ceil(filteredAllowed.length / PAGE_SIZE);
              const safeAllowedPage = Math.min(allowedPage, Math.max(0, allowedTotalPages - 1));
              const allowedPageItems = filteredAllowed.slice(safeAllowedPage * PAGE_SIZE, (safeAllowedPage + 1) * PAGE_SIZE);

              function saveAllowed(next: string[], meta?: Record<string, string>) {
                try { localStorage.setItem('webmail_allowed_senders', JSON.stringify(next)); } catch { /* */ }
                setAllowedSenders(next);
                if (meta !== undefined) {
                  try { localStorage.setItem('webmail_allowed_meta', JSON.stringify(meta)); } catch { /* */ }
                  setAllowedMeta(meta);
                }
                void setPreferences({ allowed_senders: next });
              }
              function addAllowed() {
                const val = newAllowedInput.trim().toLowerCase();
                if (!val || allowedSenders.includes(val)) return;
                const now = new Date().toISOString();
                saveAllowed([...allowedSenders, val], { ...allowedMeta, [val]: now });
                setNewAllowedInput('');
                setAllowedPage(Math.floor(allowedSenders.length / PAGE_SIZE));
              }
              function removeAllowed(addr: string) {
                const next = allowedSenders.filter((a) => a !== addr);
                const nextMeta = { ...allowedMeta };
                delete nextMeta[addr];
                saveAllowed(next, nextMeta);
                const newTotal = Math.ceil(next.filter((a) => aq ? a.includes(aq) : true).length / PAGE_SIZE);
                if (safeAllowedPage >= newTotal && safeAllowedPage > 0) setAllowedPage(safeAllowedPage - 1);
              }
              function formatAllowedDate(addr: string): string {
                const iso = allowedMeta[addr];
                if (!iso) return '—';
                try {
                  return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso));
                } catch { return iso.slice(0, 10); }
              }
              const allowedValTrimmed = newAllowedInput.trim().toLowerCase();
              return (
                <>
                  <SectionCard>
                    <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '16px 20px 0', flexWrap: 'wrap' }}>
                      <div style={{ flex: 1 }}>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('sectionAllowedSenders')}</div>
                        <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('allowedSendersDesc')}</div>
                      </div>
                      {allowedSenders.length > 0 && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                          <div style={{ position: 'relative' }}>
                            <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: 'var(--color-text-tertiary)', pointerEvents: 'none' }} />
                            <input
                              type="text"
                              value={allowedSearch}
                              onChange={(e) => { setAllowedSearch(e.target.value); setAllowedPage(0); }}
                              placeholder={t('blockedSearchPlaceholder')}
                              style={{ paddingLeft: 26, paddingRight: 8, paddingTop: 5, paddingBottom: 5, width: 190, fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', fontFamily: 'monospace' }}
                              onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                              onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                            />
                            {allowedSearch && (
                              <button onClick={() => { setAllowedSearch(''); setAllowedPage(0); }} style={{ position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--color-text-tertiary)', lineHeight: 1, fontSize: 14 }}>×</button>
                            )}
                          </div>
                          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                            {aq ? t('blockedSearchCount', { found: filteredAllowed.length, total: allowedSenders.length }) : t('blockedCount', { count: allowedSenders.length })}
                          </span>
                        </div>
                      )}
                    </div>

                    <div style={{ overflowX: 'auto', margin: '12px 0 0' }}>
                      {allowedSenders.length === 0 ? (
                        <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
                          {t('noAllowed')}
                        </div>
                      ) : filteredAllowed.length === 0 ? (
                        <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
                          {t('blockedSearchEmpty')}
                        </div>
                      ) : (
                        <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
                          <colgroup>
                            <col style={{ width: '40px' }} />
                            <col />
                            <col style={{ width: '160px' }} />
                            <col style={{ width: '72px' }} />
                          </colgroup>
                          <thead>
                            <tr>
                              <th style={thSt} />
                              <th style={thSt}>{t('allowedColAddr')}</th>
                              <th style={thSt}>{t('allowedColDate')}</th>
                              <th style={{ ...thSt, textAlign: 'center' }}>{t('blockedColAction')}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {allowedPageItems.map((addr) => {
                              const isDomain = addr.startsWith('@');
                              return (
                                <tr key={addr}
                                  onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-bg-secondary)'; }}
                                  onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                                >
                                  <td style={{ ...tdSt, textAlign: 'center' }}>
                                    {isDomain
                                      ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                                      : <CheckCircleIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                                    }
                                  </td>
                                  <td style={{ ...tdSt, fontFamily: 'monospace', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                    {addr}
                                  </td>
                                  <td style={{ ...tdSt, fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                                    {formatAllowedDate(addr)}
                                  </td>
                                  <td style={{ ...tdSt, textAlign: 'center' }}>
                                    <button
                                      onClick={() => removeAllowed(addr)}
                                      style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
                                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                                    >{t('disallow')}</button>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      )}
                    </div>

                    {allowedTotalPages > 1 && (
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '6px', padding: '10px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
                        <button onClick={() => setAllowedPage((p) => Math.max(0, p - 1))} disabled={safeAllowedPage === 0} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safeAllowedPage === 0 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safeAllowedPage === 0 ? 'default' : 'pointer', fontSize: '12px' }}>‹</button>
                        <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '80px', textAlign: 'center' }}>{safeAllowedPage + 1} / {allowedTotalPages}</span>
                        <button onClick={() => setAllowedPage((p) => Math.min(allowedTotalPages - 1, p + 1))} disabled={safeAllowedPage === allowedTotalPages - 1} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safeAllowedPage === allowedTotalPages - 1 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safeAllowedPage === allowedTotalPages - 1 ? 'default' : 'pointer', fontSize: '12px' }}>›</button>
                      </div>
                    )}
                  </SectionCard>

                  {/* ── 허용 발신자 추가 ── */}
                  <SectionCard>
                    <SectionHeader>{t('sectionAddAllowedSender')}</SectionHeader>
                    <div style={{ padding: '4px 20px 8px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                      {t('allowedInputHint')}
                    </div>
                    <div style={{ padding: '0 20px 20px' }}>
                      <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
                        <input
                          value={newAllowedInput}
                          onChange={(e) => setNewAllowedInput(e.target.value)}
                          onKeyDown={(e) => { if (e.key === 'Enter') addAllowed(); }}
                          placeholder={t('allowedInputPlaceholder')}
                          style={{ flex: 1, boxSizing: 'border-box', padding: '9px 12px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none', fontFamily: 'monospace', transition: 'border-color 120ms' }}
                          onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                          onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                        />
                        <button
                          onClick={addAllowed}
                          disabled={!allowedValTrimmed || allowedSenders.includes(allowedValTrimmed)}
                          style={{ padding: '9px 20px', borderRadius: '7px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed) ? 'pointer' : 'default', opacity: allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed) ? 1 : 0.4, flexShrink: 0, whiteSpace: 'nowrap', transition: 'opacity 120ms' }}
                          onMouseEnter={(e) => { if (allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
                          onMouseLeave={(e) => { if (allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
                        >{t('allow')}</button>
                      </div>
                      {allowedValTrimmed && allowedSenders.includes(allowedValTrimmed) && (
                        <div style={{ marginTop: '6px', fontSize: '12px', color: 'var(--color-warning)' }}>
                          {t('allowedAlready')}
                        </div>
                      )}
                    </div>
                  </SectionCard>
                </>
              );
            })()}
          </>
        );
      }

      case 'vacation': {
        const inSt: React.CSSProperties = {
          border: '1px solid var(--color-border-default)', borderRadius: '6px',
          padding: '7px 10px', fontSize: '13px', background: 'var(--color-bg-primary)',
          color: 'var(--color-text-primary)', outline: 'none', width: '100%',
        };
        function saveVac() {
          try {
            localStorage.setItem('webmail_vacation', JSON.stringify({
              enabled: vacEnabled, startDate: vacStartDate, endDate: vacEndDate,
              subject: vacSubject, body: vacBody,
            }));
          } catch { /* ignore */ }
          setVacSaved(true);
          setTimeout(() => setVacSaved(false), 2000);
        }
        return (
          <>
            <SectionCard>
              <SectionHeader>{t('sectionVacationResponder')}</SectionHeader>
              <Row label={t('vacEnabled')} description={t('vacEnabledDesc')}>
                <Toggle value={vacEnabled} onChange={setVacEnabled} />
              </Row>
              <Row label={t('vacStart')} last={false}>
                <input type="date" value={vacStartDate} onChange={(e) => setVacStartDate(e.target.value)} style={{ ...inSt, width: '160px' }} disabled={!vacEnabled} />
              </Row>
              <Row label={t('vacEnd')} last>
                <input type="date" value={vacEndDate} onChange={(e) => setVacEndDate(e.target.value)} style={{ ...inSt, width: '160px' }} disabled={!vacEnabled} />
              </Row>
            </SectionCard>

            <SectionCard>
              <SectionHeader>{t('sectionVacationMessage')}</SectionHeader>
              <div style={{ padding: '0 20px 16px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: '5px' }}>{t('subject')}</label>
                  <input
                    value={vacSubject}
                    onChange={(e) => setVacSubject(e.target.value)}
                    disabled={!vacEnabled}
                    style={inSt}
                    placeholder={t('vacSubjectDefault')}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: '5px' }}>{t('body')}</label>
                  <div style={{ opacity: vacEnabled ? 1 : 0.5, pointerEvents: vacEnabled ? 'auto' : 'none' }}>
                    <MiniEditor
                      value={vacBody}
                      onChange={(html) => { setVacBody(html); }}
                      placeholder={t('vacBodyPlaceholder')}
                    />
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  {vacEnabled && vacStartDate && vacEndDate && (
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                      {t('vacDateRange', { start: vacStartDate, end: vacEndDate })}
                    </span>
                  )}
                  <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '10px' }}>
                    {vacSaved && (
                      <span style={{ fontSize: '12px', color: 'var(--color-accent)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                        <CheckIcon style={{ width: 13, height: 13 }} /> {t('saved')}
                      </span>
                    )}
                    <button
                      onClick={saveVac}
                      style={{ padding: '6px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                    >{t('save')}</button>
                  </div>
                </div>
              </div>
            </SectionCard>
          </>
        );
      }

      case 'privacy':
        return (
          <SettingsPrivacySection
            blockTrackingPixels={blockTrackingPixels}
            setBlockTrackingPixels={setBlockTrackingPixels}
            linkPreview={linkPreview}
            setLinkPreview={setLinkPreview}
            requestReadReceipt={requestReadReceipt}
            setRequestReadReceipt={setRequestReadReceipt}
            followUpDays={followUpDays}
            setFollowUpDays={setFollowUpDays}
          />
        );

      case 'appearance':
        return (
          <>
            <SectionCard>
              <SectionHeader>{t('sectionTheme')}</SectionHeader>
              <Row label={t('themeMode')} description={t('themeModeDesc')} last>
                <Segment
                  options={[{ value: 'light' as Theme, label: t('themeLight') }, { value: 'dark' as Theme, label: t('themeDark') }, { value: 'system' as Theme, label: t('themeSystem') }]}
                  value={theme}
                  onChange={applyTheme}
                />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>{t('sectionAccent')}</SectionHeader>
              <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '14px' }}>{t('accentDesc')}</div>
                <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
                  {ACCENT_COLORS.map((c) => (
                    <button
                      key={c.value}
                      title={c.label}
                      onClick={() => applyAccent(c.value)}
                      style={{ width: '28px', height: '28px', borderRadius: '50%', background: c.value, border: `2.5px solid ${accent === c.value ? 'var(--color-text-primary)' : 'transparent'}`, cursor: 'pointer', padding: 0, boxShadow: accent === c.value ? `0 0 0 1.5px ${c.value}` : 'none', transition: 'border-color 120ms ease', flexShrink: 0 }}
                    />
                  ))}
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginLeft: '4px' }}>
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('accentCustom')}</span>
                    <input
                      type="text"
                      value={customAccent}
                      onChange={(e) => setCustomAccent(e.target.value)}
                      placeholder="#2563eb"
                      style={{ width: '80px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '12px', fontFamily: 'monospace', outline: 'none' }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          const hex = customAccent.startsWith('#') ? customAccent : `#${customAccent}`;
                          if (/^#[0-9a-f]{6}$/i.test(hex)) { applyAccent(hex); setAccent(hex); }
                        }
                      }}
                    />
                  </div>
                </div>
              </div>
            </SectionCard>
            <SectionCard>
              <SectionHeader>{t('sectionDensityFont')}</SectionHeader>
              <Row label={t('compactView')} description={t('compactViewDesc')}>
                <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label={t('fontSizeBody')} description={t('fontSizeBodyDesc')} last>
                <Segment
                  options={[{ value: 'small' as FontSize, label: t('fontSizeSmallPx') }, { value: 'medium' as FontSize, label: t('fontSizeMediumPx') }, { value: 'large' as FontSize, label: t('fontSizeLargePx') }]}
                  value={fontSize}
                  onChange={applyFontSize}
                />
              </Row>
            </SectionCard>
          </>
        );

      case 'notifications':
        const setBadgeMode = (mode: BadgeCountMode) => {
          setBadgeCountMode(mode);
          try {
            localStorage.setItem(BADGE_COUNT_MODE_KEY, mode);
            window.dispatchEvent(new StorageEvent('storage', { key: BADGE_COUNT_MODE_KEY, newValue: mode }));
          } catch {
            // local settings cache is best-effort
          }
        };
        const setFolderNotificationEnabled = (folderId: string, enabled: boolean) => {
          setNotificationFolderOverrides((prev) => {
            const next = { ...prev };
            if (enabled) {
              delete next[folderId];
            } else {
              next[folderId] = { enabled: false, dnd_inherit: true, dnd_schedule: emptyDNDSchedule() };
            }
            try {
              localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(next));
            } catch {
              // local settings cache is best-effort
            }
            return next;
          });
        };
        const setBrowserMirroring = (enabled: boolean) => {
          setBrowserNotificationsEnabled(enabled);
          try {
            localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, enabled ? 'true' : 'false');
            window.dispatchEvent(new StorageEvent('storage', { key: BROWSER_NOTIF_ENABLED_KEY, newValue: enabled ? 'true' : 'false' }));
          } catch {
            // local settings cache is best-effort
          }
        };
        return (
          <SettingsNotificationsSection
            notifPerm={notifPerm}
            notifSyncError={notifSyncError}
            onRequestNotif={requestNotif}
            browserNotificationsEnabled={browserNotificationsEnabled}
            setBrowserNotificationsEnabled={setBrowserMirroring}
            notifSound={notifSound}
            setNotifSound={(v) => { setNotifSound(v); try { localStorage.setItem('webmail_notif_sound', v ? '1' : '0'); } catch { /* */ } }}
            notifDetail={notifDetail}
            setNotifDetail={(v) => { setNotifDetail(v); try { localStorage.setItem('webmail_notif_detail', v); } catch { /* */ } }}
            badgeCountMode={badgeCountMode}
            setBadgeCountMode={setBadgeMode}
            dndEnabled={dndEnabled}
            setDndEnabled={(v) => { setDndEnabled(v); try { localStorage.setItem('webmail_dnd', v ? '1' : '0'); } catch { /* */ } }}
            dndStart={dndStart}
            setDndStart={(v) => { setDndStart(v); try { localStorage.setItem('webmail_dnd_start', v); } catch { /* */ } }}
            dndEnd={dndEnd}
            setDndEnd={(v) => { setDndEnd(v); try { localStorage.setItem('webmail_dnd_end', v); } catch { /* */ } }}
            folders={notificationFolders}
            folderOverrides={notificationFolderOverrides}
            setFolderNotificationEnabled={setFolderNotificationEnabled}
            webPushEnabled={webPushEnabled}
            setWebPushEnabled={(v) => {
              setWebPushEnabled(v);
              try { localStorage.setItem('webmail_webpush_enabled', v ? 'true' : 'false'); } catch { /* */ }
            }}
            webPushSupported={webPushSupported}
          />
        );

      case 'shortcuts':
        return (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            {SHORTCUT_GROUPS.map((group) => (
              <SectionCard key={group.titleKey}>
                <SectionHeader>{t(group.titleKey)}</SectionHeader>
                <div style={{ background: 'var(--color-bg-primary)' }}>
                  {group.items.map(([key, descKey], i) => (
                    <div key={key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '9px 16px', borderBottom: i < group.items.length - 1 ? '1px solid var(--color-border-subtle)' : 'none' }}>
                      <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1 }}>{t(descKey)}</span>
                      <Kbd k={key} />
                    </div>
                  ))}
                </div>
              </SectionCard>
            ))}
          </div>
        );

      case 'security': {
        return (
          <SettingsSecuritySection
            userEmail={userEmail}
            revokingAll={revokingAll}
            revokeAllError={revokeAllError}
            onRevokeAll={handleRevokeAll}
          />
        );
      }

      case 'mcp':
        return <SettingsMCPSection />;

      case 'accessibility':
        return (
          <>
            {/* ── 시각 보조 ─────────────────────────────────── */}
            <SectionCard>
              <SectionHeader>{t('sectionVisualAids')}</SectionHeader>
              <Row label={t('highContrast')} description={t('highContrastDesc')}>
                <Toggle value={highContrast} onChange={(v) => { setHighContrast(v); try { localStorage.setItem('webmail_high_contrast', v ? '1' : '0'); document.documentElement.classList.toggle('high-contrast', v); } catch { /* */ } }} />
              </Row>
              <Row label={t('colorBlindMode')} description={t('colorBlindModeDesc')}>
                <Segment
                  options={[
                    { value: 'none' as const, label: t('colorBlindNone') },
                    { value: 'deuteranopia' as const, label: t('colorBlindDeuteranopia') },
                    { value: 'protanopia' as const, label: t('colorBlindProtanopia') },
                    { value: 'tritanopia' as const, label: t('colorBlindTritanopia') },
                  ]}
                  value={colorBlindMode}
                  onChange={applyColorBlindMode}
                />
              </Row>
              <Row label={t('underlineLinks')} description={t('underlineLinksDesc')}>
                <Toggle value={underlineLinks} onChange={(v) => { setUnderlineLinks(v); try { localStorage.setItem('webmail_underline_links', v ? '1' : '0'); document.documentElement.classList.toggle('underline-links', v); } catch { /* */ } }} />
              </Row>
              <Row label={t('reducedMotion')} description={t('reducedMotionDesc')} last>
                <Toggle value={reducedMotion} onChange={(v) => { setReducedMotion(v); try { localStorage.setItem('webmail_reduced_motion', v ? '1' : '0'); document.documentElement.classList.toggle('reduced-motion', v); } catch { /* */ } }} />
              </Row>
            </SectionCard>

            {/* ── 글꼴 및 가독성 ────────────────────────────── */}
            <SectionCard>
              <SectionHeader>{t('sectionTypography')}</SectionHeader>
              <Row label={t('fontFamily')} description={t('fontFamilyDesc')}>
                <Segment
                  options={[
                    { value: 'system' as const, label: t('fontFamilySystem') },
                    { value: 'serif' as const, label: t('fontFamilySerif') },
                    { value: 'mono' as const, label: t('fontFamilyMono') },
                  ]}
                  value={fontFamily}
                  onChange={(v) => {
                    setFontFamily(v);
                    try {
                      localStorage.setItem('webmail_font_family', v);
                      const map: Record<string, string> = { system: '', serif: 'Georgia, serif', mono: '"JetBrains Mono", "Fira Code", monospace' };
                      document.documentElement.style.fontFamily = map[v] ?? '';
                    } catch { /* */ }
                  }}
                />
              </Row>
              <Row label={t('dyslexiaMode')} description={t('dyslexiaModeDesc')}>
                <Toggle value={dyslexiaMode} onChange={(v) => { setDyslexiaMode(v); try { localStorage.setItem('webmail_dyslexia', v ? '1' : '0'); document.documentElement.classList.toggle('dyslexia-mode', v); } catch { /* */ } }} />
              </Row>
              <Row label={t('uiFontSize')} description={t('uiFontSizeDesc')}>
                <Segment
                  options={[
                    { value: 'sm' as const, label: t('fontSizeSm') },
                    { value: 'md' as const, label: t('fontSizeMd') },
                    { value: 'lg' as const, label: t('fontSizeLg') },
                    { value: 'xl' as const, label: t('fontSizeXl') },
                  ]}
                  value={uiFontSize}
                  onChange={applyUiFontSize}
                />
              </Row>
              <Row label={t('lineSpacing')} description={t('lineSpacingDesc')}>
                <Segment
                  options={[
                    { value: 'normal' as const, label: t('lineSpacingNormal') },
                    { value: 'relaxed' as const, label: t('lineSpacingRelaxed') },
                    { value: 'loose' as const, label: t('lineSpacingLoose') },
                  ]}
                  value={lineSpacing}
                  onChange={applyLineSpacing}
                />
              </Row>
              <Row label={t('letterSpacing')} description={t('letterSpacingDesc')} last>
                <Segment
                  options={[
                    { value: 'normal' as const, label: t('letterSpacingNormal') },
                    { value: 'wide' as const, label: t('letterSpacingWide') },
                  ]}
                  value={letterSpacing}
                  onChange={applyLetterSpacing}
                />
              </Row>
            </SectionCard>

            {/* ── 키보드 및 포커스 ──────────────────────────── */}
            <SectionCard>
              <SectionHeader>{t('sectionKeyboardFocus')}</SectionHeader>
              <Row label={t('alwaysFocusRing')} description={t('alwaysFocusRingDesc')}>
                <Toggle value={alwaysFocusRing} onChange={(v) => { setAlwaysFocusRing(v); try { localStorage.setItem('webmail_always_focus_ring', v ? '1' : '0'); document.documentElement.classList.toggle('always-focus-ring', v); } catch { /* */ } }} />
              </Row>
              <Row label={t('largerClickTargets')} description={t('largerClickTargetsDesc')} last>
                <Toggle value={largerClickTargets} onChange={(v) => { setLargerClickTargets(v); try { localStorage.setItem('webmail_larger_targets', v ? '1' : '0'); document.documentElement.classList.toggle('larger-targets', v); } catch { /* */ } }} />
              </Row>
            </SectionCard>

            {/* ── 스크린리더 ────────────────────────────────── */}
            <SectionCard>
              <SectionHeader>{t('sectionScreenReader')}</SectionHeader>
              <Row label={t('screenReaderMode')} description={t('screenReaderModeDesc')} last>
                <Toggle value={screenReaderMode} onChange={(v) => { setScreenReaderMode(v); try { localStorage.setItem('webmail_screen_reader', v ? '1' : '0'); document.documentElement.classList.toggle('screen-reader-mode', v); } catch { /* */ } }} />
              </Row>
            </SectionCard>
          </>
        );

      case 'about':
        return <SettingsAboutSection />;

      case 'storage': {
        const startBackup = async (folderId: string, folderName: string, format: 'eml' | 'zip') => {
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
        };
        return (
          <SettingsStorageSection
            folderStats={folderStats}
            statsLoading={statsLoading}
            backupStates={backupStates}
            onLoadStats={() => { setStatsLoading(true); getFolderStats().then(setFolderStats).catch(() => {}).finally(() => setStatsLoading(false)); }}
            onStartBackup={startBackup}
          />
        );
      }

      default:
        return null;
    }
  }

  const currentNav = NAV_ITEMS.find((n) => n.id === activeSection);

  return (
    <div style={{ flex: 1, minWidth: 0, height: '100%', display: 'flex', overflow: 'hidden', background: 'var(--color-bg-primary)' }}>
      {/* Left sidebar nav */}
      <div style={{ width: '200px', flexShrink: 0, height: '100%', overflowY: 'auto', borderRight: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', padding: '20px 0' }}>
        <div style={{ padding: '0 12px 16px', fontSize: '11px', fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>{t('settingsTitle')}</div>
        {NAV_ITEMS.map((item) => {
          const active = item.id === activeSection;
          return (
          <button
              key={item.id}
              onClick={() => { setActiveSection(item.id); contentRef.current?.scrollTo({ top: 0 }); }}
              data-nav-group="settings-nav"
              onKeyDown={(e) => handleVerticalNavKeyDown(e, 'settings-nav')}
              style={{
                display: 'flex', alignItems: 'center', gap: '9px',
                width: '100%', padding: '8px 14px 8px 12px',
                border: 'none', borderLeft: `2px solid ${active ? 'var(--color-accent)' : 'transparent'}`,
                background: active ? 'var(--color-accent-subtle)' : 'transparent',
                color: active ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                fontSize: '13px', fontWeight: active ? 600 : 400,
                cursor: 'pointer', textAlign: 'left',
                transition: 'background 100ms ease, color 100ms ease',
              }}
              onMouseEnter={(e) => { if (!active) { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; } }}
              onMouseLeave={(e) => { if (!active) { (e.currentTarget).style.background = 'transparent'; (e.currentTarget).style.color = 'var(--color-text-secondary)'; } }}
            >
              <span style={{ flexShrink: 0, opacity: active ? 1 : 0.7 }}>{item.icon}</span>
              {t(item.labelKey)}
            </button>
          );
        })}
      </div>

      {/* Content area */}
      <div ref={contentRef} style={{ flex: 1, minWidth: 0, height: '100%', overflowY: 'auto', padding: '32px 40px' }}>
        <h2 style={{ fontSize: '17px', fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: '24px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ color: 'var(--color-text-tertiary)', display: 'flex' }}>{currentNav?.icon}</span>
          {currentNav ? t(currentNav.labelKey) : null}
        </h2>
        {renderContent()}
      </div>
    </div>
  );
}
