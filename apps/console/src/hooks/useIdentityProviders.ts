import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

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
    queryFn: async (): Promise<IdentityProviderConfig> => ({
      company_id: companyId,
      database: { enabled: true, user_table: "users", email_column: "address", password_column: "password_hash" },
      ldap: { enabled: false, server_url: "", bind_dn: "", bind_password: "", base_dn: "", user_filter: "", sync_enabled: false },
      rdbms: { enabled: false, connection_string: "", user_query: "", sync_enabled: false },
    }),
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
    }) => {
      void companyId;
      void config;
      throw new Error("Database identity provider updates are not supported by the current Admin API contract");
    },
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
    }) => {
      void companyId;
      void config;
      throw new Error("LDAP identity provider updates are not supported by the current Admin API contract");
    },
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
    }) => {
      void companyId;
      void config;
      throw new Error("RDBMS identity provider updates are not supported by the current Admin API contract");
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["identity-providers", variables.companyId],
      });
    },
  });
}

export function useSyncLDAP() {
  return useMutation({
    mutationFn: (companyId: string) => {
      void companyId;
      throw new Error("LDAP sync is not supported by the current Admin API contract");
    },
  });
}

export function useSyncRDBMS() {
  return useMutation({
    mutationFn: (companyId: string) => {
      void companyId;
      throw new Error("RDBMS sync is not supported by the current Admin API contract");
    },
  });
}
