'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { revokeAllSessions, getFolderStats, exportFolderEml, exportFolderZip, getPreferences, setPreferences, getUserProfile, updateUserProfile, uploadUserAvatar, deleteUserAvatar, changePassword, registerWebPushDevice, getNotificationPreferences, setNotificationPreferences, getFolders, type FolderStats, type WebmailPreferences, type UserProfile, type NotificationPreferences, type FolderNotificationOverride, type Folder } from '@/lib/api';
import { ReadMark, ExternalImages, SendDelay, Theme, FontSize, FilterRule, migrateFilterRule, loadFilterRules, saveFilterRules } from '@/lib/settings/settingsUtils';
import { NAV_ITEMS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { loadWmSettings, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';
import { FilterRulesSection } from '@/components/settings-view/FilterRulesSection';
import { SettingsAboutSection } from '@/components/settings-view/SettingsAboutSection';
import { SettingsStorageSection, type BackupState } from '@/components/settings-view/SettingsStorageSection';
import { SettingsPrivacySection } from '@/components/settings-view/SettingsPrivacySection';
import { SettingsNotificationsSection } from '@/components/settings-view/SettingsNotificationsSection';
import { SettingsSecuritySection } from '@/components/settings-view/SettingsSecuritySection';
import { SettingsMCPSection } from '@/components/settings-view/SettingsMCPSection';
import { SettingsBlockedSection } from '@/components/settings-view/SettingsBlockedSection';
import { SettingsVacationSection } from '@/components/settings-view/SettingsVacationSection';
import { SettingsAccountSection } from '@/components/settings-view/SettingsAccountSection';
import { SettingsInboxSection } from '@/components/settings-view/SettingsInboxSection';
import { SettingsReadingSection } from '@/components/settings-view/SettingsReadingSection';
import { SettingsComposeSection } from '@/components/settings-view/SettingsComposeSection';
import { SettingsAppearanceSection } from '@/components/settings-view/SettingsAppearanceSection';
import { SettingsAccessibilitySection } from '@/components/settings-view/SettingsAccessibilitySection';
import { SettingsContactsSection } from '@/components/settings-view/SettingsContactsSection';
import { SettingsDriveSection } from '@/components/settings-view/SettingsDriveSection';
import { SettingsShortcutsSection } from '@/components/settings-view/SettingsShortcutsSection';
import { handleVerticalNavKeyDown } from '@/lib/navKeyboard';
import { webPushPublicKeyToUint8Array } from '@/lib/webpush';
import { loadLocalEmailTemplates, normalizeEmailTemplates, saveLocalEmailTemplates, type StoredEmailTemplate } from '@/lib/emailTemplates';
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

  // Auto-load folder stats when storage section becomes active
  useEffect(() => {
    if (activeSection !== 'storage') return;
    if (folderStats.length > 0 || statsLoading) return; // already loaded
    setStatsLoading(true);
    getFolderStats().then(setFolderStats).catch(() => {}).finally(() => setStatsLoading(false));
  }, [activeSection]);

  // ─── Render ──────────────────────────────────────────────────────────────────

  function renderContent() {
    switch (activeSection) {

      case 'account':
        return (
          <SettingsAccountSection
            userEmail={userEmail}
            userName={userName}
            displayName={displayName}
            setDisplayName={setDisplayName}
            nameSaved={nameSaved}
            avatarUrl={avatarUrl}
            avatarSaving={avatarSaving}
            avatarError={avatarError}
            recoveryEmail={recoveryEmail}
            setRecoveryEmail={setRecoveryEmail}
            recoverySaved={recoverySaved}
            recoveryError={recoveryError}
            signature={signature}
            setSignature={setSignature}
            sigSaved={sigSaved}
            profile={profile}
            timezone={timezone}
            setTimezone={setTimezone}
            pwCurrent={pwCurrent}
            setPwCurrent={setPwCurrent}
            pwNew={pwNew}
            setPwNew={setPwNew}
            pwConfirm={pwConfirm}
            setPwConfirm={setPwConfirm}
            pwError={pwError}
            pwSaving={pwSaving}
            pwSaved={pwSaved}
            onAvatarUpload={(file) => { void handleAvatarUpload(file); }}
            onAvatarRemove={handleAvatarRemove}
            onSaveName={saveDisplayName}
            onSaveRecovery={saveRecoveryEmail}
            onSaveSignature={saveSignature}
            onChangePassword={handleChangePassword}
          />
        );

      case 'inbox':
        return (
          <SettingsInboxSection
            convMode={convMode}
            setConvMode={setConvMode}
            compact={compact}
            setCompact={setCompact}
            showPreview={showPreview}
            setShowPreview={setShowPreview}
            refreshInterval={refreshInterval}
            setRefreshInterval={setRefreshInterval}
            importanceMarkers={importanceMarkers}
            setImportanceMarkers={setImportanceMarkers}
            groupByDate={groupByDate}
            setGroupByDate={setGroupByDate}
            focusMode={focusMode}
            setFocusMode={setFocusMode}
            swipeLeft={swipeLeft}
            setSwipeLeft={setSwipeLeft}
            swipeRight={swipeRight}
            setSwipeRight={setSwipeRight}
          />
        );

      case 'contacts':
        return (
          <SettingsContactsSection
            contactsSort={contactsSort}
            setContactsSort={setContactsSort}
            contactsDensity={contactsDensity}
            setContactsDensity={setContactsDensity}
            contactsShowCompany={contactsShowCompany}
            setContactsShowCompany={setContactsShowCompany}
          />
        );

      case 'drive':
        return (
          <SettingsDriveSection
            driveSort={driveSort}
            setDriveSort={setDriveSort}
          />
        );

      case 'reading':
        return (
          <SettingsReadingSection
            readMark={readMark}
            setReadMark={setReadMark}
            externalImages={externalImages}
            setExternalImages={setExternalImages}
            inlineImagePreview={inlineImagePreview}
            setInlineImagePreview={setInlineImagePreview}
            smartReplySuggestions={smartReplySuggestions}
            setSmartReplySuggestions={setSmartReplySuggestions}
            showReadingTime={showReadingTime}
            setShowReadingTime={setShowReadingTime}
            readingPanePosition={readingPanePosition}
            setReadingPanePosition={setReadingPanePosition}
          />
        );

      case 'compose':
        return (
          <SettingsComposeSection
            sendDelay={sendDelay}
            setSendDelay={setSendDelay}
            quoteOnReply={quoteOnReply}
            setQuoteOnReply={setQuoteOnReply}
            fontSize={fontSize}
            ccSelf={ccSelf}
            setCcSelf={setCcSelf}
            defaultBcc={defaultBcc}
            setDefaultBcc={setDefaultBcc}
            confirmBeforeSend={confirmBeforeSend}
            setConfirmBeforeSend={setConfirmBeforeSend}
            spellCheck={spellCheck}
            setSpellCheck={setSpellCheck}
            templates={templates}
            setTemplates={setTemplates}
            newTplName={newTplName}
            setNewTplName={setNewTplName}
            newTplSubject={newTplSubject}
            setNewTplSubject={setNewTplSubject}
            newTplBody={newTplBody}
            setNewTplBody={setNewTplBody}
            showNewTpl={showNewTpl}
            setShowNewTpl={setShowNewTpl}
            applyFontSize={applyFontSize}
          />
        );

      case 'filters': {
        return <FilterRulesSection filterRules={filterRules} setFilterRules={setFilterRules} />;
      }

      case 'blocked': {
        return (
          <SettingsBlockedSection
            blockedSenders={blockedSenders}
            setBlockedSenders={setBlockedSenders}
            blockedMeta={blockedMeta}
            setBlockedMeta={setBlockedMeta}
            newBlockedInput={newBlockedInput}
            setNewBlockedInput={setNewBlockedInput}
            blockedSearch={blockedSearch}
            setBlockedSearch={setBlockedSearch}
            blockedPage={blockedPage}
            setBlockedPage={setBlockedPage}
            spamAutoDeleteDays={spamAutoDeleteDays}
            setSpamAutoDeleteDays={setSpamAutoDeleteDays}
            spamAutoBlock={spamAutoBlock}
            setSpamAutoBlock={setSpamAutoBlock}
            allowedSenders={allowedSenders}
            setAllowedSenders={setAllowedSenders}
            allowedMeta={allowedMeta}
            setAllowedMeta={setAllowedMeta}
            newAllowedInput={newAllowedInput}
            setNewAllowedInput={setNewAllowedInput}
            allowedSearch={allowedSearch}
            setAllowedSearch={setAllowedSearch}
            allowedPage={allowedPage}
            setAllowedPage={setAllowedPage}
          />
        );
      }

      case 'vacation': {
        return (
          <SettingsVacationSection
            vacEnabled={vacEnabled}
            setVacEnabled={setVacEnabled}
            vacStartDate={vacStartDate}
            setVacStartDate={setVacStartDate}
            vacEndDate={vacEndDate}
            setVacEndDate={setVacEndDate}
            vacSubject={vacSubject}
            setVacSubject={setVacSubject}
            vacBody={vacBody}
            setVacBody={setVacBody}
            vacSaved={vacSaved}
            setVacSaved={setVacSaved}
          />
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
          <SettingsAppearanceSection
            theme={theme}
            accent={accent}
            setAccent={setAccent}
            customAccent={customAccent}
            setCustomAccent={setCustomAccent}
            compact={compact}
            setCompact={setCompact}
            fontSize={fontSize}
            applyTheme={applyTheme}
            applyAccent={applyAccent}
            applyFontSize={applyFontSize}
          />
        );

      case 'notifications':
        return (
          <SettingsNotificationsSection
            notifPerm={notifPerm}
            notifSyncError={notifSyncError}
            onRequestNotif={requestNotif}
            browserNotificationsEnabled={browserNotificationsEnabled}
            setBrowserNotificationsEnabled={(enabled) => {
              setBrowserNotificationsEnabled(enabled);
              try {
                localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, enabled ? 'true' : 'false');
                window.dispatchEvent(new StorageEvent('storage', { key: BROWSER_NOTIF_ENABLED_KEY, newValue: enabled ? 'true' : 'false' }));
              } catch { /* */ }
            }}
            notifSound={notifSound}
            setNotifSound={(v) => { setNotifSound(v); try { localStorage.setItem('webmail_notif_sound', v ? '1' : '0'); } catch { /* */ } }}
            notifDetail={notifDetail}
            setNotifDetail={(v) => { setNotifDetail(v); try { localStorage.setItem('webmail_notif_detail', v); } catch { /* */ } }}
            badgeCountMode={badgeCountMode}
            setBadgeCountMode={(mode) => {
              setBadgeCountMode(mode);
              try {
                localStorage.setItem(BADGE_COUNT_MODE_KEY, mode);
                window.dispatchEvent(new StorageEvent('storage', { key: BADGE_COUNT_MODE_KEY, newValue: mode }));
              } catch { /* */ }
            }}
            dndEnabled={dndEnabled}
            setDndEnabled={(v) => { setDndEnabled(v); try { localStorage.setItem('webmail_dnd', v ? '1' : '0'); } catch { /* */ } }}
            dndStart={dndStart}
            setDndStart={(v) => { setDndStart(v); try { localStorage.setItem('webmail_dnd_start', v); } catch { /* */ } }}
            dndEnd={dndEnd}
            setDndEnd={(v) => { setDndEnd(v); try { localStorage.setItem('webmail_dnd_end', v); } catch { /* */ } }}
            folders={notificationFolders}
            folderOverrides={notificationFolderOverrides}
            setFolderNotificationEnabled={(folderId, enabled) => {
              setNotificationFolderOverrides((prev) => {
                const next = { ...prev };
                if (enabled) {
                  delete next[folderId];
                } else {
                  next[folderId] = { enabled: false, dnd_inherit: true, dnd_schedule: emptyDNDSchedule() };
                }
                try {
                  localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(next));
                } catch { /* */ }
                return next;
              });
            }}
            webPushEnabled={webPushEnabled}
            setWebPushEnabled={(v) => {
              setWebPushEnabled(v);
              try { localStorage.setItem('webmail_webpush_enabled', v ? 'true' : 'false'); } catch { /* */ }
            }}
            webPushSupported={webPushSupported}
          />
        );

      case 'shortcuts':
        return <SettingsShortcutsSection />;

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
          <SettingsAccessibilitySection
            reducedMotion={reducedMotion}
            setReducedMotion={setReducedMotion}
            highContrast={highContrast}
            setHighContrast={setHighContrast}
            underlineLinks={underlineLinks}
            setUnderlineLinks={setUnderlineLinks}
            largerClickTargets={largerClickTargets}
            setLargerClickTargets={setLargerClickTargets}
            screenReaderMode={screenReaderMode}
            setScreenReaderMode={setScreenReaderMode}
            fontFamily={fontFamily}
            setFontFamily={setFontFamily}
            colorBlindMode={colorBlindMode}
            alwaysFocusRing={alwaysFocusRing}
            setAlwaysFocusRing={setAlwaysFocusRing}
            dyslexiaMode={dyslexiaMode}
            setDyslexiaMode={setDyslexiaMode}
            uiFontSize={uiFontSize}
            lineSpacing={lineSpacing}
            letterSpacing={letterSpacing}
            applyColorBlindMode={applyColorBlindMode}
            applyUiFontSize={applyUiFontSize}
            applyLineSpacing={applyLineSpacing}
            applyLetterSpacing={applyLetterSpacing}
          />
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
