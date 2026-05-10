import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface Permission {
  id: string;
  resource: string;
  action: string;
  scope: string;
}

export interface Role {
  id: string;
  company_id: string;
  name: string;
  permissions: Permission[];
  created_at: string;
  updated_at: string;
}

export interface CreateRoleRequest {
  company_id: string;
  name: string;
  permission_ids: string[];
}

export interface UpdateRoleRequest {
  name: string;
  permission_ids: string[];
}

const AVAILABLE_PERMISSIONS: Permission[] = [
  { id: "user.read", resource: "user", action: "read", scope: "all" },
  { id: "user.create", resource: "user", action: "create", scope: "all" },
  { id: "user.update", resource: "user", action: "update", scope: "all" },
  { id: "user.delete", resource: "user", action: "delete", scope: "all" },
  { id: "domain.read", resource: "domain", action: "read", scope: "all" },
  { id: "domain.create", resource: "domain", action: "create", scope: "all" },
  { id: "domain.update", resource: "domain", action: "update", scope: "all" },
  { id: "domain.delete", resource: "domain", action: "delete", scope: "all" },
  { id: "audit.read", resource: "audit", action: "read", scope: "all" },
  { id: "policy.read", resource: "policy", action: "read", scope: "all" },
  { id: "policy.update", resource: "policy", action: "update", scope: "all" },
];

export function useRoles(companyId: string) {
  return useQuery({
    queryKey: ["roles", companyId],
    queryFn: () => api.get<Role[]>(`/roles/${companyId}`),
    enabled: !!companyId,
    staleTime: 30000,
  });
}

export function useRole(roleId: string) {
  return useQuery({
    queryKey: ["roles", roleId],
    queryFn: () => api.get<Role>(`/roles/${roleId}`),
    enabled: !!roleId,
  });
}

export function useCreateRole() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateRoleRequest) =>
      api.post<Role>("/roles", data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["roles", variables.company_id],
      });
    },
  });
}

export function useUpdateRole(roleId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateRoleRequest) =>
      api.put<Role>(`/roles/${roleId}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["roles", roleId],
      });
    },
  });
}

export function useDeleteRole() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (roleId: string) =>
      api.delete(`/roles/${roleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["roles"],
      });
    },
  });
}

export function useAvailablePermissions() {
  return AVAILABLE_PERMISSIONS;
}
