'use client';
import React from 'react';
import { useTranslations } from 'next-intl';
import { CheckIcon } from '@heroicons/react/24/outline';
import { MiniEditor, Row, SectionCard, SectionHeader, Toggle } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsVacationSectionProps {
  vacEnabled: boolean;
  setVacEnabled: (v: boolean) => void;
  vacStartDate: string;
  setVacStartDate: (v: string) => void;
  vacEndDate: string;
  setVacEndDate: (v: string) => void;
  vacSubject: string;
  setVacSubject: (v: string) => void;
  vacBody: string;
  setVacBody: (v: string) => void;
  vacSaved: boolean;
  setVacSaved: (v: boolean) => void;
}

export function SettingsVacationSection({
  vacEnabled,
  setVacEnabled,
  vacStartDate,
  setVacStartDate,
  vacEndDate,
  setVacEndDate,
  vacSubject,
  setVacSubject,
  vacBody,
  setVacBody,
  vacSaved,
  setVacSaved,
}: SettingsVacationSectionProps) {
  const t = useTranslations('settingsView');

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
