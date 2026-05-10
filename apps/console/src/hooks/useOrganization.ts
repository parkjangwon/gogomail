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
  type: 'company' | 'group' | 'user';
  parent_id?: string;
  member_count?: number;
}

export function useOrganizationStructure(companyId: string | undefined) {
  return useQuery({
    queryKey: ['organizationStructure', companyId],
    queryFn: async () => {
      if (!companyId) return { root: null, nodes: [] };
      const res = await api.get(`/companies/${companyId}/organization/structure`) as any;
      return res.data || { root: null, nodes: [] };
    },
    enabled: !!companyId,
  });
}

export function useUpdateOrganizationHierarchy() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      nodeId,
      newParentId,
    }: {
      companyId: string;
      nodeId: string;
      newParentId: string;
    }) => {
      const res = await api.put(
        `/companies/${companyId}/organization/structure/${nodeId}`,
        { parent_id: newParentId }
      ) as any;
      return res.data;
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
      const res = await api.get(`/companies/${companyId}/organization/${nodeId}`) as any;
      return res.data as OrganizationNode;
    },
    enabled: !!companyId && !!nodeId,
  });
}
