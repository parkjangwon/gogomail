'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface SecurityPolicy {
  min_length: number;
  require_uppercase: boolean;
  require_numbers: boolean;
  require_symbols: boolean;
  max_age_days: number;
  history_count: number;
  mfa_required: boolean;
  mfa_methods: string[];
  session_timeout_minutes: number;
  max_concurrent_sessions: number;
}

interface SecurityPolicyEnvelope {
  policy: SecurityPolicy;
}

export function useSecurityPolicy(companyId: string | undefined) {
  return useQuery({
    queryKey: ['securityPolicy', companyId],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get<SecurityPolicyEnvelope>(`/companies/${companyId}/security/auth-policy`);
      return res.policy;
    },
    enabled: !!companyId,
  });
}

export function useUpdateSecurityPolicy() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      policy,
    }: {
      companyId: string;
      policy: Partial<SecurityPolicy>;
    }) => {
      const res = await api.put<SecurityPolicyEnvelope>(
        `/companies/${companyId}/security/auth-policy`,
        policy
      );
      return res.policy;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['securityPolicy', companyId] });
    },
  });
}
