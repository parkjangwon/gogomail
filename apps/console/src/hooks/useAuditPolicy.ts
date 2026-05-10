import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export type AuditLevel = "level_1" | "level_2" | "level_3";

export interface AuditPolicyConfig {
  company_id: string;
  audit_level: AuditLevel;
  audit_admin_actions: boolean;
  audit_security_events: boolean;
}

export function useAuditPolicy(companyId: string) {
  return useQuery({
    queryKey: ["audit-policy", companyId],
    queryFn: () => api.get<AuditPolicyConfig>(`/audit-policy/${companyId}`),
    enabled: !!companyId,
    staleTime: 5 * 60 * 1000,
  });
}

export function useUpdateAuditPolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: AuditPolicyConfig) =>
      api.put<AuditPolicyConfig>(`/audit-policy/${data.company_id}`, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["audit-policy", variables.company_id],
      });
    },
  });
}
