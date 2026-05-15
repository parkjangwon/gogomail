'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type ApiKey = components['schemas']['APIKey'];
export type CreateApiKeyRequest =
  operations['createAdminDomainAPIKey']['requestBody']['content']['application/json'];
export type ApiKeyListEnvelope =
  operations['listAdminDomainAPIKeys']['responses'][200]['content']['application/json'];
export type ApiKeyCreateEnvelope =
  operations['createAdminDomainAPIKey']['responses'][200]['content']['application/json'];
export type ApiKeyRotateEnvelope =
  operations['rotateAdminDomainAPIKey']['responses'][200]['content']['application/json'];
export type StatusEnvelope =
  operations['deleteAdminDomainAPIKey']['responses'][200]['content']['application/json'];

export function useApiKeys(domainId: string | undefined) {
  return useQuery({
    queryKey: ['domains', domainId, 'api-keys'],
    queryFn: async () => {
      if (!domainId) return [];
      const res = await api.get<ApiKeyListEnvelope>(`/domains/${domainId}/api-keys`);
      return res.keys ?? [];
    },
    enabled: !!domainId,
    staleTime: 30 * 1000,
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
      queryClient.invalidateQueries({ queryKey: ['domains', domainId, 'api-keys'] });
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
      queryClient.invalidateQueries({ queryKey: ['domains', domainId, 'api-keys'] });
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
      queryClient.invalidateQueries({ queryKey: ['domains', domainId, 'api-keys'] });
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
      queryClient.invalidateQueries({ queryKey: ['domains', domainId, 'api-keys'] });
    },
  });
}
