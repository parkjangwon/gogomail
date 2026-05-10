"use client";

import { useState, useEffect } from "react";
import { useIdentityProviders, useUpdateDatabaseConfig, useUpdateLDAPConfig, useUpdateRDBMSConfig, useSyncLDAP, useSyncRDBMS } from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import Tabs from "@cloudscape-design/components/tabs";
import FormField from "@cloudscape-design/components/form-field";
import Input from "@cloudscape-design/components/input";
import Button from "@cloudscape-design/components/button";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Checkbox from "@cloudscape-design/components/checkbox";
import Spinner from "@cloudscape-design/components/spinner";
import Alert from "@cloudscape-design/components/alert";

const DEMO_COMPANY_ID = "demo-company";

export default function IdentityProvidersPage() {
  const { data: config, isLoading } = useIdentityProviders(DEMO_COMPANY_ID);
  const [activeTab, setActiveTab] = useState("database");

  const updateDB = useUpdateDatabaseConfig();
  const updateLDAP = useUpdateLDAPConfig();
  const updateRDBMS = useUpdateRDBMSConfig();
  const syncLDAP = useSyncLDAP();
  const syncRDBMS = useSyncRDBMS();

  // Database state
  const [dbEnabled, setDbEnabled] = useState(false);
  const [dbUserTable, setDbUserTable] = useState("users");
  const [dbEmailCol, setDbEmailCol] = useState("email");
  const [dbPasswordCol, setDbPasswordCol] = useState("password");

  // LDAP state
  const [ldapEnabled, setLdapEnabled] = useState(false);
  const [ldapUrl, setLdapUrl] = useState("");
  const [ldapBindDN, setLdapBindDN] = useState("");
  const [ldapBindPass, setLdapBindPass] = useState("");
  const [ldapBaseDN, setLdapBaseDN] = useState("");
  const [ldapFilter, setLdapFilter] = useState("(uid={0})");
  const [ldapSync, setLdapSync] = useState(false);

  // RDBMS state
  const [rdbmsEnabled, setRdbmsEnabled] = useState(false);
  const [rdbmsConnStr, setRdbmsConnStr] = useState("");
  const [rdbmsUserQuery, setRdbmsUserQuery] = useState("");
  const [rdbmsSync, setRdbmsSync] = useState(false);

  useEffect(() => {
    if (config) {
      setDbEnabled(config.database.enabled);
      setDbUserTable(config.database.user_table);
      setDbEmailCol(config.database.email_column);
      setDbPasswordCol(config.database.password_column);

      setLdapEnabled(config.ldap.enabled);
      setLdapUrl(config.ldap.server_url);
      setLdapBindDN(config.ldap.bind_dn);
      setLdapBindPass(config.ldap.bind_password);
      setLdapBaseDN(config.ldap.base_dn);
      setLdapFilter(config.ldap.user_filter);
      setLdapSync(config.ldap.sync_enabled);

      setRdbmsEnabled(config.rdbms.enabled);
      setRdbmsConnStr(config.rdbms.connection_string);
      setRdbmsUserQuery(config.rdbms.user_query);
      setRdbmsSync(config.rdbms.sync_enabled);
    }
  }, [config]);

  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Identity Providers Configuration</Header>}
      >
        {isLoading ? (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        ) : (
          <Tabs
            activeTabId={activeTab}
            onChange={(e) => setActiveTab(e.detail.activeTabId)}
            tabs={[
              {
                label: "Database",
                id: "database",
                content: (
                  <Box padding="l">
                    <SpaceBetween direction="vertical" size="l">
                      {updateDB.isSuccess && (
                        <Alert type="success">Database config updated</Alert>
                      )}
                      {updateDB.isError && (
                        <Alert type="error">Failed to update database config</Alert>
                      )}

                      <Checkbox
                        checked={dbEnabled}
                        onChange={(e) => setDbEnabled(e.detail.checked)}
                      >
                        Enable Database Authentication
                      </Checkbox>

                      {dbEnabled && (
                        <SpaceBetween direction="vertical" size="s">
                          <FormField label="User Table">
                            <Input
                              value={dbUserTable}
                              onChange={(e) => setDbUserTable(e.detail.value)}
                              placeholder="users"
                            />
                          </FormField>

                          <FormField label="Email Column">
                            <Input
                              value={dbEmailCol}
                              onChange={(e) => setDbEmailCol(e.detail.value)}
                              placeholder="email"
                            />
                          </FormField>

                          <FormField label="Password Column">
                            <Input
                              value={dbPasswordCol}
                              onChange={(e) => setDbPasswordCol(e.detail.value)}
                              placeholder="password"
                            />
                          </FormField>

                          <Button
                            variant="primary"
                            loading={updateDB.isPending}
                            onClick={() =>
                              updateDB.mutate({
                                companyId: DEMO_COMPANY_ID,
                                config: {
                                  enabled: dbEnabled,
                                  user_table: dbUserTable,
                                  email_column: dbEmailCol,
                                  password_column: dbPasswordCol,
                                },
                              })
                            }
                          >
                            Save Database Config
                          </Button>
                        </SpaceBetween>
                      )}
                    </SpaceBetween>
                  </Box>
                ),
              },
              {
                label: "LDAP",
                id: "ldap",
                content: (
                  <Box padding="l">
                    <SpaceBetween direction="vertical" size="l">
                      {updateLDAP.isSuccess && (
                        <Alert type="success">LDAP config updated</Alert>
                      )}
                      {updateLDAP.isError && (
                        <Alert type="error">Failed to update LDAP config</Alert>
                      )}

                      <Checkbox
                        checked={ldapEnabled}
                        onChange={(e) => setLdapEnabled(e.detail.checked)}
                      >
                        Enable LDAP Authentication
                      </Checkbox>

                      {ldapEnabled && (
                        <SpaceBetween direction="vertical" size="s">
                          <FormField label="Server URL">
                            <Input
                              value={ldapUrl}
                              onChange={(e) => setLdapUrl(e.detail.value)}
                              placeholder="ldap://ldap.example.com:389"
                            />
                          </FormField>

                          <FormField label="Bind DN">
                            <Input
                              value={ldapBindDN}
                              onChange={(e) => setLdapBindDN(e.detail.value)}
                              placeholder="cn=admin,dc=example,dc=com"
                            />
                          </FormField>

                          <FormField label="Bind Password">
                            <Input
                              type="password"
                              value={ldapBindPass}
                              onChange={(e) => setLdapBindPass(e.detail.value)}
                            />
                          </FormField>

                          <FormField label="Base DN">
                            <Input
                              value={ldapBaseDN}
                              onChange={(e) => setLdapBaseDN(e.detail.value)}
                              placeholder="dc=example,dc=com"
                            />
                          </FormField>

                          <FormField label="User Filter">
                            <Input
                              value={ldapFilter}
                              onChange={(e) => setLdapFilter(e.detail.value)}
                              placeholder="(uid={0})"
                            />
                          </FormField>

                          <Checkbox
                            checked={ldapSync}
                            onChange={(e) => setLdapSync(e.detail.checked)}
                          >
                            Enable Periodic Sync
                          </Checkbox>

                          <SpaceBetween direction="horizontal" size="xs">
                            <Button
                              variant="primary"
                              loading={updateLDAP.isPending}
                              onClick={() =>
                                updateLDAP.mutate({
                                  companyId: DEMO_COMPANY_ID,
                                  config: {
                                    enabled: ldapEnabled,
                                    server_url: ldapUrl,
                                    bind_dn: ldapBindDN,
                                    bind_password: ldapBindPass,
                                    base_dn: ldapBaseDN,
                                    user_filter: ldapFilter,
                                    sync_enabled: ldapSync,
                                  },
                                })
                              }
                            >
                              Save LDAP Config
                            </Button>

                            <Button
                              loading={syncLDAP.isPending}
                              onClick={() => syncLDAP.mutate(DEMO_COMPANY_ID)}
                            >
                              Sync Now
                            </Button>
                          </SpaceBetween>
                        </SpaceBetween>
                      )}
                    </SpaceBetween>
                  </Box>
                ),
              },
              {
                label: "RDBMS",
                id: "rdbms",
                content: (
                  <Box padding="l">
                    <SpaceBetween direction="vertical" size="l">
                      {updateRDBMS.isSuccess && (
                        <Alert type="success">RDBMS config updated</Alert>
                      )}
                      {updateRDBMS.isError && (
                        <Alert type="error">Failed to update RDBMS config</Alert>
                      )}

                      <Checkbox
                        checked={rdbmsEnabled}
                        onChange={(e) => setRdbmsEnabled(e.detail.checked)}
                      >
                        Enable External RDBMS Authentication
                      </Checkbox>

                      {rdbmsEnabled && (
                        <SpaceBetween direction="vertical" size="s">
                          <FormField label="Connection String">
                            <Input
                              value={rdbmsConnStr}
                              onChange={(e) => setRdbmsConnStr(e.detail.value)}
                              placeholder="postgresql://user:pass@host:5432/db"
                            />
                          </FormField>

                          <FormField label="User Query">
                            <Input
                              value={rdbmsUserQuery}
                              onChange={(e) => setRdbmsUserQuery(e.detail.value)}
                              placeholder="SELECT email, name FROM users WHERE id = $1"
                            />
                          </FormField>

                          <Checkbox
                            checked={rdbmsSync}
                            onChange={(e) => setRdbmsSync(e.detail.checked)}
                          >
                            Enable Periodic Sync
                          </Checkbox>

                          <SpaceBetween direction="horizontal" size="xs">
                            <Button
                              variant="primary"
                              loading={updateRDBMS.isPending}
                              onClick={() =>
                                updateRDBMS.mutate({
                                  companyId: DEMO_COMPANY_ID,
                                  config: {
                                    enabled: rdbmsEnabled,
                                    connection_string: rdbmsConnStr,
                                    user_query: rdbmsUserQuery,
                                    sync_enabled: rdbmsSync,
                                  },
                                })
                              }
                            >
                              Save RDBMS Config
                            </Button>

                            <Button
                              loading={syncRDBMS.isPending}
                              onClick={() => syncRDBMS.mutate(DEMO_COMPANY_ID)}
                            >
                              Sync Now
                            </Button>
                          </SpaceBetween>
                        </SpaceBetween>
                      )}
                    </SpaceBetween>
                  </Box>
                ),
              },
            ]}
          />
        )}
      </Container>
    </Box>
  );
}
