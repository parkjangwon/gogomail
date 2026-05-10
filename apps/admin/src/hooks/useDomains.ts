import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface Domain {
  id: string;
  company_id: string;
  name: string;
  verified: boolean;
  verification_token?: string;
  created_at: string;
  updated_at: string;
}

export function useDomains(companyId?: string) {
  return useQuery({
    queryKey: ["domains", companyId],
    queryFn: async () => {
      const params = companyId ? { company_id: companyId } : undefined;
      return api.get<Domain[]>("/domains", { params });
    },
    staleTime: 30 * 1000,
  });
}

export function useDomain(domainId: string) {
  return useQuery({
    queryKey: ["domains", domainId],
    queryFn: () => api.get<Domain>(`/domains/${domainId}`),
    enabled: !!domainId,
    staleTime: 30 * 1000,
  });
}

export function useCreateDomain() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Omit<Domain, "id" | "created_at" | "updated_at" | "verified">) =>
      api.post<Domain>("/domains", data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
  });
}

export function useUpdateDomain() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: Partial<Domain>;
    }) => api.put<Domain>(`/domains/${id}`, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["domains", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
  });
}

export function useDeleteDomain() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (domainId: string) => api.delete(`/domains/${domainId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
  });
}
