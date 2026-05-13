'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface ApiKey {
  id: string;
  domain_id: string;
  name: string;
  created_by: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
  is_active: boolean;
}

export interface CreateApiKeyRequest {
  name: string;
  created_by: string;
}

interface ApiKeyListEnvelope {
  keys: ApiKey[];
}

interface ApiKeyCreateEnvelope {
  id: string;
  secret: string;
}

interface ApiKeyRotateEnvelope {
  status: string;
  secret: string;
}

interface StatusEnvelope {
  status: string;
}

export function useApiKeys(domainId: string | undefined) {
  return useQuery({
    queryKey: ['apiKeys', domainId],
    queryFn: async () => {
      if (!domainId) return [];
      const res = await api.get<ApiKeyListEnvelope>(`/domains/${domainId}/api-keys`);
      return res.keys;
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
      return api.post<ApiKeyCreateEnvelope>(`/domains/${domainId}/api-keys`, data);
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
      return api.post<ApiKeyRotateEnvelope>(`/domains/${domainId}/api-keys/${keyId}/rotate`, {});
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
      return api.delete<StatusEnvelope>(`/domains/${domainId}/api-keys/${keyId}`);
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
      void domainId;
      void keyId;
      void cidrList;
      void enabled;
      throw new Error('Per-key CIDR updates are not supported by the current Admin API contract');
    },
    onSuccess: (_, { domainId }) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys', domainId] });
    },
  });
}
