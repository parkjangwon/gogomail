'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { CheckIcon, ExclamationTriangleIcon, NoSymbolIcon, ArrowDownTrayIcon } from '@heroicons/react/24/outline';
import { revokeAllSessions, getFolderStats, exportFolderEml, exportFolderZip, getPreferences, setPreferences, getUserProfile, updateUserProfile, changePassword, type FolderStats, type WebmailPreferences, type UserProfile } from '@/lib/api';
import { ReadMark, ExternalImages, SendDelay, Theme, FontSize, ACCENT_COLORS, FilterCondition, FilterAction, FilterRule, LABEL_COLORS, migrateFilterRule, loadFilterRules, saveFilterRules } from '@/lib/settings/settingsUtils';
import { NAV_ITEMS, SHORTCUT_GROUPS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { Kbd, MiniEditor, Row, SectionCard, SectionHeader, Segment, Toggle, loadWmSettings, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsViewProps {
  userEmail?: string;
  userName?: string;
}

// ─── Main component ────────────────────────────────────────────────────────────

export function SettingsView({ userEmail, userName }: SettingsViewProps) {
  const router = useRouter();
  const [activeSection, setActiveSection] = useState<SectionId>('account');
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
  const [notifSound, setNotifSound] = useState(false);
  const [notifDetail, setNotifDetail] = useState<'sender' | 'subject' | 'preview'>('subject');
  const [dndEnabled, setDndEnabled] = useState(false);
  const [dndStart, setDndStart] = useState('22:00');
  const [dndEnd, setDndEnd] = useState('08:00');

  // Templates
  const [templates, setTemplates] = useState<{ name: string; subject: string; body: string }[]>([]);
  const [newTplName, setNewTplName] = useState('');
  const [newTplSubject, setNewTplSubject] = useState('');
  const [newTplBody, setNewTplBody] = useState('');
  const [showNewTpl, setShowNewTpl] = useState(false);

  // Filters
  const [filterRules, setFilterRules] = useState<FilterRule[]>([]);
  const [editingRule, setEditingRule] = useState<FilterRule | null>(null);
  const [newRule, setNewRule] = useState<Omit<FilterRule, 'id'>>({ name: '', enabled: true, logic: 'and', conditions: [{ field: 'from', matchType: 'contains', value: '' }], action: {} });

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
  const [vacSubject, setVacSubject] = useState('부재중입니다');
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

  // Storage / Backup
  const [folderStats, setFolderStats] = useState<FolderStats[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  type BackupState = { status: 'idle' | 'running' | 'done' | 'error'; fetched: number; total: number; error?: string };
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
        if (prefs.filter_rules) setFilterRules((prefs.filter_rules as Record<string, unknown>[]).map(migrateFilterRule));
        if (prefs.blocked_senders) setBlockedSenders(prefs.blocked_senders);
        if (prefs.vacation) {
          const v = prefs.vacation;
          if (v.enabled !== undefined) setVacEnabled(v.enabled as boolean);
          if (v.startDate !== undefined) setVacStartDate(v.startDate as string);
          if (v.endDate !== undefined) setVacEndDate(v.endDate as string);
          if (v.subject) setVacSubject(v.subject as string);
          if (v.body !== undefined) setVacBody(v.body as string);
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
          notifSound, notifDetail, dndEnabled, dndStart, dndEnd,
        },
        filter_rules: filterRules as unknown[],
        blocked_senders: blockedSenders,
        vacation: { enabled: vacEnabled, startDate: vacStartDate, endDate: vacEndDate, subject: vacSubject, body: vacBody },
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
    filterRules, blockedSenders,
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
      setTemplates(JSON.parse(localStorage.getItem('webmail_templates') ?? '[]'));
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
      setVacSubject((vac.subject as string) ?? '부재중입니다');
      setVacBody((vac.body as string) ?? '');
    } catch { /* ignore */ }
    if (typeof Notification !== 'undefined') setNotifPerm(Notification.permission);
  }, [userName]);

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
      setRecoveryError(err instanceof Error ? err.message : '백업 이메일 저장에 실패했습니다.');
    }
  }

  function saveSignature() {
    try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ }
    setSigSaved(true);
    setTimeout(() => setSigSaved(false), 2000);
  }

  async function handleChangePassword() {
    setPwError('');
    if (!pwCurrent || !pwNew || !pwConfirm) { setPwError('모든 항목을 입력하세요.'); return; }
    if (pwNew.length < 8) { setPwError('새 비밀번호는 8자 이상이어야 합니다.'); return; }
    if (pwNew !== pwConfirm) { setPwError('새 비밀번호가 일치하지 않습니다.'); return; }
    setPwSaving(true);
    try {
      await changePassword(pwCurrent, pwNew);
      setPwCurrent(''); setPwNew(''); setPwConfirm('');
      setPwSaved(true);
      setTimeout(() => setPwSaved(false), 3000);
    } catch (err) {
      setPwError(err instanceof Error ? err.message : '비밀번호 변경에 실패했습니다.');
    } finally {
      setPwSaving(false);
    }
  }

  async function handleRevokeAll() {
    if (!window.confirm('모든 기기에서 로그아웃하시겠습니까? 현재 세션도 종료됩니다.')) return;
    setRevokingAll(true);
    const ok = await revokeAllSessions();
    if (ok) {
      try { localStorage.removeItem('webmail_token'); localStorage.removeItem('webmail_email'); } catch { /* ignore */ }
      router.push('/login');
    } else {
      setRevokingAll(false);
      window.alert('세션 취소에 실패했습니다. 다시 시도해 주세요.');
    }
  }

  async function requestNotif() {
    if (typeof Notification === 'undefined') return;
    const p = await Notification.requestPermission();
    setNotifPerm(p);
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
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{displayName || userName || '(이름 없음)'}</div>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '3px' }}>{userEmail}</div>
              </div>
            </div>
            <SectionCard>
              <SectionHeader>프로필</SectionHeader>
              <Row label="표시 이름" description="메일 발송 시 발신자 이름으로 표시됩니다">
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="이름 입력" style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '170px', outline: 'none' }} />
                  <button onClick={saveDisplayName} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
                    {nameSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />저장됨</> : '저장'}
                  </button>
                </div>
              </Row>
              <Row label="이메일 주소" description="변경하려면 관리자에게 문의하세요">
                <span style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>{userEmail}</span>
              </Row>
              <Row label="백업 이메일" description="비밀번호 재설정 링크를 받을 개인 이메일 주소입니다" last>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                  <input
                    type="email"
                    value={recoveryEmail}
                    onChange={(e) => setRecoveryEmail(e.target.value)}
                    placeholder="personal@example.com"
                    style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '220px', outline: 'none' }}
                  />
                  <button onClick={saveRecoveryEmail} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
                    {recoverySaved ? <><CheckIcon style={{ width: 13, height: 13 }} />저장됨</> : '저장'}
                  </button>
                  {recoveryError && <span style={{ fontSize: '12px', color: 'var(--color-danger, #dc2626)', width: '100%', textAlign: 'right' }}>{recoveryError}</span>}
                </div>
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>서명</SectionHeader>
              <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '10px' }}>메일 작성 시 자동으로 추가됩니다.</div>
                <MiniEditor
                  value={signature}
                  onChange={(html) => { setSignature(html); }}
                  placeholder="서명을 입력하세요..."
                />
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '10px' }}>
                  <button onClick={saveSignature} style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '5px' }}>
                    {sigSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />저장됨</> : '서명 저장'}
                  </button>
                </div>
              </div>
            </SectionCard>
            {profile && (
              <SectionCard>
                <SectionHeader>용량</SectionHeader>
                <div style={{ padding: '16px 20px' }}>
                  {(() => {
                    const used = profile.quota_used;
                    const limit = profile.quota_limit;
                    const pct = limit && limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : null;
                    const fmt = (b: number) => b >= 1073741824 ? `${(b / 1073741824).toFixed(1)} GB` : b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.round(b / 1024)} KB`;
                    return (
                      <>
                        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '8px' }}>
                          <span>{fmt(used)} 사용 중{limit ? ` / ${fmt(limit)}` : ''}</span>
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
              <SectionHeader>비밀번호 변경</SectionHeader>
              <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                {(['현재 비밀번호', '새 비밀번호', '새 비밀번호 확인'] as const).map((label, i) => (
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
                {pwSaved && <div style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', padding: '6px 10px', background: 'rgba(34,197,94,0.08)', borderRadius: '5px' }}>비밀번호가 변경되었습니다. 다시 로그인해 주세요.</div>}
                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button onClick={handleChangePassword} disabled={pwSaving} style={{ padding: '7px 18px', borderRadius: '6px', border: 'none', background: pwSaving ? 'var(--color-bg-tertiary)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: pwSaving ? 'not-allowed' : 'pointer' }}>
                    {pwSaving ? '변경 중...' : '비밀번호 변경'}
                  </button>
                </div>
              </div>
            </SectionCard>
          </>
        );

      case 'inbox':
        return (
          <SectionCard>
            <SectionHeader>받은편지함 설정</SectionHeader>
            <Row label="대화 모드" description="같은 제목의 메일을 하나의 대화로 묶어 표시합니다">
              <Toggle value={convMode} onChange={(v) => { setConvMode(v); try { localStorage.setItem('webmail_conv_mode', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="컴팩트 보기" description="행 높이를 줄여 더 많은 메일을 한 화면에 표시합니다">
              <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="미리보기 텍스트" description="메일 목록에서 본문 첫 줄을 미리 표시합니다">
              <Toggle value={showPreview} onChange={(v) => { setShowPreview(v); saveWmSetting('showPreview', v); }} />
            </Row>
            <Row label="자동 새로고침" description="받은편지함을 주기적으로 자동 업데이트합니다">
              <Segment
                options={[{ value: 30 as 30, label: '30초' }, { value: 60 as 60, label: '1분' }, { value: 300 as 300, label: '5분' }]}
                value={refreshInterval}
                onChange={(v) => { setRefreshInterval(v); try { localStorage.setItem('webmail_refresh_interval', String(v)); } catch { /* */ } }}
              />
            </Row>
            <Row label="날짜별 그룹" description="메일 목록을 오늘·어제·지난 7일 등으로 묶어 표시합니다">
              <Toggle value={groupByDate} onChange={(v) => { setGroupByDate(v); try { localStorage.setItem('webmail_group_by_date', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="중요도 마커" description="자동 분류된 메일에 카테고리 칩(알림·뉴스레터 등)을 표시합니다">
              <Toggle value={importanceMarkers} onChange={(v) => { setImportanceMarkers(v); try { localStorage.setItem('webmail_importance_markers', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="집중 모드" description="별표·핀·안읽음 메일만 표시하여 중요한 메일에 집중합니다">
              <Toggle value={focusMode} onChange={(v) => { setFocusMode(v); try { localStorage.setItem('webmail_focus_mode', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="모바일 왼쪽 스와이프" description="메일 목록에서 왼쪽으로 스와이프할 때 동작">
              <Segment
                options={[{ value: 'archive' as const, label: '보관' }, { value: 'delete' as const, label: '삭제' }, { value: 'snooze' as const, label: '스누즈' }, { value: 'star' as const, label: '별표' }]}
                value={swipeLeft}
                onChange={(v) => { setSwipeLeft(v); try { localStorage.setItem('webmail_swipe_left', v); } catch { /* */ } }}
              />
            </Row>
            <Row label="모바일 오른쪽 스와이프" description="메일 목록에서 오른쪽으로 스와이프할 때 동작" last>
              <Segment
                options={[{ value: 'archive' as const, label: '보관' }, { value: 'delete' as const, label: '삭제' }, { value: 'snooze' as const, label: '스누즈' }, { value: 'star' as const, label: '별표' }]}
                value={swipeRight}
                onChange={(v) => { setSwipeRight(v); try { localStorage.setItem('webmail_swipe_right', v); } catch { /* */ } }}
              />
            </Row>
          </SectionCard>
        );

      case 'reading':
        return (
          <SectionCard>
            <SectionHeader>읽기 설정</SectionHeader>
            <Row label="읽음 처리 시점" description="메일을 열었을 때 읽음으로 표시하는 시점">
              <Segment
                options={[{ value: 'instant' as ReadMark, label: '즉시' }, { value: '2s' as ReadMark, label: '2초 후' }, { value: 'manual' as ReadMark, label: '수동' }]}
                value={readMark}
                onChange={(v) => { setReadMark(v); saveWmSetting('readMark', v); }}
              />
            </Row>
            <Row label="외부 이미지" description="외부 서버에서 불러오는 이미지의 표시 방식입니다. '차단'하면 발신자가 읽음 여부를 추적하지 못합니다">
              <Segment
                options={[{ value: 'always' as ExternalImages, label: '항상 표시' }, { value: 'ask' as ExternalImages, label: '매번 확인' }, { value: 'never' as ExternalImages, label: '차단' }]}
                value={externalImages}
                onChange={(v) => { setExternalImages(v); saveWmSetting('externalImages', v); }}
              />
            </Row>
            <Row label="인라인 이미지 미리보기" description="첨부 이미지를 메일 본문 하단에 미리 표시합니다">
              <Toggle value={inlineImagePreview} onChange={(v) => { setInlineImagePreview(v); saveWmSetting('inlineImagePreview', v); }} />
            </Row>
            <Row label="스마트 답장 제안" description="메일 내용을 분석해 자주 쓰는 답장 문구를 자동 제안합니다">
              <Toggle value={smartReplySuggestions} onChange={(v) => { setSmartReplySuggestions(v); try { localStorage.setItem('webmail_smart_reply', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="읽기 소요 시간 표시" description="메일 목록에서 예상 읽기 시간을 표시합니다">
              <Toggle value={showReadingTime} onChange={(v) => { setShowReadingTime(v); try { localStorage.setItem('webmail_reading_time', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="읽기 창 위치" description="메일 읽기 창을 오른쪽 또는 아래쪽에 배치합니다" last>
              <Segment
                options={[{ value: 'right' as const, label: '오른쪽' }, { value: 'bottom' as const, label: '아래쪽' }, { value: 'hidden' as const, label: '숨김' }]}
                value={readingPanePosition}
                onChange={(v) => { setReadingPanePosition(v); try { localStorage.setItem('webmail_reading_pane', v); } catch { /* */ } }}
              />
            </Row>
          </SectionCard>
        );

      case 'compose': {
        function saveTpl() {
          if (!newTplName.trim()) return;
          const next = [...templates.filter((t) => t.name !== newTplName.trim()), { name: newTplName.trim(), subject: newTplSubject.trim(), body: newTplBody.trim() }];
          setTemplates(next);
          try { localStorage.setItem('webmail_templates', JSON.stringify(next)); } catch { /* */ }
          setNewTplName(''); setNewTplSubject(''); setNewTplBody(''); setShowNewTpl(false);
        }
        function deleteTpl(name: string) {
          const next = templates.filter((t) => t.name !== name);
          setTemplates(next);
          try { localStorage.setItem('webmail_templates', JSON.stringify(next)); } catch { /* */ }
        }
        return (
          <>
            <SectionCard>
              <SectionHeader>작성 설정</SectionHeader>
              <Row label="전송 지연" description="전송 버튼을 누른 후 실제 발송까지 대기합니다. 시간 내에 '실행 취소'가 가능합니다">
                <Segment
                  options={[{ value: 0 as SendDelay, label: '없음' }, { value: 5 as SendDelay, label: '5초' }, { value: 10 as SendDelay, label: '10초' }, { value: 30 as SendDelay, label: '30초' }]}
                  value={sendDelay}
                  onChange={(v) => { setSendDelay(v); saveWmSetting('sendDelay', v); }}
                />
              </Row>
              <Row label="답장 시 원문 인용" description="회신/전달 시 원본 메일 내용을 포함합니다">
                <Toggle value={quoteOnReply} onChange={(v) => { setQuoteOnReply(v); saveWmSetting('quoteOnReply', v); }} />
              </Row>
              <Row label="본문 기본 글꼴 크기" description="새 메일 작성 시 기본 글꼴 크기">
                <Segment
                  options={[{ value: 'small' as FontSize, label: '소' }, { value: 'medium' as FontSize, label: '중' }, { value: 'large' as FontSize, label: '대' }]}
                  value={fontSize}
                  onChange={(v) => applyFontSize(v)}
                />
              </Row>
              <Row label="발송 전 확인" description="전송 버튼 클릭 시 수신자·제목·첨부파일을 확인하는 다이얼로그를 표시합니다">
                <Toggle value={confirmBeforeSend} onChange={(v) => { setConfirmBeforeSend(v); try { localStorage.setItem('webmail_confirm_before_send', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label="나에게 참조 (CC)" description="보내는 모든 메일에 자신을 참조로 자동 추가합니다">
                <Toggle value={ccSelf} onChange={(v) => { setCcSelf(v); try { localStorage.setItem('webmail_cc_self', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label="맞춤법 검사" description="작성 중 맞춤법 오류를 브라우저 맞춤법 검사기로 표시합니다">
                <Toggle value={spellCheck} onChange={(v) => { setSpellCheck(v); try { localStorage.setItem('webmail_spell_check', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label="기본 BCC 주소" description="모든 발송 메일에 자동으로 숨은 참조 추가 (비워두면 비활성)" last>
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
              <SectionHeader>빠른 답장 템플릿</SectionHeader>
              {templates.length === 0 && !showNewTpl && (
                <div style={{ padding: '16px 20px', fontSize: '13px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-primary)' }}>
                  저장된 템플릿이 없습니다. 자주 사용하는 답장 내용을 저장해 두세요.
                </div>
              )}
              {templates.map((t, i) => (
                <div key={t.name} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '12px 20px', borderBottom: i < templates.length - 1 || showNewTpl ? '1px solid var(--color-border-subtle)' : 'none', background: 'var(--color-bg-primary)' }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t.name}</div>
                    {t.subject && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>제목: {t.subject}</div>}
                  </div>
                  <button onClick={() => deleteTpl(t.name)} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(220,38,38,0.3)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer' }}>삭제</button>
                </div>
              ))}
              {showNewTpl && (
                <div style={{ padding: '14px 20px', background: 'var(--color-bg-secondary)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  <input value={newTplName} onChange={(e) => setNewTplName(e.target.value)} placeholder="템플릿 이름 (필수)" style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
                  <input value={newTplSubject} onChange={(e) => setNewTplSubject(e.target.value)} placeholder="기본 제목 (선택)" style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
                  <textarea value={newTplBody} onChange={(e) => setNewTplBody(e.target.value)} placeholder="본문 내용" rows={4} style={{ padding: '8px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', fontFamily: 'inherit', outline: 'none' }} />
                  <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                    <button onClick={() => setShowNewTpl(false)} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>취소</button>
                    <button onClick={saveTpl} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer' }}>저장</button>
                  </div>
                </div>
              )}
              {!showNewTpl && (
                <div style={{ padding: '10px 20px', background: 'var(--color-bg-primary)', borderTop: templates.length > 0 ? '1px solid var(--color-border-subtle)' : 'none' }}>
                  <button onClick={() => setShowNewTpl(true)} style={{ fontSize: '13px', color: 'var(--color-accent)', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 500, padding: 0 }}>+ 새 템플릿 추가</button>
                </div>
              )}
            </SectionCard>
          </>
        );
      }

      case 'filters': {
        const fieldOpts: { value: FilterCondition['field']; label: string; noValue?: boolean }[] = [
          { value: 'from', label: '보낸사람' },
          { value: 'to', label: '받는사람' },
          { value: 'cc', label: '참조(CC)' },
          { value: 'subject', label: '제목' },
          { value: 'body', label: '본문' },
          { value: 'has_attachment', label: '첨부파일 있음', noValue: true },
          { value: 'is_unread', label: '읽지 않음', noValue: true },
          { value: 'size_larger', label: '크기 초과(KB)' },
          { value: 'size_smaller', label: '크기 이하(KB)' },
        ];
        const matchOpts: { value: FilterCondition['matchType']; label: string }[] = [
          { value: 'contains', label: '포함' },
          { value: 'not_contains', label: '포함 안 함' },
          { value: 'equals', label: '일치' },
          { value: 'starts_with', label: '시작' },
          { value: 'ends_with', label: '끝' },
          { value: 'regex', label: '정규식' },
        ];
        const ist: React.CSSProperties = { border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' };
        const sst: React.CSSProperties = { ...ist, cursor: 'pointer', flexShrink: 0 };
        const emptyRule = (): Omit<FilterRule, 'id'> => ({ name: '', enabled: true, logic: 'and', conditions: [{ field: 'from', matchType: 'contains', value: '' }], action: {} });
        const doSave = (rule: Omit<FilterRule, 'id'>, id?: string) => {
          const validConds = rule.conditions.filter((c) => fieldOpts.find((f) => f.value === c.field)?.noValue || c.value.trim());
          if (validConds.length === 0) return;
          const clean = { ...rule, conditions: validConds };
          const updated = id
            ? filterRules.map((r) => r.id === id ? { ...clean, id } : r)
            : [...filterRules, { ...clean, id: Math.random().toString(36).slice(2) }];
          setFilterRules(updated);
          saveFilterRules(updated);
          setEditingRule(null);
          setNewRule(emptyRule());
        };
        const cur = editingRule ?? newRule;
        const setCur = (patch: Partial<Omit<FilterRule, 'id'>>) =>
          editingRule ? setEditingRule((r) => r ? { ...r, ...patch } : r) : setNewRule((p) => ({ ...p, ...patch }));
        const toggleEnabled = (rule: FilterRule) => {
          const updated = filterRules.map((r) => r.id === rule.id ? { ...r, enabled: !r.enabled } : r);
          setFilterRules(updated);
          saveFilterRules(updated);
        };
        const condSummary = (rule: FilterRule) =>
          rule.conditions.map((c) => {
            const fo = fieldOpts.find((f) => f.value === c.field);
            if (fo?.noValue) return fo.label;
            return `${fo?.label ?? c.field} ${matchOpts.find((m) => m.value === c.matchType)?.label ?? c.matchType} "${c.value}"`;
          }).join(rule.logic === 'and' ? ' AND ' : ' OR ');
        const actionBadges = (a: FilterAction) => {
          const parts: string[] = [];
          if (a.labelColor) parts.push('라벨');
          if (a.moveToFolder) parts.push(`→ ${a.moveToFolder}`);
          if (a.markRead) parts.push('읽음');
          if (a.markUnread) parts.push('안읽음');
          if (a.markStarred) parts.push('별표');
          if (a.markImportant) parts.push('중요');
          if (a.skipInbox) parts.push('받은편지함 건너뜀');
          if (a.deleteMsg) parts.push('삭제');
          if (a.forwardTo) parts.push(`전달 → ${a.forwardTo}`);
          return parts.join(' · ') || '작업 없음';
        };
        return (
          <>
            <SectionCard>
              <SectionHeader>메일 필터 규칙</SectionHeader>
              <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                조건에 맞는 수신 메일에 자동으로 작업을 적용합니다. 위에서부터 순서대로 평가됩니다.
              </div>
              {filterRules.length === 0 && (
                <div style={{ padding: '8px 20px 16px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>필터 규칙이 없습니다. 아래에서 새 규칙을 추가하세요.</div>
              )}
              {filterRules.map((rule, idx) => (
                <div key={rule.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '10px 20px', borderTop: idx === 0 ? 'none' : '1px solid var(--color-border-subtle)', opacity: rule.enabled ? 1 : 0.5 }}>
                  <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0, width: '16px', textAlign: 'center', marginTop: '2px' }}>{idx + 1}</span>
                  {rule.action.labelColor && <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: rule.action.labelColor, flexShrink: 0, marginTop: '4px', display: 'inline-block' }} />}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    {rule.name && <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: '2px' }}>{rule.name}</div>}
                    <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{condSummary(rule)}</div>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{actionBadges(rule.action)}{rule.stopProcessing ? ' · [중단]' : ''}</div>
                  </div>
                  <button onClick={() => toggleEnabled(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: rule.enabled ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>{rule.enabled ? '활성' : '비활성'}</button>
                  <button onClick={() => setEditingRule(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', flexShrink: 0 }}>편집</button>
                  <button onClick={() => { const next = filterRules.filter((r) => r.id !== rule.id); setFilterRules(next); saveFilterRules(next); }} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}>삭제</button>
                </div>
              ))}
            </SectionCard>

            <SectionCard>
              <SectionHeader>{editingRule ? '규칙 편집' : '새 규칙 추가'}</SectionHeader>
              <div style={{ padding: '0 20px 20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
                <input placeholder="규칙 이름 (선택)" value={cur.name} onChange={(e) => setCur({ name: e.target.value })} style={{ ...ist, width: '100%', padding: '6px 10px', fontSize: '13px' }} />
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>조건 일치:</span>
                  {(['and', 'or'] as const).map((l) => (
                    <button key={l} onClick={() => setCur({ logic: l })} style={{ padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: cur.logic === l ? 'var(--color-accent)' : 'transparent', color: cur.logic === l ? '#fff' : 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>
                      {l === 'and' ? '모두 (AND)' : '하나 이상 (OR)'}
                    </button>
                  ))}
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>조건</span>
                  {cur.conditions.map((cond, idx) => {
                    const isNoValue = fieldOpts.find((f) => f.value === cond.field)?.noValue;
                    return (
                      <div key={idx} style={{ display: 'flex', gap: '5px', alignItems: 'center' }}>
                        <select value={cond.field} onChange={(e) => { const v = e.target.value as FilterCondition['field']; const updated = cur.conditions.map((c, i) => i === idx ? { ...c, field: v, value: '' } : c); setCur({ conditions: updated }); }} style={sst}>
                          {fieldOpts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                        </select>
                        {!isNoValue && (
                          <>
                            <select value={cond.matchType} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, matchType: e.target.value as FilterCondition['matchType'] } : c); setCur({ conditions: updated }); }} style={sst}>
                              {matchOpts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                            </select>
                            <input placeholder={cond.field.startsWith('size') ? '예: 1024' : '예: @naver.com'} value={cond.value} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, value: e.target.value } : c); setCur({ conditions: updated }); }} style={{ ...ist, flex: 1 }} />
                          </>
                        )}
                        {cur.conditions.length > 1 && <button onClick={() => setCur({ conditions: cur.conditions.filter((_, i) => i !== idx) })} style={{ fontSize: '16px', lineHeight: 1, padding: '2px 5px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>×</button>}
                      </div>
                    );
                  })}
                  <button onClick={() => setCur({ conditions: [...cur.conditions, { field: 'from', matchType: 'contains', value: '' }] })} style={{ alignSelf: 'flex-start', fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer' }}>+ 조건 추가</button>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>작업</span>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.labelColor} onChange={(e) => setCur({ action: { ...cur.action, labelColor: e.target.checked ? LABEL_COLORS[0] : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>라벨 색상 적용</span>
                    {cur.action.labelColor && LABEL_COLORS.map((c) => (
                      <button key={c} onClick={(e) => { e.preventDefault(); setCur({ action: { ...cur.action, labelColor: c } }); }} style={{ width: '16px', height: '16px', borderRadius: '50%', background: c, border: cur.action.labelColor === c ? '3px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', padding: 0, flexShrink: 0 }} />
                    ))}
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={cur.action.moveToFolder !== undefined} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.checked ? '' : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>편지함으로 이동</span>
                    {cur.action.moveToFolder !== undefined && <input placeholder="편지함 이름" value={cur.action.moveToFolder} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.value } })} style={{ ...ist, flex: 1 }} />}
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.markRead} onChange={(e) => setCur({ action: { ...cur.action, markRead: e.target.checked ? true : undefined, markUnread: undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>읽음으로 표시</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.markUnread} onChange={(e) => setCur({ action: { ...cur.action, markUnread: e.target.checked ? true : undefined, markRead: undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>읽지 않음으로 표시</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.markStarred} onChange={(e) => setCur({ action: { ...cur.action, markStarred: e.target.checked ? true : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>별표 표시</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.markImportant} onChange={(e) => setCur({ action: { ...cur.action, markImportant: e.target.checked ? true : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>중요 표시</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.skipInbox} onChange={(e) => setCur({ action: { ...cur.action, skipInbox: e.target.checked ? true : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>받은편지함 건너뜀 (자동 보관)</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.action.deleteMsg} onChange={(e) => setCur({ action: { ...cur.action, deleteMsg: e.target.checked ? true : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-destructive)' }}>삭제 (휴지통으로 이동)</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={cur.action.forwardTo !== undefined} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.checked ? '' : undefined } })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>전달</span>
                    {cur.action.forwardTo !== undefined && <input type="email" placeholder="전달받을 이메일 주소" value={cur.action.forwardTo} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.value } })} style={{ ...ist, flex: 1 }} />}
                  </label>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '7px', paddingTop: '4px', borderTop: '1px solid var(--color-border-subtle)' }}>
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>규칙 옵션</span>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={cur.enabled} onChange={(e) => setCur({ enabled: e.target.checked })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>이 규칙 활성화</span>
                  </label>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                    <input type="checkbox" checked={!!cur.stopProcessing} onChange={(e) => setCur({ stopProcessing: e.target.checked ? true : undefined })} />
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>이 규칙 적용 후 이후 규칙 무시</span>
                  </label>
                </div>
                <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                  {editingRule && <button onClick={() => { setEditingRule(null); setNewRule(emptyRule()); }} style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>취소</button>}
                  <button onClick={() => doSave(cur, editingRule?.id)} style={{ padding: '6px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}>{editingRule ? '저장' : '추가'}</button>
                </div>
              </div>
            </SectionCard>
          </>
        );
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
              <SectionHeader>차단된 발신자</SectionHeader>
              <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                차단된 이메일 주소 또는 도메인(@example.com)에서 받은 메일은 자동으로 스팸으로 분류됩니다.
              </div>
              {blockedSenders.length === 0 && (
                <div style={{ padding: '8px 20px 16px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>차단된 발신자가 없습니다.</div>
              )}
              {blockedSenders.map((addr, idx) => (
                <div key={addr} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '9px 20px', borderTop: idx === 0 ? 'none' : '1px solid var(--color-border-subtle)' }}>
                  <NoSymbolIcon style={{ width: 13, height: 13, color: 'var(--color-destructive)', flexShrink: 0 }} />
                  <span style={{ flex: 1, fontSize: '13px', color: 'var(--color-text-primary)', fontFamily: 'monospace' }}>{addr}</span>
                  <button
                    onClick={() => saveBlocked(blockedSenders.filter((a) => a !== addr))}
                    style={{ fontSize: '12px', padding: '2px 10px', borderRadius: '5px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}
                  >차단 해제</button>
                </div>
              ))}
            </SectionCard>

            <SectionCard>
              <SectionHeader>발신자 차단 추가</SectionHeader>
              <div style={{ padding: '0 20px 16px', display: 'flex', gap: '8px', alignItems: 'center' }}>
                <input
                  value={newBlockedInput}
                  onChange={(e) => setNewBlockedInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') addBlocked(); }}
                  placeholder="이메일 주소 또는 도메인 (예: @spam.com)"
                  style={blockInSt}
                />
                <button
                  onClick={addBlocked}
                  disabled={!newBlockedInput.trim()}
                  style={{ padding: '7px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: newBlockedInput.trim() ? 'pointer' : 'default', opacity: newBlockedInput.trim() ? 1 : 0.45, flexShrink: 0 }}
                >차단</button>
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
              <SectionHeader>자동 응답 (부재중)</SectionHeader>
              <Row label="자동 응답 사용" description="이 기간 동안 받은 메일에 자동으로 응답 메일을 전송합니다">
                <Toggle value={vacEnabled} onChange={setVacEnabled} />
              </Row>
              <Row label="시작일" last={false}>
                <input type="date" value={vacStartDate} onChange={(e) => setVacStartDate(e.target.value)} style={{ ...inSt, width: '160px' }} disabled={!vacEnabled} />
              </Row>
              <Row label="종료일" last>
                <input type="date" value={vacEndDate} onChange={(e) => setVacEndDate(e.target.value)} style={{ ...inSt, width: '160px' }} disabled={!vacEnabled} />
              </Row>
            </SectionCard>

            <SectionCard>
              <SectionHeader>응답 메시지</SectionHeader>
              <div style={{ padding: '0 20px 16px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: '5px' }}>제목</label>
                  <input
                    value={vacSubject}
                    onChange={(e) => setVacSubject(e.target.value)}
                    disabled={!vacEnabled}
                    style={inSt}
                    placeholder="부재중입니다"
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 500, color: 'var(--color-text-secondary)', marginBottom: '5px' }}>본문</label>
                  <div style={{ opacity: vacEnabled ? 1 : 0.5, pointerEvents: vacEnabled ? 'auto' : 'none' }}>
                    <MiniEditor
                      value={vacBody}
                      onChange={(html) => { setVacBody(html); }}
                      placeholder="안녕하세요, 현재 부재중으로 메일 확인이 어렵습니다..."
                    />
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  {vacEnabled && vacStartDate && vacEndDate && (
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                      {vacStartDate} ~ {vacEndDate} 동안 자동 응답이 전송됩니다
                    </span>
                  )}
                  <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '10px' }}>
                    {vacSaved && (
                      <span style={{ fontSize: '12px', color: 'var(--color-accent)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                        <CheckIcon style={{ width: 13, height: 13 }} /> 저장됨
                      </span>
                    )}
                    <button
                      onClick={saveVac}
                      style={{ padding: '6px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                    >저장</button>
                  </div>
                </div>
              </div>
            </SectionCard>
          </>
        );
      }

      case 'privacy':
        return (
          <>
            <SectionCard>
              <SectionHeader>추적 방지</SectionHeader>
              <Row label="추적 픽셀 차단" description="메일에 삽입된 1×1 추적 이미지를 자동으로 차단합니다. 발신자가 읽음 여부를 알 수 없습니다.">
                <Toggle value={blockTrackingPixels} onChange={(v) => { setBlockTrackingPixels(v); saveWmSetting('blockTrackingPixels', v); }} />
              </Row>
              <Row label="링크 미리보기" description="링크 위에 마우스를 올렸을 때 미리보기를 표시합니다." last>
                <Toggle value={linkPreview} onChange={(v) => { setLinkPreview(v); saveWmSetting('linkPreview', v); }} />
              </Row>
            </SectionCard>

            <SectionCard>
              <SectionHeader>발신 메일 설정</SectionHeader>
              <Row label="읽음 확인 요청" description="보내는 메일에 읽음 확인 요청을 자동으로 포함합니다.">
                <Toggle value={requestReadReceipt} onChange={(v) => { setRequestReadReceipt(v); saveWmSetting('requestReadReceipt', v); }} />
              </Row>
              <Row label="답장 미수신 시 알림" description="보낸 메일에 답장이 없을 경우 지정한 기간 후 알림을 받습니다." last>
                <Segment<0 | 1 | 3 | 7>
                  options={[{ value: 0, label: '없음' }, { value: 1, label: '1일' }, { value: 3, label: '3일' }, { value: 7, label: '1주일' }]}
                  value={followUpDays}
                  onChange={(v) => { setFollowUpDays(v); saveWmSetting('followUpDays', v); }}
                />
              </Row>
            </SectionCard>

            <SectionCard>
              <SectionHeader>데이터 및 개인정보</SectionHeader>
              <Row label="GoGoMail 텔레메트리" description="GoGoMail은 사용자 데이터를 수집하거나 외부 서버로 전송하지 않습니다." last>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '12px', color: '#16a34a', fontWeight: 600 }}>
                  <CheckIcon style={{ width: 14, height: 14 }} />
                  완전 로컬 처리
                </span>
              </Row>
            </SectionCard>
          </>
        );

      case 'appearance':
        return (
          <>
            <SectionCard>
              <SectionHeader>테마</SectionHeader>
              <Row label="테마 모드" description="라이트, 다크, 또는 시스템 설정에 따라 자동 전환" last>
                <Segment
                  options={[{ value: 'light' as Theme, label: '라이트' }, { value: 'dark' as Theme, label: '다크' }, { value: 'system' as Theme, label: '시스템' }]}
                  value={theme}
                  onChange={applyTheme}
                />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>강조 색상</SectionHeader>
              <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
                <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '14px' }}>버튼, 링크, 선택 영역에 사용되는 색상입니다.</div>
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
                    <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>직접 입력</span>
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
              <SectionHeader>밀도 및 폰트</SectionHeader>
              <Row label="컴팩트 보기" description="메일 목록 행 높이를 줄여 더 많은 메일을 표시합니다">
                <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
              <Row label="본문 글꼴 크기" description="메일 목록 및 UI 전반의 기본 글꼴 크기" last>
                <Segment
                  options={[{ value: 'small' as FontSize, label: '소 (13px)' }, { value: 'medium' as FontSize, label: '중 (14px)' }, { value: 'large' as FontSize, label: '대 (15px)' }]}
                  value={fontSize}
                  onChange={applyFontSize}
                />
              </Row>
            </SectionCard>
          </>
        );

      case 'notifications':
        return (
          <SectionCard>
            <SectionHeader>알림 설정</SectionHeader>
            <Row label="브라우저 알림" description={notifPerm === 'granted' ? '새 메일 알림이 허용되어 있습니다' : notifPerm === 'denied' ? '알림이 차단됨 — 브라우저 설정에서 변경하세요' : '새 메일 도착 시 데스크탑 알림을 보냅니다'}>
              {notifPerm === 'granted'
                ? <span style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}><CheckIcon style={{ width: 14, height: 14 }} />허용됨</span>
                : notifPerm === 'denied'
                ? <span style={{ fontSize: '12px', color: 'var(--color-destructive)', fontWeight: 500 }}>차단됨</span>
                : <button onClick={requestNotif} style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}>허용하기</button>
              }
            </Row>
            <Row label="알림 소리" description="새 메일 도착 시 알림음을 재생합니다">
              <Toggle value={notifSound} onChange={(v) => { setNotifSound(v); try { localStorage.setItem('webmail_notif_sound', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            <Row label="알림 표시 수준" description="알림 팝업에 표시할 정보 수준을 선택합니다">
              <Segment
                options={[{ value: 'sender' as const, label: '발신자' }, { value: 'subject' as const, label: '제목' }, { value: 'preview' as const, label: '미리보기' }]}
                value={notifDetail}
                onChange={(v) => { setNotifDetail(v); try { localStorage.setItem('webmail_notif_detail', v); } catch { /* */ } }}
              />
            </Row>
            <Row label="방해 금지 모드" description="지정한 시간대에 알림을 무음으로 처리합니다">
              <Toggle value={dndEnabled} onChange={(v) => { setDndEnabled(v); try { localStorage.setItem('webmail_dnd', v ? '1' : '0'); } catch { /* */ } }} />
            </Row>
            {dndEnabled && (
              <Row label="방해 금지 시간대" description="알림을 억제할 시작·종료 시간" last>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <input type="time" value={dndStart} onChange={(e) => { setDndStart(e.target.value); try { localStorage.setItem('webmail_dnd_start', e.target.value); } catch { /* */ } }}
                    style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>~</span>
                  <input type="time" value={dndEnd} onChange={(e) => { setDndEnd(e.target.value); try { localStorage.setItem('webmail_dnd_end', e.target.value); } catch { /* */ } }}
                    style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
                </div>
              </Row>
            )}
            {!dndEnabled && <div style={{ height: '1px' }} />}
          </SectionCard>
        );

      case 'shortcuts':
        return (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            {SHORTCUT_GROUPS.map((group) => (
              <SectionCard key={group.title}>
                <SectionHeader>{group.title}</SectionHeader>
                <div style={{ background: 'var(--color-bg-primary)' }}>
                  {group.items.map(([key, desc], i) => (
                    <div key={key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '9px 16px', borderBottom: i < group.items.length - 1 ? '1px solid var(--color-border-subtle)' : 'none' }}>
                      <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1 }}>{desc}</span>
                      <Kbd k={key} />
                    </div>
                  ))}
                </div>
              </SectionCard>
            ))}
          </div>
        );

      case 'security': {
        const apiToken = (() => { try { return btoa(`${userEmail ?? 'user'}:${Date.now().toString(36)}`).slice(0, 32); } catch { return 'token-unavailable'; } })();
        const loginHistory = [
          { device: '현재 기기', location: '서울, 대한민국', time: '지금', current: true },
          { device: 'Chrome on macOS', location: '서울, 대한민국', time: '2일 전', current: false },
          { device: 'Safari on iPhone', location: '부산, 대한민국', time: '5일 전', current: false },
        ];
        return (
          <>
            <SectionCard>
              <SectionHeader>세션 관리</SectionHeader>
              {loginHistory.map((session, i) => (
                <Row key={session.device} label={session.device} description={`${session.location} · ${session.time}`} last={i === loginHistory.length - 1}>
                  {session.current
                    ? <span style={{ fontSize: '11px', color: 'var(--color-success, #22c55e)', fontWeight: 600, background: 'rgba(34,197,94,0.1)', padding: '2px 8px', borderRadius: '10px' }}>현재</span>
                    : <button style={{ fontSize: '12px', color: 'var(--color-destructive)', background: 'transparent', border: '1px solid rgba(220,38,38,0.3)', borderRadius: '5px', padding: '3px 10px', cursor: 'pointer' }}>종료</button>
                  }
                </Row>
              ))}
            </SectionCard>
            <SectionCard>
              <SectionHeader>API 액세스 토큰</SectionHeader>
              <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                외부 앱이나 스크립트에서 GoGoMail API에 접근할 때 사용합니다.
              </div>
              <Row label="액세스 토큰" description="Bearer 토큰으로 API 요청에 포함하세요" last>
                <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
                  <code style={{ fontSize: '11px', fontFamily: 'monospace', background: 'var(--color-bg-tertiary)', padding: '4px 8px', borderRadius: '4px', color: 'var(--color-text-secondary)', maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{apiToken}…</code>
                  <button onClick={() => { try { navigator.clipboard.writeText(apiToken); } catch { /* */ } }}
                    style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>복사</button>
                </div>
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>위험 구역</SectionHeader>
              <Row label="2단계 인증 (2FA)" description="TOTP 앱을 사용한 추가 인증 레이어 (엔터프라이즈 기능)">
                <button style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-accent)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer', fontWeight: 600 }}>설정하기</button>
              </Row>
              <Row label="모든 기기에서 로그아웃" description="현재 기기를 포함한 모든 활성 세션을 즉시 종료합니다" last>
                <button
                  onClick={handleRevokeAll}
                  disabled={revokingAll}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 14px', borderRadius: '6px', border: '1px solid rgba(220,38,38,0.35)', background: 'rgba(220,38,38,0.04)', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 600, cursor: revokingAll ? 'wait' : 'pointer' }}
                >
                  <ExclamationTriangleIcon style={{ width: 13, height: 13 }} />
                  {revokingAll ? '처리 중...' : '전체 로그아웃'}
                </button>
              </Row>
            </SectionCard>
          </>
        );
      }

      case 'accessibility':
        return (
          <>
            <SectionCard>
              <SectionHeader>시각 보조</SectionHeader>
              <Row label="고대비 모드" description="텍스트와 배경 사이의 대비를 높여 가독성을 향상시킵니다">
                <Toggle value={highContrast} onChange={(v) => { setHighContrast(v); try { localStorage.setItem('webmail_high_contrast', v ? '1' : '0'); if (v) document.documentElement.classList.add('high-contrast'); else document.documentElement.classList.remove('high-contrast'); } catch { /* */ } }} />
              </Row>
              <Row label="움직임 줄이기" description="전환 애니메이션과 모션 효과를 최소화합니다">
                <Toggle value={reducedMotion} onChange={(v) => { setReducedMotion(v); try { localStorage.setItem('webmail_reduced_motion', v ? '1' : '0'); document.documentElement.style.setProperty('--motion-duration', v ? '0ms' : ''); } catch { /* */ } }} />
              </Row>
              <Row label="글꼴 종류" description="UI 전반에 사용할 글꼴 패밀리">
                <Segment
                  options={[{ value: 'system' as const, label: '시스템' }, { value: 'serif' as const, label: '명조' }, { value: 'mono' as const, label: '고정폭' }]}
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
              <Row label="클릭 영역 확장" description="버튼과 링크의 클릭 영역을 넓혀 조작을 쉽게 합니다" last>
                <Toggle value={largerClickTargets} onChange={(v) => { setLargerClickTargets(v); try { localStorage.setItem('webmail_larger_targets', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
            </SectionCard>
            <SectionCard>
              <SectionHeader>스크린 리더 지원</SectionHeader>
              <Row label="스크린 리더 최적화 모드" description="ARIA 레이블과 라이브 영역을 강화해 보조 기술과의 호환성을 높입니다" last>
                <Toggle value={screenReaderMode} onChange={(v) => { setScreenReaderMode(v); try { localStorage.setItem('webmail_screen_reader', v ? '1' : '0'); } catch { /* */ } }} />
              </Row>
            </SectionCard>
          </>
        );

      case 'about':
        return (
          <>
            <SectionCard>
              <SectionHeader>정보</SectionHeader>
              <Row label="GoGoMail Webmail" description="오픈소스 엔터프라이즈 메일 클라이언트" last>
                <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>Next.js 15 · TS · Tailwind v4</span>
              </Row>
            </SectionCard>

            <SectionCard>
              <SectionHeader>설정 내보내기 / 가져오기</SectionHeader>
              <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
                설정을 JSON 파일로 저장하거나 다른 기기에서 가져올 수 있습니다. 필터, 차단 목록, 템플릿 등 모든 설정이 포함됩니다.
              </div>
              <Row label="설정 내보내기" description="현재 모든 설정을 JSON 파일로 저장합니다">
                <button
                  onClick={() => {
                    const keys = ['webmail_settings', 'webmail_filter_rules', 'webmail_blocked_senders', 'webmail_vacation', 'webmail_templates', 'webmail_theme', 'webmail_accent', 'webmail_compact', 'webmail_conv_mode', 'webmail_display_name', 'webmail_signature', 'webmail_notif_sound', 'webmail_notif_detail', 'webmail_notif_detail', 'webmail_dnd', 'webmail_dnd_start', 'webmail_dnd_end', 'webmail_focus_mode', 'webmail_importance_markers', 'webmail_swipe_left', 'webmail_swipe_right', 'webmail_cc_self', 'webmail_default_bcc', 'webmail_confirm_before_send', 'webmail_spell_check', 'webmail_smart_reply', 'webmail_reading_time', 'webmail_reading_pane', 'webmail_pinned', 'webmail_important', 'webmail_snoozed', 'webmail_labels', 'webmail_tasks', 'webmail_notes', 'webmail_recent_recipients'];
                    const data: Record<string, unknown> = { _version: 1, _exportedAt: new Date().toISOString() };
                    keys.forEach((k) => { try { const v = localStorage.getItem(k); if (v !== null) data[k] = JSON.parse(v); } catch { /* ignore */ } });
                    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a'); a.href = url; a.download = 'gogomail-settings.json'; a.click();
                    URL.revokeObjectURL(url);
                  }}
                  style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 }}
                >내보내기</button>
              </Row>
              <Row label="설정 가져오기" description="gogomail-settings.json 파일에서 설정을 불러옵니다. 현재 설정이 대체됩니다." last>
                <label style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500, display: 'inline-block' }}>
                  가져오기
                  <input
                    type="file" accept=".json" style={{ display: 'none' }}
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (!file) return;
                      const reader = new FileReader();
                      reader.onload = (ev) => {
                        try {
                          const data = JSON.parse(ev.target?.result as string) as Record<string, unknown>;
                          Object.entries(data).forEach(([k, v]) => {
                            if (k.startsWith('webmail_')) localStorage.setItem(k, JSON.stringify(v));
                          });
                          window.location.reload();
                        } catch { window.alert('올바르지 않은 설정 파일입니다.'); }
                      };
                      reader.readAsText(file);
                    }}
                  />
                </label>
              </Row>
            </SectionCard>
          </>
        );

      case 'storage': {
        const QUOTA_BYTES = 10 * 1024 * 1024 * 1024; // 10 GB display quota
        const totalUsed = folderStats.reduce((s, f) => s + f.size_bytes, 0);
        const usedPct = Math.min((totalUsed / QUOTA_BYTES) * 100, 100);
        const fmt = (bytes: number) => {
          if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
          if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
          if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
          return `${bytes} B`;
        };
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
        const barColor = usedPct > 85 ? '#ef4444' : usedPct > 60 ? '#f97316' : 'var(--color-accent)';
        return (
          <>
            {/* Overall usage */}
            <SectionCard>
              <SectionHeader>사용자 용량 현황</SectionHeader>
              <div style={{ padding: '0 20px 20px' }}>
                {statsLoading && <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '12px 0' }}>불러오는 중...</div>}
                {!statsLoading && folderStats.length === 0 && (
                  <button onClick={() => { setStatsLoading(true); getFolderStats().then(setFolderStats).catch(() => {}).finally(() => setStatsLoading(false)); }} style={{ marginTop: '8px', padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', cursor: 'pointer', fontWeight: 600 }}>
                    용량 정보 불러오기
                  </button>
                )}
                {folderStats.length > 0 && (
                  <>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: '6px' }}>
                      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{fmt(totalUsed)} 사용 중</span>
                      <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>총 {fmt(QUOTA_BYTES)} 중</span>
                    </div>
                    <div style={{ height: '8px', borderRadius: '4px', background: 'var(--color-bg-tertiary)', overflow: 'hidden', marginBottom: '4px' }}>
                      <div style={{ height: '100%', width: `${usedPct}%`, background: barColor, borderRadius: '4px', transition: 'width 0.5s ease' }} />
                    </div>
                    <div style={{ fontSize: '11px', color: usedPct > 85 ? '#ef4444' : 'var(--color-text-tertiary)', textAlign: 'right' }}>{usedPct.toFixed(1)}% 사용</div>
                  </>
                )}
              </div>
            </SectionCard>

            {/* Per-folder table */}
            {folderStats.length > 0 && (
              <SectionCard>
                <SectionHeader>편지함별 현황</SectionHeader>
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                        {['편지함', '메일 수', '안읽음', '별표', '용량', 'EML 백업', 'ZIP 백업'].map((h) => (
                          <th key={h} style={{ padding: '8px 16px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em', whiteSpace: 'nowrap' }}>{h}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {folderStats.map((f) => {
                        const emlKey = `${f.id}-eml`;
                        const zipKey = `${f.id}-zip`;
                        const emlState = backupStates[emlKey] ?? { status: 'idle' };
                        const zipState = backupStates[zipKey] ?? { status: 'idle' };
                        const BtnLabel = ({ state, format }: { state: BackupState; format: 'EML' | 'ZIP' }) => {
                          if (state.status === 'running') return <>{state.total > 0 ? `${state.fetched}/${state.total}` : '...'}</>;
                          if (state.status === 'done') return <>완료 ✓</>;
                          if (state.status === 'error') return <>오류</>;
                          return <>{format}</>;
                        };
                        return (
                          <tr key={f.id} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                            <td style={{ padding: '10px 16px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{f.name}</td>
                            <td style={{ padding: '10px 16px', color: 'var(--color-text-secondary)', textAlign: 'right' }}>{f.total.toLocaleString()}</td>
                            <td style={{ padding: '10px 16px', color: f.unread > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)', textAlign: 'right', fontWeight: f.unread > 0 ? 600 : 400 }}>{f.unread.toLocaleString()}</td>
                            <td style={{ padding: '10px 16px', color: 'var(--color-text-tertiary)', textAlign: 'right' }}>{f.starred.toLocaleString()}</td>
                            <td style={{ padding: '10px 16px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>{fmt(f.size_bytes)}</td>
                            <td style={{ padding: '10px 16px' }}>
                              <button
                                onClick={() => startBackup(f.id, f.name, 'eml')}
                                disabled={emlState.status === 'running'}
                                style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: emlState.status === 'done' ? 'var(--color-success-subtle, #dcfce7)' : 'transparent', color: emlState.status === 'error' ? 'var(--color-destructive)' : 'var(--color-text-secondary)', fontSize: '11px', cursor: emlState.status === 'running' ? 'default' : 'pointer', display: 'inline-flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}
                              >
                                <ArrowDownTrayIcon style={{ width: 12, height: 12 }} />
                                <BtnLabel state={emlState} format="EML" />
                              </button>
                            </td>
                            <td style={{ padding: '10px 16px' }}>
                              <button
                                onClick={() => startBackup(f.id, f.name, 'zip')}
                                disabled={zipState.status === 'running'}
                                style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: zipState.status === 'done' ? 'var(--color-success-subtle, #dcfce7)' : 'transparent', color: zipState.status === 'error' ? 'var(--color-destructive)' : 'var(--color-text-secondary)', fontSize: '11px', cursor: zipState.status === 'running' ? 'default' : 'pointer', display: 'inline-flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}
                              >
                                <ArrowDownTrayIcon style={{ width: 12, height: 12 }} />
                                <BtnLabel state={zipState} format="ZIP" />
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                    <tfoot>
                      <tr style={{ borderTop: '2px solid var(--color-border-default)' }}>
                        <td style={{ padding: '8px 16px', fontWeight: 600, fontSize: '12px', color: 'var(--color-text-primary)' }}>합계</td>
                        <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600 }}>{folderStats.reduce((s, f) => s + f.total, 0).toLocaleString()}</td>
                        <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600, color: 'var(--color-accent)' }}>{folderStats.reduce((s, f) => s + f.unread, 0).toLocaleString()}</td>
                        <td style={{ padding: '8px 16px', textAlign: 'right', fontWeight: 600 }}>{folderStats.reduce((s, f) => s + f.starred, 0).toLocaleString()}</td>
                        <td style={{ padding: '8px 16px', fontWeight: 600 }}>{fmt(totalUsed)}</td>
                        <td colSpan={2} />
                      </tr>
                    </tfoot>
                  </table>
                </div>
              </SectionCard>
            )}
          </>
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
        <div style={{ padding: '0 12px 16px', fontSize: '11px', fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>설정</div>
        {NAV_ITEMS.map((item) => {
          const active = item.id === activeSection;
          return (
            <button
              key={item.id}
              onClick={() => { setActiveSection(item.id); contentRef.current?.scrollTo({ top: 0 }); }}
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
              {item.label}
            </button>
          );
        })}
      </div>

      {/* Content area */}
      <div ref={contentRef} style={{ flex: 1, minWidth: 0, height: '100%', overflowY: 'auto', padding: '32px 40px' }}>
        <h2 style={{ fontSize: '17px', fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: '24px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ color: 'var(--color-text-tertiary)', display: 'flex' }}>{currentNav?.icon}</span>
          {currentNav?.label}
        </h2>
        {renderContent()}
      </div>
    </div>
  );
}
