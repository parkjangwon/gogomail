'use client';

import { useState, useEffect, useRef } from 'react';
import { listOrgTree, listAddressBooks, listContacts, parseVCard, OrgUnit, AddressBook } from '@/lib/api';

export interface PickerItem {
  id: string;
  display_name: string;
  email: string;
}

interface OrgPickerModalProps {
  initialTo?: PickerItem[];
  initialCc?: PickerItem[];
  initialBcc?: PickerItem[];
  onClose: () => void;
  onConfirm: (result: { to: PickerItem[]; cc: PickerItem[]; bcc: PickerItem[] }) => void;
}

// ── Helpers ────────────────────────────────────────────────────────────────────

export function parseToPickerItems(str: string): PickerItem[] {
  if (!str.trim()) return [];
  const parts: string[] = [];
  let depth = 0, start = 0;
  for (let i = 0; i < str.length; i++) {
    if (str[i] === '<') depth++;
    else if (str[i] === '>') depth--;
    else if (str[i] === ',' && depth === 0) {
      parts.push(str.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(str.slice(start));
  return parts
    .map((p) => p.trim())
    .filter(Boolean)
    .map((p) => {
      const m = p.match(/^(.+?)\s*<([^>]+)>$/);
      if (m) {
        const name = m[1].trim();
        const email = m[2].trim();
        return { id: email, display_name: name || email, email };
      }
      return { id: p, display_name: p, email: p };
    });
}

export function pickerItemsToString(items: PickerItem[]): string {
  return items
    .map((i) => (i.display_name && i.display_name !== i.email ? `${i.display_name} <${i.email}>` : i.email))
    .join(', ');
}

// ── Component ──────────────────────────────────────────────────────────────────

const FIELD_LABELS = { to: '받는 사람', cc: '참조', bcc: '숨은 참조' } as const;

export function OrgPickerModal({
  initialTo = [],
  initialCc = [],
  initialBcc = [],
  onClose,
  onConfirm,
}: OrgPickerModalProps) {
  const [tab, setTab] = useState<'org' | 'contacts'>('org');

  // Org tree
  const [orgTree, setOrgTree] = useState<OrgUnit[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<OrgUnit | null>(null);
  const [treeLoading, setTreeLoading] = useState(false);
  const [orgSearch, setOrgSearch] = useState('');

  // Address book
  const [addressBooks, setAddressBooks] = useState<AddressBook[]>([]);
  const [selectedBook, setSelectedBook] = useState<AddressBook | null>(null);
  const [bookContacts, setBookContacts] = useState<PickerItem[]>([]);
  const [booksLoading, setBooksLoading] = useState(false);
  const [bookLoading, setBookLoading] = useState(false);

  // Recipients
  const [toList, setToList] = useState<Map<string, PickerItem>>(
    () => new Map(initialTo.map((i) => [i.id, i]))
  );
  const [ccList, setCcList] = useState<Map<string, PickerItem>>(
    () => new Map(initialCc.map((i) => [i.id, i]))
  );
  const [bccList, setBccList] = useState<Map<string, PickerItem>>(
    () => new Map(initialBcc.map((i) => [i.id, i]))
  );
  const [activeField, setActiveField] = useState<'to' | 'cc' | 'bcc'>('to');

  const orgSearchRef = useRef<HTMLInputElement>(null);

  // Load org tree on mount
  useEffect(() => {
    setTreeLoading(true);
    listOrgTree()
      .then((units) => {
        setOrgTree(units);
        // Select the first leaf node (depth > 0) or first unit
        const first = units.find((u) => u.depth > 0) ?? units[0] ?? null;
        setSelectedOrg(first);
        setTreeLoading(false);
      })
      .catch(() => setTreeLoading(false));
  }, []);

  // Load address books when switching to contacts tab
  useEffect(() => {
    if (tab !== 'contacts') return;
    if (addressBooks.length > 0) return;
    setBooksLoading(true);
    listAddressBooks()
      .then((books) => {
        setAddressBooks(books);
        if (books.length > 0 && !selectedBook) setSelectedBook(books[0]);
        setBooksLoading(false);
      })
      .catch(() => setBooksLoading(false));
  }, [tab, addressBooks.length, selectedBook]);

  // Load contacts when selectedBook changes
  useEffect(() => {
    if (!selectedBook) return;
    setBookLoading(true);
    listContacts(selectedBook.ID)
      .then((contacts) => {
        const items: PickerItem[] = contacts
          .map((c) => {
            const parsed = parseVCard(c.VCard);
            return { id: c.ID, display_name: parsed.fn || parsed.email, email: parsed.email };
          })
          .filter((i) => !!i.email);
        setBookContacts(items);
        setBookLoading(false);
      })
      .catch(() => setBookLoading(false));
  }, [selectedBook]);

  // ── Recipients helpers ────────────────────────────────────────────────────────

  function getActiveList(): Map<string, PickerItem> {
    if (activeField === 'to') return toList;
    if (activeField === 'cc') return ccList;
    return bccList;
  }

  function setActiveList(next: Map<string, PickerItem>) {
    if (activeField === 'to') setToList(next);
    else if (activeField === 'cc') setCcList(next);
    else setBccList(next);
  }

  function addToActive(item: PickerItem) {
    const cur = getActiveList();
    if (cur.has(item.id)) return;
    const next = new Map(cur);
    next.set(item.id, item);
    setActiveList(next);
  }

  function removeFromList(field: 'to' | 'cc' | 'bcc', id: string) {
    const setter = field === 'to' ? setToList : field === 'cc' ? setCcList : setBccList;
    const cur = field === 'to' ? toList : field === 'cc' ? ccList : bccList;
    const next = new Map(cur);
    next.delete(id);
    setter(next);
  }

  function clearList(field: 'to' | 'cc' | 'bcc') {
    if (field === 'to') setToList(new Map());
    else if (field === 'cc') setCcList(new Map());
    else setBccList(new Map());
  }

  // ── Org search filtering ──────────────────────────────────────────────────────

  const q = orgSearch.trim().toLowerCase();

  const filteredOrgs: OrgUnit[] = q
    ? orgTree.filter(
        (u) =>
          u.display_name.toLowerCase().includes(q) ||
          u.members.some(
            (m) =>
              (m.display_name || '').toLowerCase().includes(q) ||
              m.email.toLowerCase().includes(q)
          )
      )
    : orgTree;

  // Middle pane items
  const middleItems: PickerItem[] = (() => {
    if (tab === 'contacts') return bookContacts;
    if (q) {
      // All matching members across all orgs (deduped by id)
      const seen = new Set<string>();
      return orgTree
        .flatMap((u) => u.members)
        .filter((m) => {
          if (seen.has(m.id)) return false;
          const match =
            (m.display_name || '').toLowerCase().includes(q) || m.email.toLowerCase().includes(q);
          if (match) seen.add(m.id);
          return match;
        })
        .map((m) => ({ id: m.id, display_name: m.display_name || m.email, email: m.email }));
    }
    return (selectedOrg?.members ?? []).map((m) => ({
      id: m.id,
      display_name: m.display_name || m.email,
      email: m.email,
    }));
  })();

  // ── Styles ────────────────────────────────────────────────────────────────────

  const rowHover = {
    onMouseEnter: (e: React.MouseEvent<HTMLElement>) => {
      (e.currentTarget as HTMLElement).style.background = 'var(--color-bg-secondary)';
    },
    onMouseLeave: (e: React.MouseEvent<HTMLElement>) => {
      (e.currentTarget as HTMLElement).style.background = 'transparent';
    },
  };

  const avatarStyle: React.CSSProperties = {
    width: '32px', height: '32px', borderRadius: '50%',
    background: 'var(--color-accent-subtle)', color: 'var(--color-accent)',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    fontSize: '13px', fontWeight: 600, flexShrink: 0,
  };

  // ── Right pane section ────────────────────────────────────────────────────────

  function renderRightSection(field: 'to' | 'cc' | 'bcc') {
    const list = field === 'to' ? toList : field === 'cc' ? ccList : bccList;
    const isActive = activeField === field;
    return (
      <div
        key={field}
        style={{
          flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0,
          borderBottom: field !== 'bcc' ? '1px solid var(--color-border-subtle)' : undefined,
        }}
      >
        <div
          onClick={() => setActiveField(field)}
          style={{
            display: 'flex', alignItems: 'center', padding: '6px 10px',
            cursor: 'pointer', flexShrink: 0, userSelect: 'none',
            background: isActive ? 'var(--color-accent-subtle)' : 'transparent',
          }}
        >
          <span style={{ fontSize: '12px', fontWeight: 600, flex: 1, color: isActive ? 'var(--color-accent)' : 'var(--color-text-secondary)' }}>
            {FIELD_LABELS[field]} ({list.size})
          </span>
          {list.size > 0 && (
            <button type="button" onClick={(e) => { e.stopPropagation(); clearList(field); }}
              style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '0 2px' }}>
              전체 삭제
            </button>
          )}
        </div>
        <div style={{ overflowY: 'auto', flex: 1 }}>
          {Array.from(list.values()).map((item) => (
            <div key={item.id} style={{ display: 'flex', alignItems: 'center', padding: '4px 10px', gap: '6px' }} {...rowHover}>
              <span style={{ fontSize: '12px', color: 'var(--color-text-primary)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {item.display_name !== item.email ? item.display_name : item.email}
              </span>
              <button type="button" onClick={() => removeFromList(field, item.id)}
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '14px', lineHeight: 1, padding: '0 2px', flexShrink: 0 }}>
                ×
              </button>
            </div>
          ))}
        </div>
      </div>
    );
  }

  // ── Render ────────────────────────────────────────────────────────────────────

  return (
    <div
      style={{ position: 'fixed', inset: 0, zIndex: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.35)' }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{
        background: 'var(--color-bg-primary)', borderRadius: '12px',
        boxShadow: '0 8px 40px rgba(0,0,0,0.22)',
        width: 'min(900px, calc(100vw - 40px))',
        height: 'min(600px, calc(100vh - 60px))',
        display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>

        {/* Header — tabs */}
        <div style={{ display: 'flex', alignItems: 'center', padding: '0 16px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
          {(['org', 'contacts'] as const).map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)}
              style={{
                padding: '12px 14px', fontSize: '13px',
                fontWeight: tab === t ? 600 : 400,
                color: tab === t ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                background: 'none', border: 'none', cursor: 'pointer',
                borderBottom: tab === t ? '2px solid var(--color-accent)' : '2px solid transparent',
                marginBottom: '-1px', whiteSpace: 'nowrap',
              }}>
              {t === 'org' ? '조직도' : '주소록'}
            </button>
          ))}
        </div>

        {/* Body: 3 panes */}
        <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>

          {/* Left pane */}
          <div style={{ width: '210px', flexShrink: 0, borderRight: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column' }}>

            {/* Org tab: search input */}
            {tab === 'org' && (
              <div style={{ padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
                <input
                  ref={orgSearchRef}
                  type="text" value={orgSearch} onChange={(e) => setOrgSearch(e.target.value)}
                  placeholder="조직명, 이름, 이메일 검색"
                  style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', outline: 'none', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
                />
              </div>
            )}

            <div style={{ flex: 1, overflowY: 'auto' }}>
              {/* Org tab tree */}
              {tab === 'org' && treeLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>불러오는 중...</div>
              )}
              {tab === 'org' && !treeLoading && filteredOrgs.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{q ? '결과 없음' : '조직 정보 없음'}</div>
              )}
              {tab === 'org' && filteredOrgs.map((unit) => {
                const isSelected = !q && selectedOrg?.id === unit.id;
                return (
                  <div key={unit.id}
                    onClick={() => { setOrgSearch(''); setSelectedOrg(unit); }}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '4px',
                      paddingTop: '8px', paddingBottom: '8px',
                      paddingLeft: `${10 + unit.depth * 14}px`, paddingRight: '10px',
                      cursor: 'pointer',
                      borderLeft: isSelected ? '3px solid var(--color-accent)' : '3px solid transparent',
                      background: isSelected ? 'var(--color-accent-subtle)' : 'transparent',
                      fontWeight: unit.depth === 0 ? 600 : 400,
                    }}
                    {...(!isSelected ? rowHover : {})}
                  >
                    {unit.depth === 0 && (
                      <span style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px' }}>▸</span>
                    )}
                    <span style={{ fontSize: '13px', color: isSelected ? 'var(--color-accent)' : 'var(--color-text-primary)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {unit.display_name}
                    </span>
                    <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{unit.members.length}</span>
                  </div>
                );
              })}

              {/* Contacts tab address books */}
              {tab === 'contacts' && booksLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>불러오는 중...</div>
              )}
              {tab === 'contacts' && !booksLoading && addressBooks.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>주소록 없음</div>
              )}
              {tab === 'contacts' && addressBooks.map((book) => {
                const isSelected = selectedBook?.ID === book.ID;
                return (
                  <div key={book.ID}
                    onClick={() => setSelectedBook(book)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '6px',
                      padding: '9px 12px', cursor: 'pointer',
                      borderLeft: isSelected ? '3px solid var(--color-accent)' : '3px solid transparent',
                      background: isSelected ? 'var(--color-accent-subtle)' : 'transparent',
                    }}
                    {...(!isSelected ? rowHover : {})}
                  >
                    <span style={{ fontSize: '13px', color: isSelected ? 'var(--color-accent)' : 'var(--color-text-primary)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontWeight: isSelected ? 600 : 400 }}>
                      {book.Name}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Middle pane — member / contact list */}
          <div style={{ flex: 1, overflowY: 'auto', borderRight: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column' }}>
            {/* Empty states */}
            {tab === 'org' && !treeLoading && !q && !selectedOrg && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>왼쪽에서 조직을 선택하세요</div>
            )}
            {tab === 'org' && !treeLoading && !q && selectedOrg && selectedOrg.members.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>구성원 없음</div>
            )}
            {tab === 'org' && q && middleItems.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>결과 없음</div>
            )}
            {tab === 'contacts' && bookLoading && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>불러오는 중...</div>
            )}
            {tab === 'contacts' && !bookLoading && !selectedBook && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>왼쪽에서 주소록을 선택하세요</div>
            )}
            {tab === 'contacts' && !bookLoading && selectedBook && bookContacts.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>연락처 없음</div>
            )}

            {/* Item rows */}
            {middleItems.map((item) => (
              <div key={item.id}
                style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 12px', cursor: 'default' }}
                {...rowHover}
              >
                <div style={avatarStyle}>{(item.display_name || item.email)[0].toUpperCase()}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {item.display_name}
                  </div>
                  {item.display_name !== item.email && (
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {item.email}
                    </div>
                  )}
                </div>
                <button type="button" onClick={() => addToActive(item)}
                  title={`${FIELD_LABELS[activeField]}에 추가`}
                  style={{ padding: '3px 8px', fontSize: '12px', color: 'var(--color-accent)', border: '1px solid var(--color-accent)', borderRadius: '4px', background: 'transparent', cursor: 'pointer', flexShrink: 0 }}>
                  &gt;
                </button>
              </div>
            ))}
          </div>

          {/* Right pane — to/cc/bcc */}
          <div style={{ width: '240px', flexShrink: 0, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            {renderRightSection('to')}
            {renderRightSection('cc')}
            {renderRightSection('bcc')}
          </div>
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', padding: '10px 16px', borderTop: '1px solid var(--color-border-subtle)', flexShrink: 0, gap: '8px' }}>
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', flex: 1 }}>
            클릭 시 <strong>{FIELD_LABELS[activeField]}</strong>에 추가
          </span>
          <button type="button" onClick={onClose}
            style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>
            취소
          </button>
          <button type="button"
            onClick={() => onConfirm({ to: Array.from(toList.values()), cc: Array.from(ccList.values()), bcc: Array.from(bccList.values()) })}
            style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}>
            확인
          </button>
        </div>
      </div>
    </div>
  );
}
