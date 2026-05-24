'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface OrganizationMember {
  id: string;
  name: string;
  email: string;
  role: string;
  parent_id?: string;
  children?: OrganizationMember[];
}

export interface OrganizationNode {
  id: string;
  name: string;
  type?: 'company' | 'group' | 'user' | 'unit';
  parent_id?: string;
  member_count?: number;
  children?: OrganizationNode[];
}

interface OrganizationHierarchyEnvelope {
  hierarchy: OrganizationNode | null;
}

interface OrganizationUnitEnvelope {
  unit: OrganizationNode;
}

export function useOrganizationStructure(companyId: string | undefined) {
  return useQuery({
    queryKey: ['organizationStructure', companyId],
    queryFn: async () => {
      if (!companyId) return { root: null, nodes: [] };
      const res = await api.get<OrganizationHierarchyEnvelope>('/organization/hierarchy', {
        params: { company_id: companyId },
      });
      return { root: res.hierarchy, nodes: res.hierarchy ? [res.hierarchy] : [] };
    },
    enabled: !!companyId,
  });
}

export function useUpdateOrganizationHierarchy() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      nodeId,
      newParentId,
    }: {
      companyId: string;
      nodeId: string;
      newParentId: string;
    }) => {
      const res = await api.put<OrganizationUnitEnvelope>(
        `/organization/units/${nodeId}`,
        { parent_id: newParentId }
      );
      return res.unit;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({
        queryKey: ['organizationStructure', companyId],
      });
    },
  });
}

export function useGetOrganizationNode(companyId: string | undefined, nodeId: string | undefined) {
  return useQuery({
    queryKey: ['organizationNode', companyId, nodeId],
    queryFn: async () => {
      if (!companyId || !nodeId) return null;
      const res = await api.get<OrganizationUnitEnvelope>(`/organization/units/${nodeId}`);
      return res.unit;
    },
    enabled: !!companyId && !!nodeId,
  });
}

export interface OrgMembership {
  member_id: string;
  unit_id: string;
  unit_name: string;
  title: string;
  role: string;
  is_primary: boolean;
}

export interface OrgUnit {
  id: string;
  name: string;
  display_name?: string;
  type?: string;
  status?: string;
}

export function useUserOrgMemberships(userId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ['userOrgMemberships', userId],
    queryFn: async () => {
      if (!userId) return [];
      const res = await api.get<{ memberships: OrgMembership[] }>(`/organization/members`, {
        params: { user_id: userId },
      });
      return res.memberships ?? [];
    },
    enabled: enabled && !!userId,
  });
}

export function useOrgUnits(companyId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ['orgUnits', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<{ units: OrgUnit[] }>(`/organization/units`, {
        params: { company_id: companyId },
      });
      return (res.units ?? []).filter(u => u.status === 'active' || !u.status);
    },
    enabled: enabled && !!companyId,
  });
}

export function useAssignOrgMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ unitId, userId, role, title }: { unitId: string; userId: string; role: string; title: string }) => {
      await api.post('/organization/members', { unit_id: unitId, user_id: userId, role, title });
    },
    onSuccess: (_, { userId }) => {
      queryClient.invalidateQueries({ queryKey: ['userOrgMemberships', userId] });
    },
  });
}

export function useUpdateOrgMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ memberId, userId, title, role }: { memberId: string; userId: string; title: string; role: string }) => {
      await api.patch(`/organization/members/${memberId}`, { title, role });
      return userId;
    },
    onSuccess: (userId) => {
      queryClient.invalidateQueries({ queryKey: ['userOrgMemberships', userId] });
    },
  });
}

export function useRemoveOrgMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ memberId, userId }: { memberId: string; userId: string }) => {
      await api.delete(`/organization/members/${memberId}`);
      return userId;
    },
    onSuccess: (userId) => {
      queryClient.invalidateQueries({ queryKey: ['userOrgMemberships', userId] });
    },
  });
}
