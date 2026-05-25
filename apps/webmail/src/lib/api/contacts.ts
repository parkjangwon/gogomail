import { request } from './http';

export interface AddressBook {
  ID: string;
  Name: string;
  Description: string;
  UserID: string;
}

export interface ContactObject {
  ID: string;
  AddressBookID: string;
  ObjectName: string;
  UID: string;
  VCard: string; // base64-encoded vCard bytes
  CreatedAt: string;
  UpdatedAt: string;
}

export interface UpsertContactFields {
  fn: string;
  email?: string;
  tel?: string;
  org?: string;
  title?: string;
  note?: string;
}

export interface DirectoryUser {
  id: string;
  display_name: string;
  email: string;
  avatar_url?: string;
  org_unit_name?: string;
}

export interface OrgMember {
  id: string;
  display_name: string;
  email: string;
  avatar_url?: string;
}

export interface OrgUnit {
  id: string;
  display_name: string;
  parent_id?: string;
  depth: number;
  members: OrgMember[];
}

export interface DirectoryProfile {
  found: boolean;
  display_name?: string;
  org_unit_name?: string;
  title?: string;
}

/** Parse vCard fields from base64-encoded vCard data. */
export function parseVCard(base64VCard: string): {
  fn: string; email: string; tel: string; org: string; note: string; title: string;
} {
  let text = '';
  try {
    const binary = atob(base64VCard);
    if (typeof TextDecoder !== 'undefined') {
      text = new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0)));
    } else {
      text = decodeURIComponent(escape(binary));
    }
  } catch {
    text = base64VCard;
  }
  const unfolded = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');
  const get = (prop: string) => {
    const prefix = `${prop}`.toUpperCase();
    for (const line of unfolded.split(/\r?\n/)) {
      const normalized = line.toUpperCase();
      if (normalized.startsWith(`${prefix}:`) || normalized.startsWith(`${prefix};`)) {
        const valueIndex = line.indexOf(':');
        if (valueIndex >= 0) return line.slice(valueIndex + 1).trim();
      }
    }
    return '';
  };
  return {
    fn: get('FN'),
    email: get('EMAIL'),
    tel: get('TEL'),
    org: get('ORG'),
    title: get('TITLE'),
    note: get('NOTE'),
  };
}

export async function listAddressBooks(): Promise<AddressBook[]> {
  try {
    const data = await request<{ address_books?: AddressBook[] }>('addressbooks');
    return data.address_books ?? [];
  } catch { return []; }
}

export async function createAddressBook(name: string, description = ''): Promise<AddressBook> {
  const data = await request<{ address_book: AddressBook }>('addressbooks', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  });
  return data.address_book;
}

export async function renameAddressBook(id: string, name: string): Promise<AddressBook> {
  const data = await request<{ address_book: AddressBook }>(`addressbooks/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  });
  return data.address_book;
}

export async function deleteAddressBook(id: string): Promise<void> {
  await request<void>(`addressbooks/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function listContacts(addressBookId: string): Promise<ContactObject[]> {
  try {
    const data = await request<{ contacts?: ContactObject[] }>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts`);
    return data.contacts ?? [];
  } catch { return []; }
}

export async function deleteContact(addressBookId: string, objectName: string): Promise<void> {
  await request<void>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts/${encodeURIComponent(objectName)}`, { method: 'DELETE' });
}

function vcardEscape(s: string): string { return s.replace(/\\/g, '\\\\').replace(/,/g, '\\,').replace(/;/g, '\\;').replace(/\n/g, '\\n'); }

export async function upsertContact(addressBookId: string, objectName: string, fields: UpsertContactFields): Promise<ContactObject> {
  const lines = ['BEGIN:VCARD', 'VERSION:3.0', `FN:${vcardEscape(fields.fn)}`];
  const nameParts = fields.fn.trim().split(/\s+/);
  const last = nameParts.length > 1 ? nameParts[nameParts.length - 1] : '';
  const first = nameParts.length > 1 ? nameParts.slice(0, -1).join(' ') : nameParts[0];
  lines.push(`N:${vcardEscape(last)};${vcardEscape(first)};;;`);
  if (fields.email) lines.push(`EMAIL:${vcardEscape(fields.email)}`);
  if (fields.tel) lines.push(`TEL:${vcardEscape(fields.tel)}`);
  if (fields.org) lines.push(`ORG:${vcardEscape(fields.org)}`);
  if (fields.title) lines.push(`TITLE:${vcardEscape(fields.title)}`);
  if (fields.note) lines.push(`NOTE:${vcardEscape(fields.note)}`);
  lines.push('END:VCARD');
  const vcard = lines.join('\r\n');
  const data = await request<{ contact: ContactObject }>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts/${encodeURIComponent(objectName)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/vcard' },
    body: vcard,
  });
  return data.contact;
}

export async function listDirectoryUsers(q?: string, limit = 50): Promise<DirectoryUser[]> {
  try {
    const params = new URLSearchParams();
    if (q) params.set('q', q);
    params.set('limit', String(limit));
    const res = await fetch(`/api/mail/directory/users?${params}`);
    if (!res.ok) return [];
    const data = await res.json() as { users?: DirectoryUser[] };
    return data.users ?? [];
  } catch { return []; }
}

export async function listOrgTree(): Promise<OrgUnit[]> {
  try {
    const res = await fetch('/api/mail/directory/org-tree');
    if (!res.ok) return [];
    const data = await res.json() as { units?: OrgUnit[] };
    return data.units ?? [];
  } catch { return []; }
}

export async function getDirectoryProfile(email: string): Promise<DirectoryProfile | null> {
  try {
    const params = new URLSearchParams({ email });
    const res = await fetch(`/api/mail/directory/profile?${params}`);
    if (!res.ok) return null;
    return await res.json() as DirectoryProfile;
  } catch { return null; }
}

// ─── Contact autocomplete (used by compose) ───────────────────────────────────

export interface ContactSuggestion {
  type?: string;
  display_name: string;
  email: string;
  organization?: string;
}

export async function autocompleteContacts(q: string, limit = 8): Promise<ContactSuggestion[]> {
  if (!q.trim()) return [];
  const params = new URLSearchParams({ q, limit: String(limit) });
  try {
    const res = await fetch(`/api/mail/contacts/autocomplete?${params}`);
    if (!res.ok) return [];
    const data = await res.json() as { results?: ContactSuggestion[] };
    return data.results ?? [];
  } catch { return []; }
}
