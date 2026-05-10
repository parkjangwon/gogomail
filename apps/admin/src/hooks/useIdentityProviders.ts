import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";

export interface DatabaseConfig {
  enabled: boolean;
  user_table: string;
  email_column: string;
  password_column: string;
}

export interface LDAPConfig {
  enabled: boolean;
  server_url: string;
  bind_dn: string;
  bind_password: string;
  base_dn: string;
  user_filter: string;
  sync_enabled: boolean;
}

export interface RDBMSConfig {
  enabled: boolean;
  connection_string: string;
  user_query: string;
  sync_enabled: boolean;
}

export interface IdentityProviderConfig {
  company_id: string;
  database: DatabaseConfig;
  ldap: LDAPConfig;
  rdbms: RDBMSConfig;
}

export function useIdentityProviders(companyId: string) {
  return useQuery({
    queryKey: ["identity-providers", companyId],
    queryFn: () => api.get<IdentityProviderConfig>(`/identity-providers/${companyId}`),
    enabled: !!companyId,
    staleTime: 5 * 60 * 1000,
  });
}

export function useUpdateDatabaseConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      companyId,
      config,
    }: {
      companyId: string;
      config: DatabaseConfig;
    }) => api.put(`/identity-providers/${companyId}/database`, config),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["identity-providers", variables.companyId],
      });
    },
  });
}

export function useUpdateLDAPConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      companyId,
      config,
    }: {
      companyId: string;
      config: LDAPConfig;
    }) => api.put(`/identity-providers/${companyId}/ldap`, config),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["identity-providers", variables.companyId],
      });
    },
  });
}

export function useUpdateRDBMSConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      companyId,
      config,
    }: {
      companyId: string;
      config: RDBMSConfig;
    }) => api.put(`/identity-providers/${companyId}/rdbms`, config),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["identity-providers", variables.companyId],
      });
    },
  });
}

export function useSyncLDAP() {
  return useMutation({
    mutationFn: (companyId: string) =>
      api.post(`/ldap-sync/${companyId}/trigger`, {}),
  });
}

export function useSyncRDBMS() {
  return useMutation({
    mutationFn: (companyId: string) =>
      api.post(`/rdbms-sync/${companyId}/trigger`, {}),
  });
}
