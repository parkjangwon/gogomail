'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface Permission {
  resource: string;
  action: string;
  scope: string;
  conditions?: Record<string, any>;
}

export interface Role {
  id: string;
  company_id: string;
  name: string;
  description: string;
  is_builtin: boolean;
  permissions: Permission[];
  created_at: string;
}

export function useRoles(companyId: string | undefined) {
  return useQuery({
    queryKey: ['roles', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/roles`) as any;
      return (res.data?.roles || []) as Role[];
    },
    enabled: !!companyId,
  });
}

export function useGetRole(companyId: string | undefined, roleId: string | undefined) {
  return useQuery({
    queryKey: ['role', companyId, roleId],
    queryFn: async () => {
      if (!companyId || !roleId) return null;
      const res = await api.get(`/companies/${companyId}/roles/${roleId}`) as any;
      return res.data as Role;
    },
    enabled: !!companyId && !!roleId,
  });
}

export function useCreateRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      data,
    }: {
      companyId: string;
      data: Omit<Role, 'id' | 'created_at'>;
    }) => {
      const res = await api.post(`/companies/${companyId}/roles`, data) as any;
      return res.data as Role;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['roles', companyId] });
    },
  });
}

export function useUpdateRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      roleId,
      data,
    }: {
      companyId: string;
      roleId: string;
      data: Partial<Role>;
    }) => {
      const res = await api.put(`/companies/${companyId}/roles/${roleId}`, data) as any;
      return res.data as Role;
    },
    onSuccess: (_, { companyId, roleId }) => {
      queryClient.invalidateQueries({ queryKey: ['roles', companyId] });
      queryClient.invalidateQueries({ queryKey: ['role', companyId, roleId] });
    },
  });
}

export function useDeleteRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      roleId,
    }: {
      companyId: string;
      roleId: string;
    }) => {
      const res = await api.delete(`/companies/${companyId}/roles/${roleId}`) as any;
      return res.data;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['roles', companyId] });
    },
  });
}
