import { arrayBufferToBase64URL } from '../webpush';
import { request, apiGet, apiPost, apiPatch, apiDelete, responseErrorMessage } from './http';
import type { AuthTokenResponse, MFAVerifyResponse } from './types';

export type { AuthTokenResponse, MFAVerifyResponse };

export interface MFAStatus {
  enrolled: boolean;
  enabled: boolean;
  recovery_codes_remaining?: number;
}

export interface MFASetupResponse {
  secret: string;
  qr_uri: string;
  qr_image: string;
  recovery_codes: string[];
}

// Note: loginUser, verifyMFA, and MFA setup/disable calls below use raw fetch
// because they run before a session token exists and must target /api/auth/*
// (not /api/mail/*), so the request() helper cannot be used here.
export async function loginUser(
  email: string,
  password: string
): Promise<AuthTokenResponse> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, 'Sign-in failed.'));
  }

  return res.json() as Promise<AuthTokenResponse>;
}

export async function getMFAStatus(): Promise<MFAStatus> {
  const res = await apiGet<{ mfa_status: MFAStatus }>('auth/mfa/status');
  return res.mfa_status;
}

export async function startMFASetup(issuer?: string, email?: string): Promise<MFASetupResponse> {
  return apiPost<MFASetupResponse>('auth/mfa/setup', { issuer: issuer ?? 'GoGoMail', email });
}

export async function confirmMFASetup(code: string): Promise<void> {
  await apiPost<unknown>('auth/mfa/setup/confirm', { code });
}

export async function disableMFA(): Promise<void> {
  await apiDelete<unknown>('auth/mfa');
}

export async function verifyMFA(
  pendingToken: string,
  code: string,
): Promise<MFAVerifyResponse> {
  const res = await fetch('/api/auth/mfa', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pending_token: pendingToken, code }),
  });

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, 'MFA verification failed.'));
  }

  return res.json() as Promise<MFAVerifyResponse>;
}

export async function revokeAllSessions(): Promise<boolean> {
  try {
    const res = await fetch('/api/mail/auth/sessions/revoke-all', { method: 'POST' });
    return res.ok;
  } catch { return false; }
}

// ─── User profile + password ──────────────────────────────────────────────────

export interface UserProfile {
  user_id: string;
  display_name: string;
  email: string;
  recovery_email?: string;
  avatar_url?: string;
  quota_used: number;
  quota_limit: number | null;
}

export async function getUserProfile(): Promise<UserProfile | null> {
  try {
    const data = await apiGet<{ user?: UserProfile }>('me');
    return data.user ?? null;
  } catch { return null; }
}

export interface UserAddressEntry {
  id: string;
  address: string;
  is_primary: boolean;
}

export async function listUserAddresses(): Promise<UserAddressEntry[]> {
  try {
    const data = await apiGet<{ addresses?: UserAddressEntry[] }>('me/addresses');
    return data.addresses ?? [];
  } catch { return []; }
}

export async function updateUserProfile(fields: { display_name?: string; recovery_email?: string }): Promise<void> {
  await apiPatch('me', fields);
}

export async function uploadUserAvatar(file: File): Promise<string> {
  const form = new FormData();
  form.set('avatar', file);
  const res = await fetch('/api/mail/me/avatar', {
    method: 'PUT',
    body: form,
  });
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, 'Failed to upload profile photo.'));
  }
  const data = await res.json() as { avatar_url?: string };
  return data.avatar_url ?? '';
}

export async function deleteUserAvatar(): Promise<void> {
  await apiDelete('me/avatar');
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await apiPost('me/password', { current_password: currentPassword, new_password: newPassword });
}

// ─── Notification preferences ─────────────────────────────────────────────────

export interface NotificationTimeRange {
  start: string;
  end: string;
}

export interface NotificationDNDSchedule {
  weekdays: number[];
  time_ranges: NotificationTimeRange[];
  timezone: string;
}

export interface FolderNotificationOverride {
  enabled: boolean;
  dnd_inherit: boolean;
  dnd_schedule: NotificationDNDSchedule;
}

export interface ThreadNotificationOverride {
  enabled: boolean;
}

export interface NotificationPreferences {
  global_dnd_enabled: boolean;
  global_dnd_schedule: NotificationDNDSchedule;
  folder_overrides: Record<string, FolderNotificationOverride>;
  thread_overrides?: Record<string, ThreadNotificationOverride>;
  updated_at?: string;
}

export async function getNotificationPreferences(): Promise<NotificationPreferences> {
  return request<NotificationPreferences>('me/notification-preferences');
}

export async function setNotificationPreferences(prefs: NotificationPreferences): Promise<NotificationPreferences> {
  return request<NotificationPreferences>('me/notification-preferences', {
    method: 'PUT',
    body: JSON.stringify({
      global_dnd_enabled: prefs.global_dnd_enabled,
      global_dnd_schedule: prefs.global_dnd_schedule,
      folder_overrides: prefs.folder_overrides,
      thread_overrides: prefs.thread_overrides ?? {},
    }),
  });
}

// ─── User preferences + MCP settings ─────────────────────────────────────────

export interface WebmailPreferences {
  settings?: Record<string, unknown>;
  filter_rules?: unknown[];
  blocked_senders?: string[];
  allowed_senders?: string[];
  vacation?: Record<string, unknown>;
  signatures?: Record<string, string>;
  templates?: unknown[];
  mcp?: MCPSettings;
}

export type MCPPermissionMode = 'basic' | 'bypass';

export interface MCPMailSettings {
  send_enabled: boolean;
  confirm_external_recipients: boolean;
  confirm_attachments: boolean;
  daily_send_limit: number;
}

export type MCPSettingsScopes = Record<'mail' | 'contacts' | 'drive' | 'calendar', string[]>;

export interface MCPSettings {
  enabled: boolean;
  permission_mode: MCPPermissionMode;
  generated_mail_notice_enabled: boolean;
  generated_mail_notice_forced?: boolean;
  generated_mail_notice_text: string;
  require_confirmation_for_sensitive_actions: boolean;
  bypass_mode_allowed: boolean;
  mail: MCPMailSettings;
  scopes: MCPSettingsScopes;
}

export interface MCPAccessKey {
  id: string;
  user_id: string;
  domain_id: string;
  name: string;
  token_suffix: string;
  scopes: string[];
  permission_mode: MCPPermissionMode;
  allowed_cidrs?: string[];
  expires_at?: string;
  created_at: string;
  last_used_at?: string;
  revoked: boolean;
}

export interface CreateMCPAccessKeyRequest {
  name: string;
  scopes: string[];
  permission_mode: MCPPermissionMode;
  allowed_cidrs?: string[];
  expires_at?: string | null;
}

export async function getMCPSettings(): Promise<MCPSettings> {
  const data = await request<{ mcp: MCPSettings }>('me/mcp/settings');
  return data.mcp;
}

export async function setMCPSettings(settings: MCPSettings): Promise<MCPSettings> {
  const data = await request<{ mcp: MCPSettings }>('me/mcp/settings', {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
  return data.mcp;
}

export async function listMCPAccessKeys(): Promise<MCPAccessKey[]> {
  const data = await request<{ mcp_access_keys?: MCPAccessKey[] }>('me/mcp/access-keys');
  return data.mcp_access_keys ?? [];
}

export async function createMCPAccessKey(req: CreateMCPAccessKeyRequest): Promise<{ key: MCPAccessKey; token: string }> {
  const data = await request<{ mcp_access_key: MCPAccessKey; token: string }>('me/mcp/access-keys', {
    method: 'POST',
    body: JSON.stringify(req),
  });
  return { key: data.mcp_access_key, token: data.token };
}

export async function revokeMCPAccessKey(id: string): Promise<MCPAccessKey> {
  const data = await request<{ mcp_access_key: MCPAccessKey }>(`me/mcp/access-keys/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
  return data.mcp_access_key;
}

export async function getPreferences(): Promise<WebmailPreferences> {
  try {
    const res = await fetch('/api/mail/preferences');
    if (!res.ok) return {};
    const data = await res.json() as { preferences?: WebmailPreferences };
    return data.preferences ?? {};
  } catch { return {}; }
}

function mergePreferences(current: WebmailPreferences, next: WebmailPreferences): WebmailPreferences {
  const merged: WebmailPreferences = { ...current, ...next };
  if (current.settings || next.settings) {
    merged.settings = { ...(current.settings ?? {}), ...(next.settings ?? {}) };
  }
  if (current.signatures || next.signatures) {
    merged.signatures = { ...(current.signatures ?? {}), ...(next.signatures ?? {}) };
  }
  return merged;
}

export async function setPreferences(prefs: WebmailPreferences): Promise<WebmailPreferences> {
  const current = await getPreferences();
  const merged = mergePreferences(current, prefs);
  try {
    const res = await fetch('/api/mail/preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(merged),
    });
    if (!res.ok) return merged;
    const data = await res.json() as { preferences?: WebmailPreferences };
    return data.preferences ?? merged;
  } catch { return merged; }
}

// ─── WebPush device registration ───────────────────────────────────────────────

export async function registerWebPushDevice(subscription: PushSubscription): Promise<void> {
  const key = subscription.getKey('p256dh');
  const auth = subscription.getKey('auth');
  const token = JSON.stringify({
    endpoint: subscription.endpoint,
    keys: {
      p256dh: arrayBufferToBase64URL(key),
      auth: arrayBufferToBase64URL(auth),
    },
  });
  await request<unknown>('push-devices', {
    method: 'POST',
    body: JSON.stringify({ platform: 'webpush', token, label: 'browser' }),
  });
}
