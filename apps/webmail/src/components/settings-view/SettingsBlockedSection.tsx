'use client';
import { useTranslations } from 'next-intl';
import { setPreferences } from '@/lib/api';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';
import { SenderListTable } from './SenderListTable';

interface SettingsBlockedSectionProps {
  blockedSenders: string[];
  setBlockedSenders: (v: string[]) => void;
  blockedMeta: Record<string, string>;
  setBlockedMeta: (v: Record<string, string>) => void;
  newBlockedInput: string;
  setNewBlockedInput: (v: string) => void;
  blockedSearch: string;
  setBlockedSearch: (v: string) => void;
  blockedPage: number;
  setBlockedPage: (v: number) => void;
  spamAutoDeleteDays: number;
  setSpamAutoDeleteDays: (v: number) => void;
  spamAutoBlock: boolean;
  setSpamAutoBlock: (v: boolean) => void;
  allowedSenders: string[];
  setAllowedSenders: (v: string[]) => void;
  allowedMeta: Record<string, string>;
  setAllowedMeta: (v: Record<string, string>) => void;
  newAllowedInput: string;
  setNewAllowedInput: (v: string) => void;
  allowedSearch: string;
  setAllowedSearch: (v: string) => void;
  allowedPage: number;
  setAllowedPage: (v: number) => void;
}

export function SettingsBlockedSection({
  blockedSenders,
  setBlockedSenders,
  blockedMeta,
  setBlockedMeta,
  newBlockedInput,
  setNewBlockedInput,
  blockedSearch,
  setBlockedSearch,
  blockedPage,
  setBlockedPage,
  spamAutoDeleteDays,
  setSpamAutoDeleteDays,
  spamAutoBlock,
  setSpamAutoBlock,
  allowedSenders,
  setAllowedSenders,
  allowedMeta,
  setAllowedMeta,
  newAllowedInput,
  setNewAllowedInput,
  allowedSearch,
  setAllowedSearch,
  allowedPage,
  setAllowedPage,
}: SettingsBlockedSectionProps) {
  const t = useTranslations('settingsView');

  const PAGE_SIZE = 5;

  // ── Blocked senders derived state ──
  const bq = blockedSearch.trim().toLowerCase();
  const filteredBlocked = bq ? blockedSenders.filter((a) => a.includes(bq)) : blockedSenders;
  const blockedTotalPages = Math.ceil(filteredBlocked.length / PAGE_SIZE);
  const safeBlockedPage = Math.min(blockedPage, Math.max(0, blockedTotalPages - 1));
  const blockedPageItems = filteredBlocked.slice(safeBlockedPage * PAGE_SIZE, (safeBlockedPage + 1) * PAGE_SIZE);

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
    setBlockedPage(Math.floor(blockedSenders.length / PAGE_SIZE));
  }
  function removeBlocked(addr: string) {
    const next = blockedSenders.filter((a) => a !== addr);
    const nextMeta = { ...blockedMeta };
    delete nextMeta[addr];
    saveBlocked(next, nextMeta);
    const newTotal = Math.ceil(next.length / PAGE_SIZE);
    if (safeBlockedPage >= newTotal && safeBlockedPage > 0) setBlockedPage(safeBlockedPage - 1);
  }
  function formatBlockedDate(addr: string): string {
    const iso = blockedMeta[addr];
    if (!iso) return '—';
    try {
      return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso));
    } catch { return iso.slice(0, 10); }
  }

  // ── Allowed senders derived state ──
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

  const autoDeleteOptions: { value: number; labelKey: string }[] = [
    { value: 14, labelKey: 'spamDelete14' },
    { value: 30, labelKey: 'spamDelete30' },
    { value: 60, labelKey: 'spamDelete60' },
    { value: 90, labelKey: 'spamDelete90' },
    { value: 0, labelKey: 'spamDeleteNever' },
  ];

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

      {/* ── 차단된 발신자 ── */}
      <SenderListTable
        variant="blocked"
        senders={blockedSenders}
        meta={blockedMeta}
        search={blockedSearch}
        setSearch={setBlockedSearch}
        setPage={setBlockedPage}
        newInput={newBlockedInput}
        setNewInput={setNewBlockedInput}
        onAdd={addBlocked}
        onRemove={removeBlocked}
        formatDate={formatBlockedDate}
        filteredSenders={filteredBlocked}
        pageItems={blockedPageItems}
        totalPages={blockedTotalPages}
        safePage={safeBlockedPage}
      />

      {/* ── 허용된 발신자 ── */}
      <SenderListTable
        variant="allowed"
        senders={allowedSenders}
        meta={allowedMeta}
        search={allowedSearch}
        setSearch={setAllowedSearch}
        setPage={setAllowedPage}
        newInput={newAllowedInput}
        setNewInput={setNewAllowedInput}
        onAdd={addAllowed}
        onRemove={removeAllowed}
        formatDate={formatAllowedDate}
        filteredSenders={filteredAllowed}
        pageItems={allowedPageItems}
        totalPages={allowedTotalPages}
        safePage={safeAllowedPage}
      />
    </>
  );
}
