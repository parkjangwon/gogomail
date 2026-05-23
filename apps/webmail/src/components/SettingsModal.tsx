'use client';

import { useState, useEffect } from 'react';
import { XMarkIcon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
import { SettingsModalContent } from './SettingsModalContent';
import { getPreferences, setPreferences, type WebmailPreferences } from '@/lib/api';
import {
  ACCENT_PRESETS,
  CATEGORIES,
  Category,
  DEFAULT_SETTINGS,
  FilterAction,
  FilterCondition,
  FilterRule,
  LABEL_COLORS,
  WebmailSettings,
  applyAccent,
  createEmptyRule,
  loadFilterRules,
  loadSettings,
  migrateRule,
  saveFilterRules,
  saveSettings,
} from './settings/settingsConfig';

interface SettingsModalProps {
  onClose: () => void;
  userEmail?: string;
}

export function SettingsModal({ onClose, userEmail }: SettingsModalProps) {
  const tNav = useTranslations('nav');
  const tCommon = useTranslations('common');
  const tSettingsView = useTranslations('settingsView');
  const [activeCategory, setActiveCategory] = useState<Category>('mailbox');
  const [settings, setSettings] = useState<WebmailSettings>(DEFAULT_SETTINGS);
  const [hoveredCategory, setHoveredCategory] = useState<Category | null>(null);
  const [avatarUrl, setAvatarUrl] = useState('');
  const [filterRules, setFilterRules] = useState<FilterRule[]>([]);
  const [prefsLoaded, setPrefsLoaded] = useState(false);
  const [editingRule, setEditingRule] = useState<FilterRule | null>(null);
  const [newRule, setNewRule] = useState<Omit<FilterRule, 'id'>>(createEmptyRule());

  useEffect(() => {
    setSettings(loadSettings());
    setFilterRules(loadFilterRules());
    try { setAvatarUrl(localStorage.getItem('webmail_avatar') ?? ''); } catch { /* */ }
    getPreferences().then((prefs: WebmailPreferences) => {
      if (prefs.settings || prefs.signatures) {
        setSettings((prev) => {
          const next = {
            ...prev,
            ...(prefs.settings as Partial<WebmailSettings> | undefined),
            signature: prefs.signatures?.default ?? prev.signature,
          };
          saveSettings(next);
          return next;
        });
      }
      if (prefs.filter_rules) {
        const serverRules = (prefs.filter_rules as Record<string, unknown>[]).map(migrateRule);
        setFilterRules(serverRules);
        saveFilterRules(serverRules);
      }
      setPrefsLoaded(true);
    }).catch(() => setPrefsLoaded(true));
  }, []);

  useEffect(() => {
    if (settings.accentColor) applyAccent(settings.accentColor);
  }, [settings.accentColor]);

  useEffect(() => {
    if (!prefsLoaded) return;
    const timer = setTimeout(() => {
      const { signature, ...settingsPrefs } = settings;
      setPreferences({
        settings: settingsPrefs as unknown as Record<string, unknown>,
        signatures: { default: signature },
        filter_rules: filterRules as unknown[],
      }).catch(() => {});
    }, 500);
    return () => clearTimeout(timer);
  }, [filterRules, prefsLoaded, settings]);

  function update<K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) {
    setSettings((prev) => {
      const next = { ...prev, [key]: value };
      saveSettings(next);
      return next;
    });
  }

  function applyTheme(theme: WebmailSettings['theme']) {
    update('theme', theme);
    if (theme === 'dark') {
      document.documentElement.setAttribute('data-theme', 'dark');
    } else if (theme === 'light') {
      document.documentElement.setAttribute('data-theme', 'light');
    } else {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    }
  }

  function handleNotificationToggle(checked: boolean) {
    if (checked && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
      Notification.requestPermission().then((perm) => {
        update('notifications', perm === 'granted');
      });
    } else {
      update('notifications', checked);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={tNav('settings')}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div
        style={{
          width: '680px',
          height: '520px',
          borderRadius: '12px',
          background: 'var(--color-bg-primary)',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 20px 60px rgba(0,0,0,0.25)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--color-border-subtle)',
            flexShrink: 0,
          }}
        >
          <span style={{ fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{tNav('settings')}</span>
          <button
            aria-label={tCommon('close')}
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--color-text-tertiary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '4px',
              borderRadius: '6px',
            }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'none'; (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        </div>

        {/* Body */}
        <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
          {/* Left nav */}
          <div
            style={{
              width: '160px',
              flexShrink: 0,
              borderRight: '1px solid var(--color-border-subtle)',
              padding: '8px 0',
              overflowY: 'auto',
            }}
          >
            {CATEGORIES.map(({ id, labelKey }) => {
              const isActive = activeCategory === id;
              const isHovered = hoveredCategory === id;
              return (
                <button
                  key={id}
                  onClick={() => setActiveCategory(id)}
                  onMouseEnter={() => setHoveredCategory(id)}
                  onMouseLeave={() => setHoveredCategory(null)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    padding: '9px 16px',
                    fontSize: '13px',
                    fontWeight: isActive ? 500 : 400,
                    color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                    background: isActive
                      ? 'var(--color-bg-tertiary)'
                      : isHovered
                      ? 'var(--color-bg-secondary)'
                      : 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    transition: 'background 100ms ease',
                  }}
                >
                  {tSettingsView(labelKey)}
                </button>
              );
            })}
          </div>

          {/* Right content */}
          <div
            style={{
              flex: 1,
              overflowY: 'auto',
              padding: '24px',
            }}
          >
            <SettingsModalContent
              activeCategory={activeCategory}
              settings={settings}
              userEmail={userEmail}
              avatarUrl={avatarUrl}
              setAvatarUrl={setAvatarUrl}
              update={update}
              applyTheme={applyTheme}
              handleNotificationToggle={handleNotificationToggle}
              filterRules={filterRules}
              setFilterRules={setFilterRules}
              editingRule={editingRule}
              setEditingRule={setEditingRule}
              newRule={newRule}
              setNewRule={setNewRule}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
