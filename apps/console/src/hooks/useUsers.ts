import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface User {
  id: string;
  email: string;
  name: string;
  role_id: string;
  company_id: string;
  created_at: string;
  updated_at: string;
}

export function useUsers(companyId?: string) {
  return useQuery({
    queryKey: ["users", companyId],
    queryFn: async () => {
      const params = companyId ? { company_id: companyId } : undefined;
      return api.get<User[]>("/users", { params });
    },
    staleTime: 30 * 1000,
  });
}

export function useUser(userId: string) {
  return useQuery({
    queryKey: ["users", userId],
    queryFn: () => api.get<User>(`/users/${userId}`),
    enabled: !!userId,
    staleTime: 30 * 1000,
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Omit<User, "id" | "created_at" | "updated_at">) =>
      api.post<User>("/users", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: Partial<User>;
    }) => api.put<User>(`/users/${id}`, data),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["users", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (userId: string) => api.delete(`/users/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}
