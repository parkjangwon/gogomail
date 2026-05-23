'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { CheckIcon, ExclamationTriangleIcon, NoSymbolIcon, ArrowDownTrayIcon } from '@heroicons/react/24/outline';
import { revokeAllSessions, getFolderStats, exportFolderEml, exportFolderZip, getPreferences, setPreferences, getUserProfile, updateUserProfile, changePassword, registerWebPushDevice, getNotificationPreferences, setNotificationPreferences, type FolderStats, type WebmailPreferences, type UserProfile, type NotificationPreferences } from '@/lib/api';
import { ReadMark, ExternalImages, SendDelay, Theme, FontSize, ACCENT_COLORS, FilterRule, migrateFilterRule, loadFilterRules, saveFilterRules } from '@/lib/settings/settingsUtils';
import { NAV_ITEMS, SHORTCUT_GROUPS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { Kbd, MiniEditor, Row, SectionCard, SectionHeader, Segment, Toggle, loadWmSettings, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';
import { FilterRulesSection } from '@/components/settings-view/FilterRulesSection';
import { SettingsAboutSection } from '@/components/settings-view/SettingsAboutSection';
import { SettingsStorageSection, type BackupState } from '@/components/settings-view/SettingsStorageSection';
import { SettingsPrivacySection } from '@/components/settings-view/SettingsPrivacySection';
import { SettingsNotificationsSection } from '@/components/settings-view/SettingsNotificationsSection';
import { SettingsSecuritySection } from '@/components/settings-view/SettingsSecuritySection';
import { handleVerticalNavKeyDown } from '@/lib/navKeyboard';
import { webPushPublicKeyToUint8Array } from '@/lib/webpush';
import { loadLocalEmailTemplates, normalizeEmailTemplates, saveLocalEmailTemplates, type StoredEmailTemplate } from '@/lib/emailTemplates';
import { stableId } from '@/lib/stableId';

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
    folder_overrides: base?.folder_overrides ?? {},
  };
}

export function SettingsView({ userEmail, userName, initialSection }: SettingsViewProps) {
  const router = useRouter();
  const t = useTranslations('settingsView');
  const [activeSection, setActiveSection] = useState<SectionId>(initialSection ?? 'account');
  const contentRef = useRef<HTMLDivElement>(null);

  // Account
  const [displayName, setDisplayName] = useState('');
  const [nameSaved, setNameSaved] = useState(false);
  const [recoveryEmail, setRecoveryEmail] = useState('');
  const [recoverySaved, setRecoverySaved] = useState(false);
  const [recoveryError, setRecoveryError] = useState('');
  const [signature, setSignature] = useState('');
  const [sigSaved, setSigSaved] = useState(false);

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
  const [notifSound, setNotifSound] = useState(false);
  const [notifDetail, setNotifDetail] = useState<'sender' | 'subject' | 'preview'>('subject');
  const [dndEnabled, setDndEnabled] = useState(false);
  const [dndStart, setDndStart] = useState('22:00');
  const [dndEnd, setDndEnd] = useState('08:00');
  const [notificationPrefsLoaded, setNotificationPrefsLoaded] = useState(false);
  const notificationPrefsBaseRef = useRef<NotificationPreferences | null>(null);
  const skipNotificationPrefsInitialSaveRef = useRef(true);

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

  // Blocked senders
  const [blockedSenders, setBlockedSenders] = useState<string[]>([]);
  const [newBlockedInput, setNewBlockedInput] = useState('');

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
          if (s.notifSound !== undefined) setNotifSound(s.notifSound as boolean);
          if (s.notifDetail) setNotifDetail(s.notifDetail as typeof notifDetail);
          if (s.dndEnabled !== undefined) setDndEnabled(s.dndEnabled as boolean);
          if (s.dndStart) setDndStart(s.dndStart as string);
          if (s.dndEnd) setDndEnd(s.dndEnd as string);
        }
        if (prefs.filter_rules) {
          const serverRules = (prefs.filter_rules as Record<string, unknown>[]).map(migrateFilterRule);
          setFilterRules(serverRules);
          saveFilterRules(serverRules);
        }
        if (prefs.blocked_senders) setBlockedSenders(prefs.blocked_senders);
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
    let cancelled = false;
    getNotificationPreferences()
      .then((prefs) => {
        if (cancelled) return;
        notificationPrefsBaseRef.current = prefs;
        setDndEnabled(prefs.global_dnd_enabled);
        const firstRange = prefs.global_dnd_schedule?.time_ranges?.[0];
        if (firstRange?.start) setDndStart(firstRange.start);
        if (firstRange?.end) setDndEnd(firstRange.end);
        try {
          localStorage.setItem('webmail_dnd', prefs.global_dnd_enabled ? '1' : '0');
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
      const next = quietHoursPreferences(notificationPrefsBaseRef.current, dndEnabled, dndStart, dndEnd);
      setNotificationPreferences(next)
        .then((saved) => { notificationPrefsBaseRef.current = saved; })
        .catch(() => {});
    }, 800);
    return () => clearTimeout(timer);
  }, [notificationPrefsLoaded, dndEnabled, dndStart, dndEnd]);

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
          notifSound, notifDetail, dndEnabled, dndStart, dndEnd,
        },
        filter_rules: filterRules as unknown[],
        blocked_senders: blockedSenders,
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
    notifSound, notifDetail, dndEnabled, dndStart, dndEnd,
    filterRules, blockedSenders, templates,
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
      setNotifDetail((localStorage.getItem('webmail_notif_detail') as 'sender' | 'subject' | 'preview') ?? 'subject');
      setTemplates(loadLocalEmailTemplates());
      setFilterRules(loadFilterRules());
      setBlockedSenders(JSON.parse(localStorage.getItem('webmail_blocked_senders') ?? '[]') as string[]);
      const priv = loadWmSettings();
      setBlockTrackingPixels((priv.blockTrackingPixels as boolean) !== false);
      setRequestReadReceipt((priv.requestReadReceipt as boolean) === true);
      setLinkPreview((priv.linkPreview as boolean) !== false);
      setFollowUpDays(((priv.followUpDays as number) ?? 0) as 0 | 1 | 3 | 7);
      const vac = JSON.parse(localStorage.getItem('webmail_vacation') ?? '{}') as Record<string, unknown>;
      setVacEnabled(vac.enabled === true);
      setVacStartDate((vac.startDate as string) ?? '');
      setVacEndDate((vac.endDate as string) ?? '');
      setVacSubject((vac.subject as string) ?? t('vacSubjectDefault'));
      setVacBody((vac.body as string) ?? '');
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
              <div style={{ width: '52px', height: '52px', borderRadius: '50%', background: 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px', fontWeight: 700, flexShrink: 0 }}>
                {(displayName || userEmail || '?')[0].toUpperCase()}
              </div>
              <div>
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{displayName || userName || t('nameEmpty')}</div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '3px' }}>{userEmail}</div>
              </div>
            </div>
            <SectionCard>
              <SectionHeader>{t('sectionProfile')}</SectionHeader>
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
              <Row label={t('recoveryEmail')} description={t('recoveryEmailDesc')} last>
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
                onChange={(v) => { setRefreshInterval(v); try { localStorage.setItem('webmail_refresh_interval', String(v)); } catch { /* */ } }}
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
        const blockInSt: React.CSSProperties = {
          border: '1px solid var(--color-border-default)', borderRadius: '6px',
          padding: '7px 10px', fontSize: '13px', background: 'var(--color-bg-primary)',
          color: 'var(--color-text-primary)', outline: 'none', flex: 1,
        };
        function saveBlocked(next: string[]) {
          try { localStorage.setItem('webmail_blocked_senders', JSON.stringify(next)); } catch { /* ignore */ }
          setBlockedSenders(next);
        }
        function addBlocked() {
          const val = newBlockedInput.trim().toLowerCase();
          if (!val || blockedSenders.includes(val)) return;
          saveBlocked([...blockedSenders, val]);
          setNewBlockedInput('');
        }
        return (
          <>
            <SectionCard>
              <SectionHeader>{t('sectionBlockedSenders')}</SectionHeader>
              <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                {t('blockedSendersDesc')}
              </div>
              {blockedSenders.length === 0 && (
                <div style={{ padding: '8px 20px 16px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{t('noBlocked')}</div>
              )}
              {blockedSenders.map((addr, idx) => (
                <div key={addr} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '9px 20px', borderTop: idx === 0 ? 'none' : '1px solid var(--color-border-subtle)' }}>
                  <NoSymbolIcon style={{ width: 13, height: 13, color: 'var(--color-destructive)', flexShrink: 0 }} />
                  <span style={{ flex: 1, fontSize: '13px', color: 'var(--color-text-primary)', fontFamily: 'monospace' }}>{addr}</span>
                  <button
                    onClick={() => saveBlocked(blockedSenders.filter((a) => a !== addr))}
                    style={{ fontSize: '12px', padding: '2px 10px', borderRadius: '5px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}
                  >{t('unblock')}</button>
                </div>
              ))}
            </SectionCard>

            <SectionCard>
              <SectionHeader>{t('sectionAddBlockedSender')}</SectionHeader>
              <div style={{ padding: '0 20px 16px', display: 'flex', gap: '8px', alignItems: 'center' }}>
                <input
                  value={newBlockedInput}
                  onChange={(e) => setNewBlockedInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') addBlocked(); }}
                  placeholder={t('blockedInputPlaceholder')}
                  style={blockInSt}
                />
                <button
                  onClick={addBlocked}
                  disabled={!newBlockedInput.trim()}
                  style={{ padding: '7px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: newBlockedInput.trim() ? 'pointer' : 'default', opacity: newBlockedInput.trim() ? 1 : 0.45, flexShrink: 0 }}
                >{t('block')}</button>
              </div>
            </SectionCard>
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
        return (
          <SettingsNotificationsSection
            notifPerm={notifPerm}
            notifSyncError={notifSyncError}
            onRequestNotif={requestNotif}
            notifSound={notifSound}
            setNotifSound={(v) => { setNotifSound(v); try { localStorage.setItem('webmail_notif_sound', v ? '1' : '0'); } catch { /* */ } }}
            notifDetail={notifDetail}
            setNotifDetail={(v) => { setNotifDetail(v); try { localStorage.setItem('webmail_notif_detail', v); } catch { /* */ } }}
            dndEnabled={dndEnabled}
            setDndEnabled={(v) => { setDndEnabled(v); try { localStorage.setItem('webmail_dnd', v ? '1' : '0'); } catch { /* */ } }}
            dndStart={dndStart}
            setDndStart={(v) => { setDndStart(v); try { localStorage.setItem('webmail_dnd_start', v); } catch { /* */ } }}
            dndEnd={dndEnd}
            setDndEnd={(v) => { setDndEnd(v); try { localStorage.setItem('webmail_dnd_end', v); } catch { /* */ } }}
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

      case 'accessibility':
        return (
          <>
            <SectionCard>
              <SectionHeader>{t('sectionVisualAids')}</SectionHeader>
              <Row label={t('highContrast')} description={t('highContrastDesc')}>
                <Toggle value={highContrast} onChange={(v) => { setHighContrast(v); try { localStorage.setItem('webmail_high_contrast', v ? '1' : '0'); if (v) document.documentElement.classList.add('high-contrast'); else document.documentElement.classList.remove('high-contrast'); } catch { /* */ } }} />
              </Row>
              <Row label={t('reducedMotion')} description={t('reducedMotionDesc')}>
                <Toggle value={reducedMotion} onChange={(v) => { setReducedMotion(v); try { localStorage.setItem('webmail_reduced_motion', v ? '1' : '0'); document.documentElement.style.setProperty('--motion-duration', v ? '0ms' : ''); } catch { /* */ } }} />
              </Row>
              <Row label={t('fontFamily')} description={t('fontFamilyDesc')}>
                <Segment
                  options={[{ value: 'system' as const, label: t('fontFamilySystem') }, { value: 'serif' as const, label: t('fontFamilySerif') }, { value: 'mono' as const, label: t('fontFamilyMono') }]}
                  value={fontFamily}
                  onChange={(v) => {
                    setFontFamily(v);
                    try {
                      localStorage.setItem('webmail_font_family', v);
                      const map = { system: 'system-ui, sans-serif', serif: 'Georgia, serif', mono: '"JetBrains Mono", "Fira Code", monospace' };
                      document.documentElement.style.setProperty('font-family', map[v]);
                    } catch { /* */ }
                  }}
                />
              </Row>
              <Row label={t('largerClickTargets')} description={t('largerClickTargetsDesc')} last>
                <Toggle value={largerClickTargets} onChange={(v) => { setLargerClickTargets(v); try { localStorage.setItem('webmail_larger_targets', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>{t('sectionScreenReader')}</SectionHeader>
              <Row label={t('screenReaderMode')} description={t('screenReaderModeDesc')} last>
                <Toggle value={screenReaderMode} onChange={(v) => { setScreenReaderMode(v); try { localStorage.setItem('webmail_screen_reader', v ? '1' : '0'); } catch { /* */ } }} />
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
