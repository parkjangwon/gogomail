'use client';

import { useState, useEffect, useRef } from 'react';
import { listOrgTree, listAddressBooks, listContacts, parseVCard, OrgUnit, AddressBook, getUserProfile } from '@/lib/api';

export interface PickerItem {
  id: string;
  display_name: string;
  email: string;
  kind?: 'user' | 'org' | 'addressbook';
  include_children?: boolean;
  count?: number;
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
        const kind = email.startsWith('org:') ? 'org' : email.startsWith('addressbook:') ? 'addressbook' : 'user';
        return { id: email, display_name: name || email, email, kind };
      }
      const kind = p.startsWith('org:') ? 'org' : p.startsWith('addressbook:') ? 'addressbook' : 'user';
      return { id: p, display_name: p, email: p, kind };
    });
}

export function pickerItemsToString(items: PickerItem[]): string {
  return items
    .map((i) => (i.display_name && i.display_name !== i.email ? `${i.display_name} <${i.email}>` : i.email))
    .join(', ');
}

// ── Hierarchical tree renderer ─────────────────────────────────────────────────

interface RenderOrgTreeProps {
  units: OrgUnit[];
  getChildren: (parentId: string) => OrgUnit[];
  selectedOrg: OrgUnit | null;
  expandedIds: Set<string>;
  onToggleExpanded: (id: string) => void;
  onSelectOrg: (unit: OrgUnit) => void;
  rowHover: Record<string, (e: React.MouseEvent<HTMLElement>) => void>;
  depth?: number;
}

function RenderOrgTree({
  units,
  getChildren,
  selectedOrg,
  expandedIds,
  onToggleExpanded,
  onSelectOrg,
  rowHover,
  depth = 0,
}: RenderOrgTreeProps) {
  return (
    <>
      {units.map((unit) => {
        const children = getChildren(unit.id);
        const isExpanded = expandedIds.has(unit.id);
        const isSelected = selectedOrg?.id === unit.id;

        const fontSize = depth === 0 ? 13 : depth === 1 ? 12 : 11;
        const fontWeight = depth === 0 ? 600 : depth === 1 ? 500 : 400;
        const textColor = depth === 0 ? 'var(--color-text-primary)' : 'var(--color-text-secondary)';
        const bgColor = !isSelected ? (
          depth === 0 ? 'transparent' :
          depth === 1 ? 'var(--color-bg-secondary)' :
          'var(--color-bg-tertiary)'
        ) : 'var(--color-accent-subtle)';

        return (
          <div key={unit.id}>
            <div
              onClick={() => {
                if (children.length > 0) onToggleExpanded(unit.id);
                onSelectOrg(unit);
              }}
              style={{
                display: 'flex', alignItems: 'center', gap: '4px',
                paddingTop: '8px', paddingBottom: '8px',
                paddingLeft: `${12 + depth * 24}px`, paddingRight: '12px',
                cursor: 'pointer',
                borderLeft: isSelected ? '3px solid var(--color-accent)' : '3px solid transparent',
                background: bgColor,
                fontWeight,
              }}
              {...(!isSelected ? rowHover : {})}
            >
              {children.length > 0 ? (
                <span
                  onClick={(e) => { e.stopPropagation(); onToggleExpanded(unit.id); }}
                  style={{
                    fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px',
                    width: '12px', textAlign: 'center', cursor: 'pointer',
                  }}>
                  {isExpanded ? '▼' : '▶'}
                </span>
              ) : (
                <span style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px', width: '12px', textAlign: 'center' }}>
                  {depth === 0 ? '▸' : '└'}
                </span>
              )}
              <span style={{ fontSize, color: isSelected ? 'var(--color-accent)' : textColor, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {unit.display_name}
              </span>
              <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{unit.members.length}</span>
            </div>

            {/* Children */}
            {isExpanded && children.length > 0 && (
              <RenderOrgTree
                units={children}
                getChildren={getChildren}
                selectedOrg={selectedOrg}
                expandedIds={expandedIds}
                onToggleExpanded={onToggleExpanded}
                onSelectOrg={onSelectOrg}
                rowHover={rowHover}
                depth={depth + 1}
              />
            )}
          </div>
        );
      })}
    </>
  );
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
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // Address book
  const [addressBooks, setAddressBooks] = useState<AddressBook[]>([]);
  const [selectedBook, setSelectedBook] = useState<AddressBook | null>(null);
  const [bookContacts, setBookContacts] = useState<PickerItem[]>([]);
  const [booksLoading, setBooksLoading] = useState(false);
  const [bookLoading, setBookLoading] = useState(false);
  const [contactsSearch, setContactsSearch] = useState('');
  const [includeChildOrgs, setIncludeChildOrgs] = useState(true);

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
    Promise.all([getUserProfile(), listOrgTree()])
      .then(([userProfile, units]) => {
        setOrgTree(units);

        // Find user's organization
        let userOrgId: string | null = null;
        if (userProfile) {
          for (const unit of units) {
            const member = unit.members.find((m) => m.id === userProfile.user_id);
            if (member) {
              userOrgId = unit.id;
              break;
            }
          }
        }

        // Build parent chain from user's org to root
        const toExpand = new Set<string>();
        if (userOrgId) {
          const userOrg = units.find((u) => u.id === userOrgId) ?? null;
          let current = userOrg;
          while (current && current.parent_id) {
            const parent = units.find((u) => u.id === current!.parent_id);
            if (parent) {
              toExpand.add(parent.id);
              current = parent;
            } else {
              break;
            }
          }
          if (userOrg && units.some((u) => u.parent_id === userOrg.id)) {
            toExpand.add(userOrg.id);
          }
          setSelectedOrg(userOrg);
        } else {
          units.filter((u) => !u.parent_id).forEach((u) => toExpand.add(u.id));
          setSelectedOrg(units.find((u) => !u.parent_id) ?? null);
        }

        setExpandedIds(toExpand);
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

  // ── Org tree helpers ──────────────────────────────────────────────────────

  const getChildrenOf = (parentId: string | undefined): OrgUnit[] => {
    return orgTree.filter((u) => u.parent_id === parentId);
  };

  const getRootOrgs = (): OrgUnit[] => {
    return orgTree.filter((u) => !u.parent_id);
  };

  const descendantOrgs = (orgId: string): OrgUnit[] => {
    const children = getChildrenOf(orgId);
    return children.flatMap((child) => [child, ...descendantOrgs(child.id)]);
  };

  const orgMemberCount = (unit: OrgUnit, includeChildren: boolean): number => {
    if (!includeChildren) return unit.members.length;
    return unit.members.length + descendantOrgs(unit.id).reduce((sum, child) => sum + child.members.length, 0);
  };

  const orgToken = (unit: OrgUnit): PickerItem => {
    const token = `org:${unit.id}${includeChildOrgs ? ':children' : ''}`;
    const suffix = includeChildOrgs ? ' + 하위 조직' : '';
    return {
      id: token,
      display_name: `[조직] ${unit.display_name}${suffix}`,
      email: token,
      kind: 'org',
      include_children: includeChildOrgs,
      count: orgMemberCount(unit, includeChildOrgs),
    };
  };

  const addressBookToken = (book: AddressBook): PickerItem => ({
    id: `addressbook:${book.ID}`,
    display_name: `[주소록] ${book.Name}`,
    email: `addressbook:${book.ID}`,
    kind: 'addressbook',
    count: bookContacts.length,
  });

  const toggleExpanded = (id: string) => {
    const next = new Set(expandedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setExpandedIds(next);
  };

  // ── Org search filtering ──────────────────────────────────────────────────────

  const q = orgSearch.trim().toLowerCase();

  const matchesSearch = (unit: OrgUnit): boolean => {
    if (!q) return true;
    return (
      unit.display_name.toLowerCase().includes(q) ||
      unit.members.some(
        (m) =>
          (m.display_name || '').toLowerCase().includes(q) ||
          m.email.toLowerCase().includes(q)
      )
    );
  };

  // ── Contacts search filtering ──────────────────────────────────────────────

  const cq = contactsSearch.trim().toLowerCase();
  const filteredContacts: PickerItem[] = cq
    ? bookContacts.filter(
        (c) =>
          c.display_name.toLowerCase().includes(cq) ||
          c.email.toLowerCase().includes(cq)
      )
    : bookContacts;

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
          display_name: `[하위 조직] ${child.display_name}${includeChildOrgs ? ' + 하위 조직' : ''}`,
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

            {/* Contacts tab: search input */}
            {tab === 'contacts' && (
              <div style={{ padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
                <input
                  type="text" value={contactsSearch} onChange={(e) => setContactsSearch(e.target.value)}
                  placeholder="이름, 이메일 검색"
                  style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', outline: 'none', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
                />
              </div>
            )}

            <div style={{ flex: 1, overflowY: 'auto' }}>
              {/* Org tab tree */}
              {tab === 'org' && treeLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>불러오는 중...</div>
              )}
              {tab === 'org' && !treeLoading && orgTree.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>조직 정보 없음</div>
              )}
              {tab === 'org' && !treeLoading && (q ? orgTree.filter(matchesSearch).length === 0 : getRootOrgs().length === 0) && q && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>결과 없음</div>
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
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>왼쪽에서 조직을 선택하세요</div>
            )}
            {tab === 'org' && !treeLoading && !q && selectedOrg && middleItems.length === 0 && (
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
            {tab === 'contacts' && !bookLoading && selectedBook && bookContacts.length > 0 && filteredContacts.length === 0 && (
              <div style={{ padding: '40px 20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>결과 없음</div>
            )}

            {tab === 'org' && selectedOrg && (
              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', fontSize: '12px', color: 'var(--color-text-secondary)', userSelect: 'none' }}>
                <input
                  type="checkbox"
                  checked={includeChildOrgs}
                  onChange={(e) => setIncludeChildOrgs(e.target.checked)}
                />
                조직 선택 시 하위 조직 포함
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
                }}>{item.kind === 'org' ? '조' : item.kind === 'addressbook' ? '록' : (item.display_name || item.email)[0].toUpperCase()}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {item.display_name}
                  </div>
                  {item.kind === 'org' || item.kind === 'addressbook' ? (
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {item.count ?? 0}명으로 발송 시 확장
                    </div>
                  ) : item.display_name !== item.email && (
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {item.email}
                    </div>
                  )}
                </div>
                <button type="button" onClick={() => addToActive(item)}
                  title={`${FIELD_LABELS[activeField]}에 추가`}
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
