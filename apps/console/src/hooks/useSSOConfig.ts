'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface SSOConfig {
  id: string;
  company_id: string;
  provider_type: 'ldap' | 'oidc' | 'saml';
  name: string;
  is_enabled: boolean;
  config: Record<string, any>;
  attribute_mapping: Record<string, string>;
  created_at: string;
}

export function useSSOConfigs(companyId: string | undefined) {
  return useQuery({
    queryKey: ['ssoConfigs', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/sso-config`) as any;
      return (res.data?.configs || []) as SSOConfig[];
    },
    enabled: !!companyId,
  });
}

export function useCreateSSOConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      config,
    }: {
      companyId: string;
      config: Omit<SSOConfig, 'id' | 'created_at'>;
    }) => {
      const res = await api.post(`/companies/${companyId}/sso-config`, config) as any;
      return res.data as SSOConfig;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['ssoConfigs', companyId] });
    },
  });
}

export function useUpdateSSOConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      configId,
      config,
    }: {
      companyId: string;
      configId: string;
      config: Partial<SSOConfig>;
    }) => {
      const res = await api.put(
        `/companies/${companyId}/sso-config/${configId}`,
        config
      ) as any;
      return res.data as SSOConfig;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['ssoConfigs', companyId] });
    },
  });
}
