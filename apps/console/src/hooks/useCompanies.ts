'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type Company = components['schemas']['Company'];
export type CompanyListEnvelope = operations['listAdminCompanies']['responses'][200]['content']['application/json'];
export type CompanyCreateRequest = operations['createAdminCompany']['requestBody']['content']['application/json'];
export type CompanyCreateEnvelope = operations['createAdminCompany']['responses'][201]['content']['application/json'];
export type CompanyUpdateRequest = operations['updateAdminCompany']['requestBody']['content']['application/json'];
export type CompanyUpdateEnvelope = operations['updateAdminCompany']['responses'][200]['content']['application/json'];

const companiesKey = (limit = 200) => ['admin', 'companies', limit] as const;

export function useCompanies(limit = 200) {
  return useQuery({
    queryKey: companiesKey(limit),
    queryFn: async () => {
      const res = await api.get<CompanyListEnvelope>('/companies', { params: { limit } });
      return res.companies ?? [];
    },
  });
}

export function useCreateCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CompanyCreateRequest) => api.post<CompanyCreateEnvelope>('/companies', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'companies'] });
    },
  });
}

export function useUpdateCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: CompanyUpdateRequest }) =>
      api.patch<CompanyUpdateEnvelope>(`/companies/${companyId}`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'companies'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'companies', companyId] });
    },
  });
}

export function useDeleteCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId }: { companyId: string }) => api.delete<void>(`/companies/${companyId}`),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'companies'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'companies', companyId] });
    },
  });
}
