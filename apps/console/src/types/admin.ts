// Mirror of Go backend models from internal/admin/models.go
// These are manually maintained — prefer using OpenAPI-generated types from @gogomail/api-types

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role_id: string;
  company_id: string;
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: string;
  company_id: string;
  name: string;
  permissions: Permission[];
  created_at: string;
  updated_at: string;
}

export interface Permission {
  id: string;
  resource: string;
  action: string;
  scope: string;
}

export interface Company {
  id: string;
  name: string;
  email: string;
  website?: string;
  logo_url?: string;
  created_at: string;
  updated_at: string;
}

export interface Domain {
  id: string;
  company_id: string;
  name: string;
  verified: boolean;
  verification_token?: string;
  created_at: string;
  updated_at: string;
}

export interface AuditLog {
  id: string;
  company_id: string;
  admin_user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  ip_address: string;
  timestamp: string;
}

export interface AuditPolicyConfig {
  company_id: string;
  audit_level: "level_1" | "level_2" | "level_3";
  audit_admin_actions: boolean;
  audit_security_events: boolean;
  retention_days: number;
  mask_mail_content: boolean;
  mask_recipient_emails: boolean;
}

export interface StatisticsData {
  total_users: number;
  active_sessions: number;
  mail_operations: number;
  audit_logs_24h: number;
  timestamp: string;
}
