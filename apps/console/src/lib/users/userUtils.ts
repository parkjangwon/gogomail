export interface User {
  id: string;
  domain_id: string;
  username: string;
  display_name: string;
  recovery_email?: string;
  role: string;
  status: UserStatus;
  password_configured: boolean;
  must_change_password: boolean;
  quota_used: number;
  quota_limit: number;
  created_at: string;
}

export interface Domain {
  id: string;
  name: string;
  status: string;
}

export type UserStatus = 'active' | 'suspended' | 'disabled';

export const STATUS_COLORS: Record<UserStatus, 'green' | 'red' | 'grey' | 'blue'> = {
  active: 'green',
  suspended: 'red',
  disabled: 'red',
};

export function normalizeUserStatus(rawStatus: unknown): UserStatus {
  switch (String(rawStatus).trim().toLowerCase()) {
    case 'active':
      return 'active';
    case 'suspended':
      return 'suspended';
    case 'disabled':
      return 'disabled';
    default:
      return 'disabled';
  }
}

export function formatQuotaUsage(used: number, limit: number): string {
  const percentage = limit > 0 ? Math.round((used / limit) * 100) : 0;
  return `${percentage}%`;
}

export function getQuotaStatus(used: number, limit: number): 'healthy' | 'warning' | 'critical' {
  if (limit === 0) return 'healthy';
  const percentage = (used / limit) * 100;
  if (percentage >= 90) return 'critical';
  if (percentage >= 75) return 'warning';
  return 'healthy';
}
