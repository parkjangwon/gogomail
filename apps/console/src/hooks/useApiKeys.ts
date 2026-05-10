'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface ApiKey {
  id: string;
  domain_id: string;
  name: string;
  key_hash: string;
  prefix: string;
  cidr_allowlist?: string[];
  cidr_allowlist_enabled: boolean;
  last_used_at?: string;
  created_at: string;
  expires_at?: string;
  request_count: number;
}

export interface CreateApiKeyRequest {
  name: string;
  expires_in_days?: number;
  cidr_allowlist?: string[];
}

export function useApiKeys(domainId: string | undefined) {
  return useQuery({
    queryKey: ['apiKeys', domainId],
    queryFn: async () => {
      if (!domainId) return [];
      const res = await api.get(`/domains/${domainId}/api-keys`) as any;
      return (res.data?.keys || []) as ApiKey[];
    },
    enabled: !!domainId,
  });
}

export function useCreateApiKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      domainId,
      data,
    }: {
      domainId: string;
      data: CreateApiKeyRequest;
    }) => {
      const res = await api.post(`/domains/${domainId}/api-keys`, data) as any;
      return res.data;
    },
    onSuccess: (_, { domainId }) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys', domainId] });
    },
  });
}

export function useRotateApiKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      domainId,
      keyId,
    }: {
      domainId: string;
      keyId: string;
    }) => {
      const res = await api.post(`/domains/${domainId}/api-keys/${keyId}/rotate`, {}) as any;
      return res.data;
    },
    onSuccess: (_, { domainId }) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys', domainId] });
    },
  });
}

export function useDeleteApiKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      domainId,
      keyId,
    }: {
      domainId: string;
      keyId: string;
    }) => {
      const res = await api.delete(`/domains/${domainId}/api-keys/${keyId}`) as any;
      return res.data;
    },
    onSuccess: (_, { domainId }) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys', domainId] });
    },
  });
}

export function useUpdateApiKeyCIDR() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      domainId,
      keyId,
      cidrList,
      enabled,
    }: {
      domainId: string;
      keyId: string;
      cidrList: string[];
      enabled: boolean;
    }) => {
      const res = await api.put(
        `/domains/${domainId}/api-keys/${keyId}`,
        { cidr_allowlist: cidrList, cidr_allowlist_enabled: enabled }
      ) as any;
      return res.data;
    },
    onSuccess: (_, { domainId }) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys', domainId] });
    },
  });
}
