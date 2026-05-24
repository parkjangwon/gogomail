'use client';

import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useTranslations } from 'next-intl';
import { CheckIcon, ClipboardIcon, KeyIcon, TrashIcon } from '@heroicons/react/24/outline';
import {
  createMCPAccessKey,
  getMCPSettings,
  listMCPAccessKeys,
  revokeMCPAccessKey,
  setMCPSettings,
  type MCPAccessKey,
  type MCPMailSettings,
  type MCPPermissionMode,
  type MCPSettings,
} from '@/lib/api';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';

const MCP_SCOPE_GROUPS = [
  { id: 'mail', labelKey: 'mcpScopeGroupMail', scopes: ['mail:read', 'mail:write', 'mail:send', 'mail:manage'] },
  { id: 'contacts', labelKey: 'mcpScopeGroupContacts', scopes: ['contacts:read', 'contacts:write', 'contacts:manage'] },
  { id: 'drive', labelKey: 'mcpScopeGroupDrive', scopes: ['drive:read', 'drive:write', 'drive:manage'] },
  { id: 'calendar', labelKey: 'mcpScopeGroupCalendar', scopes: ['calendar:read', 'calendar:write', 'calendar:manage'] },
] as const;

const MCP_SCOPE_LABEL_KEYS: Record<string, string> = {
  'mail:read': 'mcpScopeMailRead',
  'mail:write': 'mcpScopeMailWrite',
  'mail:send': 'mcpScopeMailSend',
  'mail:manage': 'mcpScopeMailManage',
  'contacts:read': 'mcpScopeContactsRead',
  'contacts:write': 'mcpScopeContactsWrite',
  'contacts:manage': 'mcpScopeContactsManage',
  'drive:read': 'mcpScopeDriveRead',
  'drive:write': 'mcpScopeDriveWrite',
  'drive:manage': 'mcpScopeDriveManage',
  'calendar:read': 'mcpScopeCalendarRead',
  'calendar:write': 'mcpScopeCalendarWrite',
  'calendar:manage': 'mcpScopeCalendarManage',
};

const DEFAULT_SETTINGS: MCPSettings = {
  enabled: false,
  permission_mode: 'basic',
  generated_mail_notice_enabled: true,
  generated_mail_notice_text: 'MCP를 통해 작성된 메일입니다.',
  require_confirmation_for_sensitive_actions: true,
  bypass_mode_allowed: false,
  mail: {
    send_enabled: true,
    confirm_external_recipients: true,
    confirm_attachments: true,
    daily_send_limit: 100,
  },
  scopes: {
    mail: ['read', 'write', 'send', 'delete'],
    contacts: ['read', 'write', 'delete'],
    drive: ['read', 'write', 'delete', 'share'],
    calendar: ['read', 'write', 'delete', 'invite'],
  },
};

function mergeSettings(settings?: Partial<MCPSettings>): MCPSettings {
  return {
    ...DEFAULT_SETTINGS,
    ...(settings ?? {}),
    mail: { ...DEFAULT_SETTINGS.mail, ...(settings?.mail ?? {}) },
    scopes: { ...DEFAULT_SETTINGS.scopes, ...(settings?.scopes ?? {}) },
  };
}

function inputStyle(width = '240px'): CSSProperties {
  return {
    width,
    maxWidth: '100%',
    padding: '7px 10px',
    borderRadius: '6px',
    border: '1px solid var(--color-border-default)',
    background: 'var(--color-bg-secondary)',
    color: 'var(--color-text-primary)',
    fontSize: '12px',
    outline: 'none',
  };
}

function boundedDailySendLimit(value: string): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  return Math.max(0, Math.min(10000, Math.trunc(parsed)));
}

export function SettingsMCPSection() {
  const t = useTranslations('settingsView');
  const [settings, setSettingsState] = useState<MCPSettings>(DEFAULT_SETTINGS);
  const [keys, setKeys] = useState<MCPAccessKey[]>([]);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyPermissionMode, setNewKeyPermissionMode] = useState<MCPPermissionMode>('basic');
  const [newKeyScopes, setNewKeyScopes] = useState<string[]>(['mail:read']);
  const [newKeyCIDRs, setNewKeyCIDRs] = useState('');
  const [newKeyExpiresAt, setNewKeyExpiresAt] = useState('');
  const [newToken, setNewToken] = useState('');
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState(false);

  useEffect(() => {
    Promise.all([getMCPSettings(), listMCPAccessKeys()])
      .then(([loadedSettings, loadedKeys]) => {
        setSettingsState(mergeSettings(loadedSettings));
        if (!loadedSettings.bypass_mode_allowed) setNewKeyPermissionMode('basic');
        setKeys(loadedKeys);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : t('mcpLoadFailed')))
      .finally(() => setLoading(false));
  }, [t]);

  const activeKeys = useMemo(() => keys.filter((key) => !key.revoked), [keys]);

  async function updateSettings(next: MCPSettings) {
    const normalized = mergeSettings(next);
    if (!normalized.bypass_mode_allowed && normalized.permission_mode === 'bypass') {
      normalized.permission_mode = 'basic';
    }
    setSettingsState(normalized);
    setSaved(false);
    setError('');
    try {
      const persisted = await setMCPSettings(normalized);
      setSettingsState(mergeSettings(persisted));
      setSaved(true);
      window.setTimeout(() => setSaved(false), 1800);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t('mcpSaveFailed'));
    }
  }

  function updateMailSettings(patch: Partial<MCPMailSettings>) {
    updateSettings({ ...settings, mail: { ...settings.mail, ...patch } });
  }

  function permissionModeLabel(mode: MCPPermissionMode) {
    return mode === 'bypass' ? t('mcpModeBypass') : t('mcpModeBasic');
  }

  function scopeLabel(scope: string) {
    const key = MCP_SCOPE_LABEL_KEYS[scope];
    return key ? t(key) : scope;
  }

  function scopeListLabel(scopes: string[]) {
    return scopes.map(scopeLabel).join(', ');
  }

  function toggleKeyScope(scope: string, checked: boolean) {
    setNewKeyScopes((prev) => {
      const next = checked ? [...prev, scope] : prev.filter((item) => item !== scope);
      return next.length > 0 ? Array.from(new Set(next)) : prev;
    });
  }

  function splitCIDRs(value: string) {
    return value.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean);
  }

  async function handleCreateKey() {
    const name = newKeyName.trim();
    if (!name) {
      setError(t('mcpKeyNameRequired'));
      return;
    }
    setSavingKey(true);
    setError('');
    setNewToken('');
    try {
      const created = await createMCPAccessKey({
        name,
        permission_mode: settings.bypass_mode_allowed ? newKeyPermissionMode : 'basic',
        scopes: newKeyScopes,
        allowed_cidrs: splitCIDRs(newKeyCIDRs),
        expires_at: newKeyExpiresAt ? new Date(newKeyExpiresAt).toISOString() : null,
      });
      setKeys((prev) => [created.key, ...prev]);
      setNewKeyName('');
      setNewKeyScopes(['mail:read']);
      setNewKeyCIDRs('');
      setNewKeyExpiresAt('');
      setNewToken(created.token);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t('mcpKeyCreateFailed'));
    } finally {
      setSavingKey(false);
    }
  }

  async function handleRevoke(key: MCPAccessKey) {
    if (!window.confirm(t('mcpKeyRevokeConfirm', { name: key.name }))) return;
    setError('');
    try {
      const revoked = await revokeMCPAccessKey(key.id);
      setKeys((prev) => prev.map((item) => (item.id === key.id ? revoked : item)));
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : t('mcpKeyRevokeFailed'));
    }
  }

  async function copyToken() {
    if (!newToken) return;
    await navigator.clipboard?.writeText(newToken);
  }

  if (loading) {
    return (
      <SectionCard>
        <SectionHeader>{t('sectionMcp')}</SectionHeader>
        <div style={{ padding: '18px 20px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{t('mcpLoading')}</div>
      </SectionCard>
    );
  }

  return (
    <>
      {error && (
        <div style={{ border: '1px solid rgba(220,38,38,0.25)', background: 'rgba(220,38,38,0.06)', color: 'var(--color-destructive)', borderRadius: '8px', padding: '10px 12px', fontSize: '12px', marginBottom: '16px' }}>
          {error}
        </div>
      )}
      {saved && (
        <div style={{ color: 'var(--color-accent)', display: 'flex', alignItems: 'center', gap: '5px', fontSize: '12px', marginBottom: '10px' }}>
          <CheckIcon style={{ width: 14, height: 14 }} /> {t('saved')}
        </div>
      )}

      <SectionCard>
        <SectionHeader>{t('sectionMcp')}</SectionHeader>
        <Row label={t('mcpEnabled')} description={t('mcpEnabledDesc')}>
          <Toggle value={settings.enabled} onChange={(enabled) => updateSettings({ ...settings, enabled })} />
        </Row>
        <Row label={t('mcpSensitiveConfirm')} description={t('mcpSensitiveConfirmDesc')}>
          <Toggle value={settings.require_confirmation_for_sensitive_actions} onChange={(require_confirmation_for_sensitive_actions) => updateSettings({ ...settings, require_confirmation_for_sensitive_actions })} />
        </Row>
        <Row label={t('mcpGeneratedNotice')} description={t('mcpGeneratedNoticeDesc')}>
          <Toggle value={settings.generated_mail_notice_enabled} onChange={(generated_mail_notice_enabled) => updateSettings({ ...settings, generated_mail_notice_enabled })} disabled={settings.generated_mail_notice_forced} />
        </Row>
        {settings.generated_mail_notice_enabled && (
          <Row label={t('mcpGeneratedNoticeText')} description={t('mcpGeneratedNoticeTextDesc')} last>
            <input
              value={settings.generated_mail_notice_text}
              disabled={settings.generated_mail_notice_forced}
              onChange={(e) => setSettingsState({ ...settings, generated_mail_notice_text: e.target.value })}
              onBlur={(e) => updateSettings({ ...settings, generated_mail_notice_text: e.currentTarget.value })}
              style={inputStyle('280px')}
            />
          </Row>
        )}
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('sectionMcpGranular')}</SectionHeader>
        <Row label={t('mcpMailSend')} description={t('mcpMailSendDesc')}>
          <Toggle value={settings.mail.send_enabled} onChange={(send_enabled) => updateMailSettings({ send_enabled })} />
        </Row>
        {settings.mail.send_enabled && (
          <>
            <Row label={t('mcpExternalConfirm')} description={t('mcpExternalConfirmDesc')}>
              <Toggle value={settings.mail.confirm_external_recipients} onChange={(confirm_external_recipients) => updateMailSettings({ confirm_external_recipients })} />
            </Row>
            <Row label={t('mcpAttachmentConfirm')} description={t('mcpAttachmentConfirmDesc')}>
              <Toggle value={settings.mail.confirm_attachments} onChange={(confirm_attachments) => updateMailSettings({ confirm_attachments })} />
            </Row>
            <Row label={t('mcpDailySendLimit')} description={t('mcpDailySendLimitDesc')} last>
              <input
                type="number"
                min={0}
                max={10000}
                value={String(settings.mail.daily_send_limit)}
                onChange={(e) => setSettingsState({ ...settings, mail: { ...settings.mail, daily_send_limit: boundedDailySendLimit(e.target.value) } })}
                onBlur={(e) => updateMailSettings({ daily_send_limit: boundedDailySendLimit(e.currentTarget.value) })}
                style={inputStyle('110px')}
              />
            </Row>
          </>
        )}
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('sectionMcpKeys')}</SectionHeader>
        <div style={{ padding: '14px 20px', display: 'grid', gap: '12px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          {!settings.enabled && (
            <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('mcpKeysDisabledUntilEnabled')}</div>
          )}
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
          <input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder={t('mcpKeyNamePlaceholder')} style={inputStyle('220px')} />
          {settings.bypass_mode_allowed && (
            <Segment
              options={[{ value: 'basic' as MCPPermissionMode, label: t('mcpModeBasic') }, { value: 'bypass' as MCPPermissionMode, label: t('mcpModeBypass') }]}
              value={newKeyPermissionMode}
              onChange={(permission_mode) => setNewKeyPermissionMode(permission_mode)}
            />
          )}
          <input type="datetime-local" value={newKeyExpiresAt} onChange={(e) => setNewKeyExpiresAt(e.target.value)} style={inputStyle('190px')} aria-label={t('mcpKeyExpiresAt')} />
          <button onClick={handleCreateKey} disabled={savingKey || !settings.enabled || !newKeyName.trim() || newKeyScopes.length === 0} style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 12px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: savingKey ? 'wait' : 'pointer' }}>
            <KeyIcon style={{ width: 14, height: 14 }} /> {savingKey ? t('saving') : t('mcpCreateKey')}
          </button>
          </div>
          <input value={newKeyCIDRs} onChange={(e) => setNewKeyCIDRs(e.target.value)} placeholder={t('mcpKeyCIDRsPlaceholder')} style={inputStyle('520px')} />
          <div style={{ display: 'grid', gap: '10px' }}>
            {MCP_SCOPE_GROUPS.map((group) => (
              <div key={group.id} style={{ display: 'grid', gap: '6px' }}>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--color-text-secondary)' }}>{t(group.labelKey)}</div>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  {group.scopes.map((scope) => (
                    <label key={scope} title={scope} style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '11px', color: 'var(--color-text-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '6px', padding: '5px 7px' }}>
                      <input type="checkbox" checked={newKeyScopes.includes(scope)} onChange={(e) => toggleKeyScope(scope, e.target.checked)} />
                      {scopeLabel(scope)}
                    </label>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
        {newToken && (
          <div style={{ padding: '12px 20px', background: 'var(--color-bg-secondary)', borderBottom: '1px solid var(--color-border-subtle)' }}>
            <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '6px' }}>{t('mcpTokenShownOnce')}</div>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <code style={{ flex: 1, minWidth: 0, wordBreak: 'break-all', fontSize: '12px', color: 'var(--color-text-secondary)' }}>{newToken}</code>
              <button onClick={copyToken} title={t('mcpCopyToken')} style={{ border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', borderRadius: '6px', padding: '6px', cursor: 'pointer' }}>
                <ClipboardIcon style={{ width: 15, height: 15 }} />
              </button>
            </div>
          </div>
        )}
        <div style={{ background: 'var(--color-bg-primary)' }}>
          {activeKeys.length === 0 ? (
            <div style={{ padding: '16px 20px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('mcpNoKeys')}</div>
          ) : activeKeys.map((key, index) => (
            <div key={key.id} style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center', padding: '12px 20px', borderBottom: index === activeKeys.length - 1 ? 'none' : '1px solid var(--color-border-subtle)' }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{key.name}</div>
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>...{key.token_suffix} · {permissionModeLabel(key.permission_mode)} · {scopeListLabel(key.scopes)}</div>
              </div>
              <button onClick={() => handleRevoke(key)} title={t('mcpRevokeKey')} style={{ border: '1px solid rgba(220,38,38,0.35)', background: 'transparent', color: 'var(--color-destructive)', borderRadius: '6px', padding: '6px', cursor: 'pointer' }}>
                <TrashIcon style={{ width: 15, height: 15 }} />
              </button>
            </div>
          ))}
        </div>
      </SectionCard>
    </>
  );
}
