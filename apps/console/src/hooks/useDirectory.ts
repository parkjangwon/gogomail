'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components } from '@gogomail/api-types';

export type DirectoryPrincipal = components['schemas']['DirectoryPrincipal'];
export type DirectoryPrincipalListEnvelope =
  components['responses']['DirectoryPrincipalList']['content']['application/json'];

export type DirectoryAlias = components['schemas']['DirectoryAlias'];
export type DirectoryAliasListEnvelope =
  components['responses']['DirectoryAliasList']['content']['application/json'];
export type DirectoryAliasCreateRequest =
  components['requestBodies']['DirectoryAliasCreate']['content']['application/json'];
export type DirectoryAliasCreateResponse =
  components['responses']['DirectoryAlias']['content']['application/json'];

export type DirectoryDelegation = components['schemas']['DirectoryDelegation'];
export type DirectoryDelegationListEnvelope =
  components['responses']['DirectoryDelegationList']['content']['application/json'];
export type DirectoryDelegationCreateRequest =
  components['requestBodies']['DirectoryDelegationCreate']['content']['application/json'];
export type DirectoryDelegationCreateResponse =
  components['responses']['DirectoryDelegation']['content']['application/json'];

export type DirectoryGroupMembership = components['schemas']['DirectoryGroupMembership'];
export type DirectoryGroupMembershipListEnvelope =
  components['responses']['DirectoryGroupMembershipList']['content']['application/json'];
export type DirectoryGroupMembershipCreateRequest =
  components['requestBodies']['DirectoryGroupMembershipCreate']['content']['application/json'];
export type DirectoryGroupMembershipCreateResponse =
  components['responses']['DirectoryGroupMembership']['content']['application/json'];

export type DirectoryDelegationRoleUpdateRequest =
  components['requestBodies']['DirectoryDelegationRoleUpdate']['content']['application/json'];
export type DirectoryDelegationRoleUpdateResponse =
  components['responses']['DirectoryDelegation']['content']['application/json'];
export type DirectoryDelegationReassignRequest =
  components['requestBodies']['DirectoryDelegationReassign']['content']['application/json'];
export type DirectoryGroupMembershipRoleUpdateRequest =
  components['requestBodies']['DirectoryGroupMembershipRoleUpdate']['content']['application/json'];
export type DirectoryGroupMembershipRoleUpdateResponse =
  components['responses']['DirectoryGroupMembership']['content']['application/json'];
export type DirectoryGroupMembershipReassignRequest =
  components['requestBodies']['DirectoryGroupMembershipReassign']['content']['application/json'];

const principalsKey = (companyId: string) => ['companies', companyId, 'directory-principals'] as const;
const aliasesKey = (companyId: string) => ['companies', companyId, 'directory-aliases'] as const;
const delegationsKey = (companyId: string) => ['companies', companyId, 'directory-delegations'] as const;
const membershipsKey = (companyId: string) => ['companies', companyId, 'directory-group-memberships'] as const;

export function useDirectoryPrincipals(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? principalsKey(companyId) : ['companies', 'directory-principals'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<DirectoryPrincipalListEnvelope>('/directory/principals', {
        params: { company_id: companyId, limit: 100 },
      });
      return res.directory_principals ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}

export function useDirectoryAliases(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? aliasesKey(companyId) : ['companies', 'directory-aliases'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<DirectoryAliasListEnvelope>('/directory/aliases', {
        params: { company_id: companyId, limit: 100 },
      });
      return res.directory_aliases ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}

export function useCreateDirectoryAlias() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: DirectoryAliasCreateRequest }) =>
      api.post<DirectoryAliasCreateResponse>('/directory/aliases', { ...data, company_id: companyId }),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: aliasesKey(companyId) });
    },
  });
}

export function useDeleteDirectoryAlias() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }: { id: string; companyId: string }) =>
      api.delete<DirectoryAliasCreateResponse>(`/directory/aliases/${id}`),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: aliasesKey(companyId) });
    },
  });
}

export function useDirectoryDelegations(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? delegationsKey(companyId) : ['companies', 'directory-delegations'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<DirectoryDelegationListEnvelope>('/directory/delegations', {
        params: { company_id: companyId, limit: 200 },
      });
      return res.directory_delegations ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}

export function useCreateDirectoryDelegation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: DirectoryDelegationCreateRequest }) =>
      api.post<DirectoryDelegationCreateResponse>('/directory/delegations', { ...data, company_id: companyId }),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: delegationsKey(companyId) });
    },
  });
}

export function useDeleteDirectoryDelegation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }: { id: string; companyId: string }) =>
      api.delete<DirectoryDelegationCreateResponse>(`/directory/delegations/${id}`),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: delegationsKey(companyId) });
    },
  });
}

export function useUpdateDirectoryDelegationRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; companyId: string; data: DirectoryDelegationRoleUpdateRequest }) =>
      api.patch<DirectoryDelegationRoleUpdateResponse>(`/directory/delegations/${id}/role`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: delegationsKey(companyId) });
    },
  });
}

export function useReassignDirectoryDelegation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; companyId: string; data: DirectoryDelegationReassignRequest }) =>
      api.patch<DirectoryDelegationRoleUpdateResponse>(`/directory/delegations/${id}/assignment`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: delegationsKey(companyId) });
    },
  });
}

export function useDirectoryGroupMemberships(companyId: string | undefined) {
  return useQuery({
    queryKey: companyId ? membershipsKey(companyId) : ['companies', 'directory-group-memberships'],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<DirectoryGroupMembershipListEnvelope>('/directory/group-memberships', {
        params: { company_id: companyId, limit: 100 },
      });
      return res.directory_group_memberships ?? [];
    },
    enabled: !!companyId,
    staleTime: 30_000,
  });
}

export function useCreateDirectoryGroupMembership() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ companyId, data }: { companyId: string; data: DirectoryGroupMembershipCreateRequest }) =>
      api.post<DirectoryGroupMembershipCreateResponse>('/directory/group-memberships', {
        ...data,
        company_id: companyId,
      }),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: membershipsKey(companyId) });
    },
  });
}

export function useDeleteDirectoryGroupMembership() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }: { id: string; companyId: string }) =>
      api.delete<DirectoryGroupMembershipCreateResponse>(`/directory/group-memberships/${id}`),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: membershipsKey(companyId) });
    },
  });
}

export function useUpdateDirectoryGroupMembershipRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; companyId: string; data: DirectoryGroupMembershipRoleUpdateRequest }) =>
      api.patch<DirectoryGroupMembershipRoleUpdateResponse>(`/directory/group-memberships/${id}/role`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: membershipsKey(companyId) });
    },
  });
}

export function useReassignDirectoryGroupMembership() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }: { id: string; companyId: string; data: DirectoryGroupMembershipReassignRequest }) =>
      api.patch<DirectoryGroupMembershipRoleUpdateResponse>(`/directory/group-memberships/${id}/assignment`, data),
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: membershipsKey(companyId) });
    },
  });
}
