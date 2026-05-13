'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface Domain {
  id: string;
  company_id: string;
  name: string;
  name_ace: string;
  status: 'active' | 'suspended' | 'disabled';
  quota_used: number;
  quota_limit?: number;
  quota_remaining: number;
  default_user_quota?: number;
  allocated_user_quota: number;
  allocatable_user_quota: number;
  over_allocated: boolean;
  last_dns_check_status?: 'ok' | 'missing' | 'mismatch' | 'error';
  last_dns_checked_at?: string;
  created_at: string;
}

interface DomainListEnvelope {
  domains: Domain[];
}

interface DomainEnvelope {
  domain: Domain;
}

export interface CreateDomainInput {
  company_id: string;
  name: string;
  name_ace?: string;
  quota_limit?: number;
  default_user_quota?: number;
  quota_source?: 'default' | 'custom';
}

export function useDomains(companyId: string | undefined) {
  return useQuery({
    queryKey: ['domains', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<DomainListEnvelope>('/domains', {
        params: { company_id: companyId, limit: 200 },
      });
      return res.domains;
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
      domain: Omit<CreateDomainInput, 'company_id'>;
    }) => {
      const res = await api.post<DomainEnvelope>('/domains', {
        ...domain,
        company_id: companyId,
      });
      return res.domain;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['domains', companyId] });
    },
  });
}

export function useDeleteDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ domainId }: {
      companyId: string;
      domainId: string;
    }) => {
      return api.delete<void>(`/domains/${domainId}`);
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['domains', companyId] });
    },
  });
}
