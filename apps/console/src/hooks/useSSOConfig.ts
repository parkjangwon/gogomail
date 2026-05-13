'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface SSOConfig {
  enabled: boolean;
  provider: string;
  entity_id: string;
  metadata_url: string;
  sso_login_url: string;
  certificate: string;
  attribute_email: string;
  attribute_name: string;
  force_sso: boolean;
  auto_provision: boolean;
  default_role: string;
}

interface SSOConfigEnvelope {
  config: SSOConfig;
}

export function useSSOConfigs(companyId: string | undefined) {
  return useQuery({
    queryKey: ['ssoConfigs', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<SSOConfigEnvelope>(`/companies/${companyId}/sso/config`);
      return [res.config];
    },
    enabled: !!companyId,
  });
}

export function useUpdateSSOConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      config,
    }: {
      companyId: string;
      config: SSOConfig;
    }) => {
      const res = await api.put<SSOConfigEnvelope>(`/companies/${companyId}/sso/config`, config);
      return res.config;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['ssoConfigs', companyId] });
    },
  });
}

export const useCreateSSOConfig = useUpdateSSOConfig;
