'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface Domain {
  id: string;
  company_id: string;
  name: string;
  status: 'active' | 'inactive' | 'pending';
  is_primary: boolean;
  created_at: string;
  dkim_configured: boolean;
  spf_configured: boolean;
  dmarc_configured: boolean;
}

export function useDomains(companyId: string | undefined) {
  return useQuery({
    queryKey: ['domains', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/domains`) as any;
      return (res.data?.domains || []) as Domain[];
    },
    enabled: !!companyId,
  });
}

export function useCreateDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      domain,
    }: {
      companyId: string;
      domain: Omit<Domain, 'id' | 'created_at'>;
    }) => {
      const res = await api.post(`/companies/${companyId}/domains`, domain) as any;
      return res.data as Domain;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['domains', companyId] });
    },
  });
}

export function useDeleteDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      domainId,
    }: {
      companyId: string;
      domainId: string;
    }) => {
      const res = await api.delete(
        `/companies/${companyId}/domains/${domainId}`
      ) as any;
      return res.data;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['domains', companyId] });
    },
  });
}
