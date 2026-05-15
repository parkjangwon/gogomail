import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import type { components, operations } from "@gogomail/api-types";

export type APISettings = components["schemas"]["APISettings"];
export type APISettingsUpdateRequest = components["schemas"]["APISettingsUpdateRequest"];
export type APISettingsEnvelope = components["schemas"]["APISettingsEnvelope"];
export type APISettingsUpdateResponse =
  operations["updateAdminDomainAPISettings"]["responses"][200]["content"]["application/json"];

export function useAPISettings(domainId: string | undefined) {
  return useQuery({
    queryKey: ["domains", domainId, "api-settings"],
    queryFn: async () => {
      if (!domainId) return undefined;
      const res = await api.get<APISettingsEnvelope>(`/domains/${domainId}/api-settings`);
      return res.settings;
    },
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
      data: APISettingsUpdateRequest;
    }) => api.put<APISettingsUpdateResponse>(`/domains/${domainId}/api-settings`, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["domains", variables.domainId, "api-settings"],
      });
    },
  });
}

export type {
  ApiKey as APIKey,
  ApiKeyCreateEnvelope as APIKeyCreateResponse,
  ApiKeyRotateEnvelope as APIKeyRotateResponse,
} from "./useApiKeys";
export {
  useApiKeys as useAPIKeys,
  useCreateApiKey as useCreateAPIKey,
  useDeleteApiKey as useDeleteAPIKey,
  useRotateApiKey as useRotateAPIKey,
} from "./useApiKeys";
