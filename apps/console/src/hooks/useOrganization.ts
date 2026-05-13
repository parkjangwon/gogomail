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
