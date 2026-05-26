'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { getFolderStats } from '@/lib/api';
import { NAV_ITEMS, type SectionId } from '@/components/settings-view/settingsViewConfig';
import { FilterRulesSection } from '@/components/settings-view/FilterRulesSection';
import { SettingsAboutSection } from '@/components/settings-view/SettingsAboutSection';
import { SettingsStorageSection } from '@/components/settings-view/SettingsStorageSection';
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
import { useSettingsPrefs } from '@/components/settings-view/useSettingsPrefs';

export interface SettingsViewProps {
  userEmail?: string;
  userName?: string;
  initialSection?: SectionId;
}

// ─── Main component ────────────────────────────────────────────────────────────

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

  const {
    // Account
    displayName, setDisplayName,
    nameSaved,
    recoveryEmail, setRecoveryEmail,
    recoverySaved,
    recoveryError,
    signature, setSignature,
    sigSaved,
    avatarUrl,
    avatarSaving,
    avatarError,
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
    fontSize,
    ccSelf, setCcSelf,
    defaultBcc, setDefaultBcc,
    confirmBeforeSend, setConfirmBeforeSend,
    spellCheck, setSpellCheck,
    templates, setTemplates,
    newTplName, setNewTplName,
    newTplSubject, setNewTplSubject,
    newTplBody, setNewTplBody,
    showNewTpl, setShowNewTpl,
    // Appearance
    theme,
    accent, setAccent,
    customAccent, setCustomAccent,
    // Notifications
    notifPerm,
    notifSyncError,
    browserNotificationsEnabled,
    notifSound,
    notifDetail,
    badgeCountMode,
    dndEnabled,
    dndStart,
    dndEnd,
    webPushEnabled,
    webPushSupported,
    notificationFolderOverrides,
    notificationFolders,
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
    colorBlindMode,
    alwaysFocusRing, setAlwaysFocusRing,
    underlineLinks, setUnderlineLinks,
    dyslexiaMode, setDyslexiaMode,
    uiFontSize,
    lineSpacing,
    letterSpacing,
    // Timezone
    timezone, setTimezone,
    // Security
    revokingAll,
    revokeAllError,
    // Storage
    folderStats, setFolderStats,
    statsLoading, setStatsLoading,
    backupStates, setBackupStates,
    // User profile
    profile,
    pwCurrent, setPwCurrent,
    pwNew, setPwNew,
    pwConfirm, setPwConfirm,
    pwError,
    pwSaving,
    pwSaved,
    // Handlers
    applyTheme,
    applyAccent,
    applyFontSize,
    applyColorBlindMode,
    applyUiFontSize,
    applyLineSpacing,
    applyLetterSpacing,
    handleAvatarUpload,
    handleAvatarRemove,
    saveDisplayName,
    saveRecoveryEmail,
    saveSignature,
    handleChangePassword,
    handleRevokeAll,
    requestNotif,
    setFolderNotificationEnabled,
    setBrowserNotificationsEnabledWithStorage,
    setNotifSoundWithStorage,
    setNotifDetailWithStorage,
    setBadgeCountModeWithStorage,
    setDndEnabledWithStorage,
    setDndStartWithStorage,
    setDndEndWithStorage,
    setWebPushEnabledWithStorage,
    startBackup,
  } = useSettingsPrefs({ userEmail, userName, activeSection, t, router });

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
            setBrowserNotificationsEnabled={setBrowserNotificationsEnabledWithStorage}
            notifSound={notifSound}
            setNotifSound={setNotifSoundWithStorage}
            notifDetail={notifDetail}
            setNotifDetail={setNotifDetailWithStorage}
            badgeCountMode={badgeCountMode}
            setBadgeCountMode={setBadgeCountModeWithStorage}
            dndEnabled={dndEnabled}
            setDndEnabled={setDndEnabledWithStorage}
            dndStart={dndStart}
            setDndStart={setDndStartWithStorage}
            dndEnd={dndEnd}
            setDndEnd={setDndEndWithStorage}
            folders={notificationFolders}
            folderOverrides={notificationFolderOverrides}
            setFolderNotificationEnabled={setFolderNotificationEnabled}
            webPushEnabled={webPushEnabled}
            setWebPushEnabled={setWebPushEnabledWithStorage}
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
