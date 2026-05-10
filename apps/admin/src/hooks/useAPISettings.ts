import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface APISettings {
  domain_id: string;
  rate_limit_rps: number;
  rate_limit_bps: number;
  cidr_allowlist_enabled: boolean;
  cidr_allowlist?: string[];
  require_api_key: boolean;
  updated_at: string;
  updated_by: string;
}

export interface APIKey {
  id: string;
  domain_id: string;
  name: string;
  created_by: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
  is_active: boolean;
}

export interface APIKeyCreateResponse {
  id: string;
  secret: string;
}

export interface APIKeyRotateResponse {
  status: string;
  secret: string;
}

export function useAPISettings(domainId: string) {
  return useQuery({
    queryKey: ["api-settings", domainId],
    queryFn: () => api.get<APISettings>(`/domains/${domainId}/api-settings`),
    enabled: !!domainId,
    staleTime: 30 * 1000,
  });
}

export function useUpdateAPISettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      domainId,
      data,
    }: {
      domainId: string;
      data: Omit<APISettings, "updated_at" | "updated_by">;
    }) => api.put<APISettings>(`/domains/${domainId}/api-settings`, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["api-settings", variables.domainId],
      });
    },
  });
}

export function useAPIKeys(domainId: string) {
  return useQuery({
    queryKey: ["api-keys", domainId],
    queryFn: async () => {
      const response = await api.get<{ keys: APIKey[] }>(
        `/domains/${domainId}/api-keys`
      );
      return response.keys;
    },
    enabled: !!domainId,
    staleTime: 30 * 1000,
  });
}

export function useCreateAPIKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      domainId,
      name,
      created_by,
    }: {
      domainId: string;
      name: string;
      created_by: string;
    }) =>
      api.post<APIKeyCreateResponse>(`/domains/${domainId}/api-keys`, {
        name,
        created_by,
      }),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["api-keys", variables.domainId],
      });
    },
  });
}

export function useDeleteAPIKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ domainId, keyId }: { domainId: string; keyId: string }) =>
      api.delete(`/domains/${domainId}/api-keys/${keyId}`),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["api-keys", variables.domainId],
      });
    },
  });
}

export function useRotateAPIKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ domainId, keyId }: { domainId: string; keyId: string }) =>
      api.post<APIKeyRotateResponse>(
        `/domains/${domainId}/api-keys/${keyId}/rotate`,
        {}
      ),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["api-keys", variables.domainId],
      });
    },
  });
}
