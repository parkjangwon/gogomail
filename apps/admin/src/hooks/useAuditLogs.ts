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
  changes?: Record<string, any>;
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
  logs: AuditLog[];
  total: number;
  page: number;
  per_page: number;
}

export function useAuditLogs(companyId: string | undefined, filters?: AuditLogFilter) {
  return useQuery({
    queryKey: ['auditLogs', companyId, filters],
    queryFn: async () => {
      if (!companyId) return { logs: [], total: 0, page: 1, per_page: 50 };

      const params = new URLSearchParams();
      if (filters?.start_date) params.append('start_date', filters.start_date);
      if (filters?.end_date) params.append('end_date', filters.end_date);
      if (filters?.action) params.append('action', filters.action);
      if (filters?.resource_type) params.append('resource_type', filters.resource_type);
      if (filters?.admin_user_id) params.append('admin_user_id', filters.admin_user_id);
      if (filters?.limit) params.append('limit', filters.limit.toString());
      if (filters?.offset) params.append('offset', filters.offset.toString());

      const res = await api.get(`/companies/${companyId}/audit-logs?${params}`) as any;
      return res.data || { logs: [], total: 0, page: 1, per_page: 50 };
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

      const res = await api.get(`/companies/${companyId}/audit-logs/export?${params}`) as any;
      return res.data;
    },
  });
}
