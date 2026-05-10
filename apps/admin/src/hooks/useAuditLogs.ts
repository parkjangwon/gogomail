import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

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

export interface AuditLogFilter {
  company_id?: string;
  admin_user_id?: string;
  action?: string;
  resource_type?: string;
  start_time?: string;
  end_time?: string;
  limit?: number;
  offset?: number;
}

export function useAuditLogs(filter?: AuditLogFilter) {
  return useQuery({
    queryKey: ["audit-logs", filter],
    queryFn: () => api.get<AuditLog[]>("/audit-logs", { params: filter as any }),
    staleTime: 0, // Audit logs should always be fresh
  });
}

export function useAuditLog(logId: string) {
  return useQuery({
    queryKey: ["audit-logs", logId],
    queryFn: () => api.get<AuditLog>(`/audit-logs/${logId}`),
    enabled: !!logId,
  });
}
