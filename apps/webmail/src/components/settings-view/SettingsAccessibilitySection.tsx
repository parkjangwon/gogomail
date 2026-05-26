'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from './settingsViewPrimitives';

export interface SettingsAccessibilitySectionProps {
  reducedMotion: boolean;
  setReducedMotion: (v: boolean) => void;
  highContrast: boolean;
  setHighContrast: (v: boolean) => void;
  underlineLinks: boolean;
  setUnderlineLinks: (v: boolean) => void;
  largerClickTargets: boolean;
  setLargerClickTargets: (v: boolean) => void;
  screenReaderMode: boolean;
  setScreenReaderMode: (v: boolean) => void;
  fontFamily: 'system' | 'serif' | 'mono';
  setFontFamily: (v: 'system' | 'serif' | 'mono') => void;
  colorBlindMode: 'none' | 'deuteranopia' | 'protanopia' | 'tritanopia';
  alwaysFocusRing: boolean;
  setAlwaysFocusRing: (v: boolean) => void;
  dyslexiaMode: boolean;
  setDyslexiaMode: (v: boolean) => void;
  uiFontSize: 'sm' | 'md' | 'lg' | 'xl';
  lineSpacing: 'normal' | 'relaxed' | 'loose';
  letterSpacing: 'normal' | 'wide';
  applyColorBlindMode: (mode: 'none' | 'deuteranopia' | 'protanopia' | 'tritanopia') => void;
  applyUiFontSize: (size: 'sm' | 'md' | 'lg' | 'xl') => void;
  applyLineSpacing: (spacing: 'normal' | 'relaxed' | 'loose') => void;
  applyLetterSpacing: (spacing: 'normal' | 'wide') => void;
}

export function SettingsAccessibilitySection({
  reducedMotion,
  setReducedMotion,
  highContrast,
  setHighContrast,
  underlineLinks,
  setUnderlineLinks,
  largerClickTargets,
  setLargerClickTargets,
  screenReaderMode,
  setScreenReaderMode,
  fontFamily,
  setFontFamily,
  colorBlindMode,
  alwaysFocusRing,
  setAlwaysFocusRing,
  dyslexiaMode,
  setDyslexiaMode,
  uiFontSize,
  lineSpacing,
  letterSpacing,
  applyColorBlindMode,
  applyUiFontSize,
  applyLineSpacing,
  applyLetterSpacing,
}: SettingsAccessibilitySectionProps) {
  const t = useTranslations('settingsView');

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
}
