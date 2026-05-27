'use client';

import { useTranslations } from 'next-intl';
import { type WebmailSettings } from '../settings/settingsConfig';
import { sectionStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  handleNotificationToggle: (checked: boolean) => void;
}

export function NotificationsSection({ settings, handleNotificationToggle }: Props) {
  const t = useTranslations('settingsModal');

  return (
    <div style={sectionStyle}>
      <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
        <input type="checkbox" checked={settings.notifications} onChange={(e) => handleNotificationToggle(e.target.checked)} />
        <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('notifications')}</span>
      </label>
      {settings.notifications && (
        <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '8px', marginLeft: '24px' }}>{t('notificationsActive')}</p>
      )}
    </div>
  );
}
