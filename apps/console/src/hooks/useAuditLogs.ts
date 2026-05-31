'use client';

import { useQuery, useMutation } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface AuditLog {
  id: string;
  company_id: string;
  admin_user_id: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  changes?: Record<string, unknown>;
  ip_address?: string;
  user_agent?: string;
  timestamp: string;
}

export interface AuditLogFilter {
  start_date?: string;
  end_date?: string;
  action?: string;
  resource_type?: string;
  admin_user_id?: string;
  limit?: number;
  offset?: number;
}

export interface AuditLogResponse {
  audit_logs: AuditLog[];
}

export function useAuditLogs(companyId: string | undefined, filters?: AuditLogFilter) {
  return useQuery({
    queryKey: ['auditLogs', companyId, filters],
    queryFn: async () => {
      if (!companyId) return { audit_logs: [] };

      const params = new URLSearchParams();
      params.append('company_id', companyId);
      if (filters?.start_date) params.append('since', filters.start_date);
      if (filters?.action) params.append('action', filters.action);
      if (filters?.resource_type) params.append('target_type', filters.resource_type);
      if (filters?.admin_user_id) params.append('actor_id', filters.admin_user_id);
      if (filters?.limit) params.append('limit', filters.limit.toString());

      const res = await api.get<AuditLogResponse>(`/audit-logs?${params}`);
      return res;
    },
    enabled: !!companyId,
    staleTime: 0,
  });
}

export function useExportAuditLogs() {
  return useMutation({
    mutationFn: async ({
      companyId,
      format,
      filters,
    }: {
      companyId: string;
      format: 'csv' | 'json';
      filters?: AuditLogFilter;
    }) => {
      const params = new URLSearchParams();
      params.append('format', format);
      if (filters?.start_date) params.append('start_date', filters.start_date);
      if (filters?.end_date) params.append('end_date', filters.end_date);
      if (filters?.action) params.append('action', filters.action);
      if (filters?.resource_type) params.append('resource_type', filters.resource_type);

      return api.get(`/companies/${companyId}/audit-logs/export?${params}`);
    },
  });
}
