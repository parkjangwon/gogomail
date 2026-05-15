'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { operations } from '@gogomail/api-types';

export type OrganizationSettingsEnvelope = operations['adminGetOrganizationSettings']['responses'][200]['content']['application/json'];
export type OrganizationSettingsUpdateRequest = operations['adminUpdateOrganizationSettings']['requestBody']['content']['application/json'];
export type CompanySignatureEnvelope = operations['getGlobalSignature']['responses'][200]['content']['application/json'];
export type CompanySignatureUpdateRequest = operations['putGlobalSignature']['requestBody']['content']['application/json'];
export type CompanySSOConfigEnvelope = operations['adminGetCompanySSOConfig']['responses'][200]['content']['application/json'];
export type CompanySSOConfigUpdateRequest = operations['adminUpdateCompanySSOConfig']['requestBody']['content']['application/json'];
export type CompanySCIMStatusEnvelope = operations['getSCIMStatus']['responses'][200]['content']['application/json'];

const orgSettingsKey = ['admin', 'organization', 'settings'] as const;
const signatureKey = (companyId: string) => ['companies', companyId, 'signature'] as const;
const ssoKey = (companyId: string) => ['companies', companyId, 'sso'] as const;
const scimKey = (companyId: string) => ['companies', companyId, 'scim-status'] as const;

export function useOrganizationSettings() {
  return useQuery({
    queryKey: orgSettingsKey,
    queryFn: async () => api.get<OrganizationSettingsEnvelope>('/organization/settings'),
  });
}

export function useUpdateOrganizationSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: OrganizationSettingsUpdateRequest) =>
      api.put<OrganizationSettingsEnvelope>('/organization/settings', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: orgSettingsKey });
    },
  });
}

export function useCompanySignature(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? signatureKey(companyId) : ['companies', 'signature'],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get<CompanySignatureEnvelope>(`/companies/${companyId}/signature`);
      return res.signature ?? null;
    },
    enabled: !!companyId,
  });
}

export function useUpdateCompanySignature() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: CompanySignatureUpdateRequest }) =>
      api.put<Record<string, never>>(`/companies/${companyId}/signature`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: signatureKey(companyId) });
    },
  });
}

export function useCompanySSOConfig(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? ssoKey(companyId) : ['companies', 'sso'],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get<CompanySSOConfigEnvelope>(`/companies/${companyId}/sso/config`);
      return res.config ?? null;
    },
    enabled: !!companyId,
  });
}

export function useUpdateCompanySSOConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: CompanySSOConfigUpdateRequest }) =>
      api.put<Record<string, never>>(`/companies/${companyId}/sso/config`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ssoKey(companyId) });
    },
  });
}

export function useTestCompanySSOConfig() {
  return useMutation({
    mutationFn: async ({ companyId }: { companyId: string }) =>
      api.post<{ message?: string; success?: boolean }>(`/companies/${companyId}/sso/test`),
  });
}

export function useSCIMStatus(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? scimKey(companyId) : ['companies', 'scim-status'],
    queryFn: async () => {
      if (!companyId) return null;
      return api.get<CompanySCIMStatusEnvelope>(`/companies/${companyId}/scim/status`);
    },
    enabled: !!companyId,
  });
}
