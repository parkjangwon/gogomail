export const WEBMAIL_TEMPLATES_KEY = 'webmail_templates';

export interface StoredEmailTemplate {
  id: string;
  name: string;
  subject: string;
  body: string;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function templateId(): string {
  const cryptoObj = globalThis.crypto;
  if (cryptoObj?.randomUUID) return `template-${cryptoObj.randomUUID()}`;
  if (cryptoObj?.getRandomValues) {
    const bytes = new Uint32Array(2);
    cryptoObj.getRandomValues(bytes);
    return `template-${bytes[0].toString(36)}${bytes[1].toString(36)}`;
  }
  const value = `${Date.now().toString(36)}${performance.now().toString(36).replace('.', '')}`;
  return `template-${value}`;
}

export function normalizeEmailTemplates(value: unknown): StoredEmailTemplate[] {
  if (!Array.isArray(value)) return [];
  const templates: StoredEmailTemplate[] = [];
  const seenNames = new Set<string>();

  for (const item of value) {
    if (!item || typeof item !== 'object') continue;
    const record = item as Record<string, unknown>;
    const name = stringValue(record.name).trim();
    if (!name) continue;

    const normalizedName = name.toLowerCase();
    if (seenNames.has(normalizedName)) continue;
    seenNames.add(normalizedName);

    const id = stringValue(record.id).trim() || templateId();
    templates.push({
      id,
      name,
      subject: stringValue(record.subject).trim(),
      body: stringValue(record.body),
    });
  }

  return templates;
}

export function loadLocalEmailTemplates(): StoredEmailTemplate[] {
  try {
    return normalizeEmailTemplates(localStorage.getItem(WEBMAIL_TEMPLATES_KEY) ? JSON.parse(localStorage.getItem(WEBMAIL_TEMPLATES_KEY) ?? '[]') : []);
  } catch {
    return [];
  }
}

export function saveLocalEmailTemplates(templates: StoredEmailTemplate[]): void {
  try {
    localStorage.setItem(WEBMAIL_TEMPLATES_KEY, JSON.stringify(normalizeEmailTemplates(templates)));
  } catch {
    // Best-effort cache for offline compose/search. Server preferences remain canonical.
  }
}
