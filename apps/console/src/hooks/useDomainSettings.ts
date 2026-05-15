'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface DomainSettings {
  domain_id: string;
  tls_policy: string;
  quota_per_user: number;
  ip_whitelist_enabled: boolean;
  ip_whitelist: string[];
  require_2fa: boolean;
  session_timeout_minutes: number;
  password_min_length: number;
  password_require_uppercase: boolean;
  password_require_numbers: boolean;
  password_require_special_chars: boolean;
  password_expiry_days: number;
  user_registration_mode: string;
  password_reset_token_ttl_minutes: number;
  updated_at: string;
  updated_by: string;
}

interface DomainSettingsEnvelope {
  settings: DomainSettings;
}

export function useDomainSettings(domainId: string | undefined) {
  return useQuery({
    queryKey: ['domain-settings', domainId],
    queryFn: async () => {
      if (!domainId) return null;
      const res = await api.get<DomainSettingsEnvelope>(`/domains/${domainId}/settings`);
      return res.settings;
    },
    enabled: !!domainId,
  });
}

export function useUpdateDomainSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      domainId,
      settings,
    }: {
      domainId: string;
      settings: DomainSettings;
    }) => api.put<{ status: string; id: string }>(`/domains/${domainId}/settings`, settings),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['domain-settings', variables.domainId] });
      queryClient.invalidateQueries({ queryKey: ['domains'] });
    },
  });
}
