export interface DomainDetail {
  id: string;
  company_id: string;
  company_name: string;
  name: string;
  name_ace: string;
  status: string;
  last_dns_check_status: string;
  last_dns_checked_at?: string;
  quota_used: number;
  quota_limit: number;
  quota_remaining: number;
  allocated_user_quota: number;
  allocatable_user_quota: number;
  over_allocated: boolean;
  created_at: string;
}

export interface User {
  id: string;
  username: string;
  display_name: string;
  status: string;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

export interface DomainSetting {
  ID: string;
  Key: string;
  Value: unknown;
  Locked: boolean;
  Version: number;
  UpdatedAt: string;
}

export interface DailyCount {
  date: string;
  label: string;
  total: number;
  success: number;
  failed: number;
}

export interface DomainMCPPolicy {
  enabled: boolean;
  allow_user_access_keys: boolean;
  allow_bypass_mode: boolean;
  force_generated_mail_notice: boolean;
  audit_level: string;
  allowed_scopes: string[];
  external_recipient_confirmation?: string;
  public_drive_share_confirmation?: string;
  [key: string]: unknown;
}

export interface DomainMCPPolicyConfig {
  Locked?: boolean;
  Version?: number;
  UpdatedAt?: string;
}

export const DEFAULT_MCP_SCOPES = [
  'mail:read',
  'mail:write',
  'mail:send',
  'mail:manage',
  'contacts:read',
  'contacts:write',
  'contacts:manage',
  'drive:read',
  'drive:write',
  'drive:manage',
  'calendar:read',
  'calendar:write',
  'calendar:manage',
];

export const DEFAULT_MCP_POLICY: DomainMCPPolicy = {
  enabled: false,
  allow_user_access_keys: false,
  allow_bypass_mode: false,
  force_generated_mail_notice: false,
  audit_level: 'full',
  allowed_scopes: [],
  external_recipient_confirmation: 'basic',
  public_drive_share_confirmation: 'basic',
};

export function normalizeMCPPolicy(policy?: Partial<DomainMCPPolicy> | null): DomainMCPPolicy {
  return {
    ...DEFAULT_MCP_POLICY,
    ...(policy ?? {}),
    enabled: policy?.enabled ?? DEFAULT_MCP_POLICY.enabled,
    allow_user_access_keys: policy?.allow_user_access_keys ?? DEFAULT_MCP_POLICY.allow_user_access_keys,
    allow_bypass_mode: policy?.allow_bypass_mode ?? DEFAULT_MCP_POLICY.allow_bypass_mode,
    force_generated_mail_notice: policy?.force_generated_mail_notice ?? DEFAULT_MCP_POLICY.force_generated_mail_notice,
    audit_level: policy?.audit_level ?? DEFAULT_MCP_POLICY.audit_level,
    allowed_scopes: Array.isArray(policy?.allowed_scopes) ? policy.allowed_scopes : DEFAULT_MCP_POLICY.allowed_scopes,
  };
}
