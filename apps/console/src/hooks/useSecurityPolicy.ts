'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface SecurityPolicy {
  company_id: string;
  mfa_mode: 'disabled' | 'optional' | 'required';
  mfa_grace_period_days: number;
  session_timeout_minutes: number;
  password_min_length: number;
  password_require_uppercase: boolean;
  password_require_numbers: boolean;
  password_require_special: boolean;
  password_expiration_days?: number;
  ip_restriction_enabled: boolean;
  allowed_ips?: string[];
  login_failure_lockout_attempts: number;
  login_failure_lockout_duration_minutes: number;
  updated_at: string;
}

export function useSecurityPolicy(companyId: string | undefined) {
  return useQuery({
    queryKey: ['securityPolicy', companyId],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get(`/companies/${companyId}/security-policy`) as any;
      return res.data as SecurityPolicy;
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
      const res = await api.put(
        `/companies/${companyId}/security-policy`,
        policy
      ) as any;
      return res.data as SecurityPolicy;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['securityPolicy', companyId] });
    },
  });
}
