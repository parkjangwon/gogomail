import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

type UserStatus = "active" | "suspended" | "disabled";
type UserRole = "user" | "company_admin" | "system_admin";

export interface User {
  id: string;
  domain_id: string;
  username: string;
  display_name: string;
  role: UserRole | string;
  status: UserStatus;
  password_configured: boolean;
  must_change_password?: boolean;
  quota_used: number;
  quota_limit?: number;
  quota_remaining: number;
  quota_source: "default" | "custom";
  created_at: string;
}

interface Domain {
  id: string;
  company_id: string;
}

interface DomainListEnvelope {
  domains: Domain[];
}

interface UserListEnvelope {
  users: User[];
}

interface UserEnvelope {
  user: User;
}

interface IDStatusEnvelope {
  status: "ok";
  id: string;
}

export interface CreateUserInput {
  domain_id: string;
  username: string;
  display_name: string;
  address: string;
  password?: string;
  password_hash?: string;
  must_change_password?: boolean;
  quota_limit?: number;
}

export function useUsers(companyId?: string) {
  return useQuery({
    queryKey: ["users", companyId],
    queryFn: async () => {
      if (!companyId) {
        const res = await api.get<UserListEnvelope>("/users", { params: { limit: 200 } });
        return res.users;
      }

      const domains = await api.get<DomainListEnvelope>("/domains", {
        params: { company_id: companyId, limit: 200 },
      });
      const usersByDomain = await Promise.all(
        domains.domains.map((domain) =>
          api.get<UserListEnvelope>("/users", {
            params: { domain_id: domain.id, limit: 200 },
          })
        )
      );
      return usersByDomain.flatMap((res) => res.users);
    },
    staleTime: 30 * 1000,
  });
}

export function useUser(userId: string) {
  return useQuery({
    queryKey: ["users", userId],
    queryFn: async () => {
      const res = await api.get<UserEnvelope>(`/users/${userId}`);
      return res.user;
    },
    enabled: !!userId,
    staleTime: 30 * 1000,
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: CreateUserInput) => {
      const res = await api.post<UserEnvelope>("/users", data);
      return res.user;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUserQuota() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      quota_limit,
      quota_source,
    }: {
      id: string;
      quota_limit: number;
      quota_source?: "default" | "custom";
    }) => api.patch<IDStatusEnvelope>(`/users/${id}/quota`, { quota_limit, quota_source }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["users", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUserStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: UserStatus }) =>
      api.patch<IDStatusEnvelope>(`/users/${id}/status`, { status }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["users", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUserRole() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, role }: { id: string; role: UserRole }) =>
      api.patch<IDStatusEnvelope>(`/users/${id}/role`, { role }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["users", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUserPasswordHash() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, password_hash }: { id: string; password_hash: string }) =>
      api.patch<IDStatusEnvelope>(`/users/${id}/password-hash`, { password_hash }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["users", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}
