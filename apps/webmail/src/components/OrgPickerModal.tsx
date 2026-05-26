'use client';

import { useState, useRef } from 'react';
import { useTranslations } from 'next-intl';
import { OrgUnit } from '@/lib/api';
import type { PickerItem } from '@/lib/mail-address';
import { RenderOrgTree } from './OrgPickerTree';
import { useOrgTree } from './org-picker/useOrgTree';
import { useAddressBook } from './org-picker/useAddressBook';
import { useRecipients } from './org-picker/useRecipients';

interface OrgPickerModalProps {
  initialTo?: PickerItem[];
  initialCc?: PickerItem[];
  initialBcc?: PickerItem[];
  onClose: () => void;
  onConfirm: (result: { to: PickerItem[]; cc: PickerItem[]; bcc: PickerItem[] }) => void;
}

// ── Component ──────────────────────────────────────────────────────────────────

export function OrgPickerModal({
  initialTo = [],
  initialCc = [],
  initialBcc = [],
  onClose,
  onConfirm,
}: OrgPickerModalProps) {
  const tr = useTranslations('modals.orgPicker');
  const FIELD_LABELS = { to: tr('fields.to'), cc: tr('fields.cc'), bcc: tr('fields.bcc') } as const;
  const [tab, setTab] = useState<'org' | 'contacts'>('org');
  const [includeChildOrgs, setIncludeChildOrgs] = useState(true);

  const {
    orgTree,
    selectedOrg,
    setSelectedOrg,
    treeLoading,
    orgSearch,
    setOrgSearch,
    expandedIds,
    toggleExpanded,
    getChildrenOf,
    getRootOrgs,
    orgMemberCount,
    matchesSearch,
    q,
  } = useOrgTree();

  const {
    addressBooks,
    selectedBook,
    setSelectedBook,
    bookContacts,
    booksLoading,
    bookLoading,
    contactsSearch,
    setContactsSearch,
    filteredContacts,
    addressBookToken,
  } = useAddressBook(tab);

  const {
    toList,
    ccList,
    bccList,
    activeField,
    setActiveField,
    addToActive,
    removeFromList,
    clearList,
  } = useRecipients(initialTo, initialCc, initialBcc);

  const orgSearchRef = useRef<HTMLInputElement>(null);

  const orgToken = (unit: OrgUnit): PickerItem => {
    const token = `org:${unit.id}${includeChildOrgs ? ':children' : ''}`;
    return {
      id: token,
      display_name: unit.display_name,
      email: token,
      kind: 'org',
      include_children: includeChildOrgs,
      count: orgMemberCount(unit, includeChildOrgs),
    };
  };

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

  // Middle pane items
  const middleItems: PickerItem[] = (() => {
    if (tab === 'contacts') {
      const rows = selectedBook && !contactsSearch.trim() ? [addressBookToken(selectedBook)] : [];
      return [...rows, ...filteredContacts.map((item) => ({ ...item, kind: 'user' as const }))];
    }
    if (q) {
      // All matching members across all orgs (deduped by id)
      const seen = new Set<string>();
      const matchedOrgs = orgTree.filter(matchesSearch).map((unit) => orgToken(unit));
      const matchedMembers = orgTree
        .flatMap((u) => u.members)
        .filter((m) => {
          if (seen.has(m.id)) return false;
          const match =
            (m.display_name || '').toLowerCase().includes(q) || m.email.toLowerCase().includes(q);
          if (match) seen.add(m.id);
          return match;
        })
        .map((m) => ({ id: m.id, display_name: m.display_name || m.email, email: m.email, kind: 'user' as const }));
      return [...matchedOrgs, ...matchedMembers];
    }
    const rows: PickerItem[] = [];
    if (selectedOrg) {
      rows.push(orgToken(selectedOrg));
      for (const child of getChildrenOf(selectedOrg.id)) {
        rows.push({
          id: `org:${child.id}${includeChildOrgs ? ':children' : ''}`,
          display_name: child.display_name,
          email: `org:${child.id}${includeChildOrgs ? ':children' : ''}`,
          kind: 'org',
          include_children: includeChildOrgs,
          count: orgMemberCount(child, includeChildOrgs),
        });
      }
    }
    return [
      ...rows,
      ...(selectedOrg?.members ?? []).map((m) => ({
        id: m.id,
        display_name: m.display_name || m.email,
        email: m.email,
        kind: 'user' as const,
      })),
    ];
  })();

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
              {tr('clearAll')}
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
              {t === 'org' ? tr('tabOrg') : tr('tabContacts')}
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
                  placeholder={tr('orgSearchPlaceholder')}
                  style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', outline: 'none', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
                />
              </div>
            )}

            {/* Contacts tab: search input */}
            {tab === 'contacts' && (
              <div style={{ padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
                <input
                  type="text" value={contactsSearch} onChange={(e) => setContactsSearch(e.target.value)}
                  placeholder={tr('contactsSearchPlaceholder')}
                  style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', outline: 'none', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
                />
              </div>
            )}

            <div style={{ flex: 1, overflowY: 'auto' }}>
              {/* Org tab tree */}
              {tab === 'org' && treeLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('loading')}</div>
              )}
              {tab === 'org' && !treeLoading && orgTree.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noOrg')}</div>
              )}
              {tab === 'org' && !treeLoading && (q ? orgTree.filter(matchesSearch).length === 0 : getRootOrgs().length === 0) && q && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noResults')}</div>
              )}

              {/* Recursive tree renderer */}
              {tab === 'org' && !treeLoading && (
                <div>
                  {q ? (
                    // Search mode: flat list
                    orgTree.filter(matchesSearch).map((unit) => {
                      const isSelected = selectedOrg?.id === unit.id;
                      const depthIndicator = unit.depth === 0 ? '▸' : unit.depth === 1 ? '└' : '  └';
                      const fontSize = unit.depth === 0 ? 13 : unit.depth === 1 ? 12 : 11;
                      const fontWeight = unit.depth === 0 ? 600 : unit.depth === 1 ? 500 : 400;
                      const textColor = unit.depth === 0 ? 'var(--color-text-primary)' : 'var(--color-text-secondary)';
                      const bgColor = !isSelected ? (
                        unit.depth === 0 ? 'transparent' :
                        unit.depth === 1 ? 'var(--color-bg-secondary)' :
                        'var(--color-bg-tertiary)'
                      ) : 'var(--color-accent-subtle)';

                      return (
                        <div key={unit.id}
                          onClick={() => { setOrgSearch(''); setSelectedOrg(unit); }}
                          style={{
                            display: 'flex', alignItems: 'center', gap: '4px',
                            paddingTop: '8px', paddingBottom: '8px',
                            paddingLeft: `${12 + unit.depth * 16}px`, paddingRight: '12px',
                            cursor: 'pointer',
                            borderLeft: isSelected ? '3px solid var(--color-accent)' : '3px solid transparent',
                            background: bgColor,
                            fontWeight,
                          }}
                          {...(!isSelected ? rowHover : {})}
                        >
                          <span style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px', width: '12px', textAlign: 'center' }}>
                            {depthIndicator}
                          </span>
                          <span style={{ fontSize, color: isSelected ? 'var(--color-accent)' : textColor, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {unit.display_name}
                          </span>
                          <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{unit.members.length}</span>
                        </div>
                      );
                    })
                  ) : (
                    // Normal mode: hierarchical tree
                    <RenderOrgTree
                      units={getRootOrgs()}
                      getChildren={getChildrenOf}
                      selectedOrg={selectedOrg}
                      expandedIds={expandedIds}
                      onToggleExpanded={toggleExpanded}
                      onSelectOrg={(unit) => { setOrgSearch(''); setSelectedOrg(unit); }}
                      rowHover={rowHover}
                    />
                  )}
                </div>
              )}

              {/* Contacts tab address books */}
              {tab === 'contacts' && booksLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('loading')}</div>
              )}
              {tab === 'contacts' && !booksLoading && addressBooks.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noAddressBooks')}</div>
              )}
              {tab === 'contacts' && addressBooks.map((book) => {
                const isSelected = selectedBook?.ID === book.ID;
                return (
                  <div key={book.ID}
                    onClick={() => setSelectedBook(book)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '4px',
                      paddingTop: '8px', paddingBottom: '8px',
                      paddingLeft: '12px', paddingRight: '12px',
                      cursor: 'pointer',
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
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('selectOrgHint')}</div>
            )}
            {tab === 'org' && !treeLoading && !q && selectedOrg && middleItems.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noMembers')}</div>
            )}
            {tab === 'org' && q && middleItems.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noResults')}</div>
            )}
            {tab === 'contacts' && bookLoading && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('loading')}</div>
            )}
            {tab === 'contacts' && !bookLoading && !selectedBook && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('selectBookHint')}</div>
            )}
            {tab === 'contacts' && !bookLoading && selectedBook && bookContacts.length === 0 && contactsSearch.trim() && filteredContacts.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noContacts')}</div>
            )}
            {tab === 'contacts' && !bookLoading && selectedBook && bookContacts.length > 0 && filteredContacts.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{tr('noResults')}</div>
            )}

            {tab === 'org' && selectedOrg && (
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', fontSize: '12px', color: 'var(--color-text-secondary)', userSelect: 'none' }}>
                <input
                  type="checkbox"
                  checked={includeChildOrgs}
                  onChange={(e) => setIncludeChildOrgs(e.target.checked)}
                />
                {tr('includeChildren')}
              </label>
            )}

            {/* Item rows */}
            {middleItems.map((item) => (
              <div key={item.id}
                style={{ display: 'flex', alignItems: 'center', gap: '10px', paddingTop: '8px', paddingBottom: '8px', paddingLeft: '12px', paddingRight: '12px', cursor: 'default' }}
                {...rowHover}
              >
                <div style={{
                  ...avatarStyle,
                  borderRadius: item.kind === 'user' || !item.kind ? '50%' : '8px',
                  background: item.kind === 'org' ? 'var(--color-accent-subtle)' : item.kind === 'addressbook' ? 'var(--color-bg-tertiary)' : avatarStyle.background,
                }}>{item.kind === 'org' ? tr('orgBadge') : item.kind === 'addressbook' ? tr('bookBadge') : (item.display_name || item.email)[0].toUpperCase()}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {item.display_name}
                  </div>
                  {item.kind === 'org' || item.kind === 'addressbook' ? (
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {tr('expandTo', { count: item.count ?? 0 })}
                    </div>
                  ) : item.display_name !== item.email && (
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {item.email}
                    </div>
                  )}
                </div>
                <button type="button" onClick={() => addToActive(item)}
                  title={tr('addTooltip', { field: FIELD_LABELS[activeField] })}
                  style={{ padding: '6px 12px', fontSize: '12px', color: 'var(--color-accent)', border: '1px solid var(--color-accent)', borderRadius: '4px', background: 'transparent', cursor: 'pointer', flexShrink: 0, fontWeight: 500 }}>
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
            {tr('footerHint', { field: FIELD_LABELS[activeField] })}
          </span>
          <button type="button" onClick={onClose}
            style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>
            {tr('cancel')}
          </button>
          <button type="button"
            onClick={() => onConfirm({ to: Array.from(toList.values()), cc: Array.from(ccList.values()), bcc: Array.from(bccList.values()) })}
            style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}>
            {tr('confirm')}
          </button>
        </div>
      </div>
    </div>
  );
}
