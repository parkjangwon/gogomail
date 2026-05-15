'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components, operations } from '@gogomail/api-types';

export type CompanyHealth = NonNullable<operations['adminGetCompanyHealth']['responses'][200]['content']['application/json']['health']>;
export type CompanyChangeLog = components['schemas']['AuditLogView'];
export type CompanyApproval = components['schemas']['ApprovalItem'];

export type CompanyHealthEnvelope = operations['adminGetCompanyHealth']['responses'][200]['content']['application/json'];
export type CompanyChangeHistoryEnvelope = operations['adminGetCompanyChangeHistory']['responses'][200]['content']['application/json'];
export type CompanyApprovalListEnvelope = operations['adminListPendingApprovals']['responses'][200]['content']['application/json'];
export type CompanyApprovalCreateRequest = operations['adminCreatePendingApproval']['requestBody']['content']['application/json'];
export type CompanyApprovalCreateEnvelope = operations['adminCreatePendingApproval']['responses'][201]['content']['application/json'];
export type CompanyApprovalActionEnvelope = operations['adminApproveApproval']['responses'][200]['content']['application/json'];

const healthKey = (companyId: string) => ['companies', companyId, 'health'] as const;
const changeHistoryKey = (companyId: string, category?: string) => ['companies', companyId, 'change-history', category ?? 'all'] as const;
const approvalsKey = (companyId: string, status?: string) => ['companies', companyId, 'pending-approvals', status ?? 'pending'] as const;

export function useTenantHealth(companyId: string | undefined, refetchInterval = 30_000) {
  return useQuery({
    queryKey: companyId ? healthKey(companyId) : ['companies', 'health'],
    queryFn: async () => {
      if (!companyId) return null;
      const res = await api.get<CompanyHealthEnvelope>(`/companies/${companyId}/health`);
      return res.health ?? null;
    },
    enabled: !!companyId,
    refetchInterval,
  });
}

export function useCompanyChangeHistory(companyId: string | undefined, category?: string) {
  return useQuery({
    queryKey: companyId ? changeHistoryKey(companyId, category) : ['companies', 'change-history'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<CompanyChangeHistoryEnvelope>(`/companies/${companyId}/change-history`, {
        params: {
          limit: 100,
          ...(category ? { category } : {}),
        },
      });
      return res.changes ?? [];
    },
    enabled: !!companyId,
  });
}

export function usePendingApprovals(companyId: string | undefined, status = 'pending') {
  return useQuery({
    queryKey: companyId ? approvalsKey(companyId, status) : ['companies', 'pending-approvals', status],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<CompanyApprovalListEnvelope>(`/companies/${companyId}/pending-approvals`, {
        params: { status },
      });
      return res.approvals ?? [];
    },
    enabled: !!companyId,
  });
}

export function useCreatePendingApproval() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: CompanyApprovalCreateRequest }) =>
      api.post<CompanyApprovalCreateEnvelope>(`/companies/${companyId}/pending-approvals`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: approvalsKey(companyId) });
    },
  });
}

export function useApprovePendingApproval() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      approvalId,
      data,
    }: {
      companyId: string;
      approvalId: string;
      data?: { comment?: string; reviewed_by?: string };
    }) => api.post<CompanyApprovalActionEnvelope>(`/companies/${companyId}/pending-approvals/${approvalId}/approve`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: approvalsKey(companyId) });
    },
  });
}

export function useRejectPendingApproval() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      approvalId,
      data,
    }: {
      companyId: string;
      approvalId: string;
      data?: { comment?: string; reviewed_by?: string };
    }) => api.post<CompanyApprovalActionEnvelope>(`/companies/${companyId}/pending-approvals/${approvalId}/reject`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: approvalsKey(companyId) });
    },
  });
}
