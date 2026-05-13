'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface Role {
  id: string;
  name: string;
  description?: string;
  permissions_count: number;
  assigned_users: number;
  created_at: string;
}

interface RoleListEnvelope {
  roles: Role[];
}

interface RoleEnvelope {
  role: Role;
}

export interface CreateRoleInput {
  name: string;
  description?: string;
}

export function useRoles(companyId: string | undefined) {
  return useQuery({
    queryKey: ['roles', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<RoleListEnvelope>('/roles', {
        params: { limit: 200 },
      });
      return res.roles;
    },
    enabled: !!companyId,
  });
}

export function useGetRole(companyId: string | undefined, roleId: string | undefined) {
  return useQuery({
    queryKey: ['role', companyId, roleId],
    queryFn: async () => {
      if (!companyId || !roleId) return null;
      const res = await api.get<RoleListEnvelope>('/roles', {
        params: { limit: 200 },
      });
      return res.roles.find((role) => role.id === roleId) ?? null;
    },
    enabled: !!companyId && !!roleId,
  });
}

export function useCreateRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ data }: {
      companyId: string;
      data: CreateRoleInput;
    }) => {
      const res = await api.post<RoleEnvelope>('/roles', data);
      return res.role;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['roles', companyId] });
    },
  });
}

export function useUpdateRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (_variables: {
      companyId: string;
      roleId: string;
      data: Partial<Role>;
    }) => {
      throw new Error('Role update is not supported by the current Admin API contract');
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
    mutationFn: async (_variables: {
      companyId: string;
      roleId: string;
    }) => {
      throw new Error('Role delete is not supported by the current Admin API contract');
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['roles', companyId] });
    },
  });
}
