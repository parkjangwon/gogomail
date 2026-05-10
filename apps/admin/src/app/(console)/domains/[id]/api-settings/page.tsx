"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import {
  useAPISettings,
  useUpdateAPISettings,
  useAPIKeys,
  useCreateAPIKey,
  useDeleteAPIKey,
  useRotateAPIKey,
  type APIKey,
} from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import FormField from "@cloudscape-design/components/form-field";
import Input from "@cloudscape-design/components/input";
import Button from "@cloudscape-design/components/button";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Checkbox from "@cloudscape-design/components/checkbox";
import Spinner from "@cloudscape-design/components/spinner";
import Alert from "@cloudscape-design/components/alert";
import Table from "@cloudscape-design/components/table";
import Badge from "@cloudscape-design/components/badge";
import Modal from "@cloudscape-design/components/modal";
import Textarea from "@cloudscape-design/components/textarea";

export default function APISettingsPage() {
  const params = useParams();
  const domainId = Array.isArray(params.id) ? params.id[0] : params.id;

  const settingsQuery = useAPISettings(domainId);
  const keysQuery = useAPIKeys(domainId);
  const updateSettings = useUpdateAPISettings();
  const createKey = useCreateAPIKey();
  const deleteKey = useDeleteAPIKey();
  const rotateKey = useRotateAPIKey();

  // Settings form state
  const [rateLimitRps, setRateLimitRps] = useState("100");
  const [rateLimitBps, setRateLimitBps] = useState("0");
  const [cidrEnabled, setCidrEnabled] = useState(false);
  const [cidrList, setCidrList] = useState("");
  const [requireAPIKey, setRequireAPIKey] = useState(true);
  const [formInitialized, setFormInitialized] = useState(false);

  // API Key creation state
  const [showCreateKeyModal, setShowCreateKeyModal] = useState(false);
  const [keyName, setKeyName] = useState("");
  const [showSecret, setShowSecret] = useState(false);
  const [createdSecret, setCreatedSecret] = useState("");

  // Initialize form from loaded settings
  if (settingsQuery.data && !formInitialized) {
    setRateLimitRps(String(settingsQuery.data.rate_limit_rps));
    setRateLimitBps(String(settingsQuery.data.rate_limit_bps));
    setCidrEnabled(settingsQuery.data.cidr_allowlist_enabled);
    setCidrList((settingsQuery.data.cidr_allowlist || []).join("\n"));
    setRequireAPIKey(settingsQuery.data.require_api_key);
    setFormInitialized(true);
  }

  const handleSaveSettings = async () => {
    const cidrAllowlist = cidrEnabled
      ? cidrList
          .split("\n")
          .map((ip) => ip.trim())
          .filter((ip) => ip.length > 0)
      : [];

    updateSettings.mutate({
      domainId,
      data: {
        domain_id: domainId,
        rate_limit_rps: parseInt(rateLimitRps, 10),
        rate_limit_bps: parseInt(rateLimitBps, 10),
        cidr_allowlist_enabled: cidrEnabled,
        cidr_allowlist: cidrAllowlist,
        require_api_key: requireAPIKey,
      },
    });
  };

  const handleCreateKey = async () => {
    if (!keyName.trim()) return;

    createKey.mutate(
      {
        domainId,
        name: keyName,
        created_by: "current-user",
      },
      {
        onSuccess: (response) => {
          setCreatedSecret(response.secret);
          setShowSecret(true);
          setKeyName("");
        },
      }
    );
  };

  const handleDeleteKey = (keyId: string) => {
    deleteKey.mutate({ domainId, keyId });
  };

  const handleRotateKey = (keyId: string) => {
    rotateKey.mutate({ domainId, keyId });
  };

  const isLoading = settingsQuery.isLoading || keysQuery.isLoading;

  return (
    <Box padding="l">
      <SpaceBetween direction="vertical" size="l">
        {/* API Settings Section */}
        <Container
          header={
            <Header variant="h2">API Settings</Header>
          }
        >
          {isLoading ? (
            <Box textAlign="center" padding="l">
              <Spinner />
            </Box>
          ) : (
            <SpaceBetween direction="vertical" size="l">
              {updateSettings.isSuccess && (
                <Alert type="success">Settings updated successfully</Alert>
              )}
              {updateSettings.isError && (
                <Alert type="error">Failed to update settings</Alert>
              )}

              <FormField label="Rate Limiting - Requests per Second (RPS)">
                <Input
                  type="number"
                  value={rateLimitRps}
                  onChange={(e) => setRateLimitRps(e.detail.value)}
                  inputMode="numeric"
                />
              </FormField>

              <FormField label="Rate Limiting - Bytes per Second (BPS)">
                <Input
                  type="number"
                  value={rateLimitBps}
                  onChange={(e) => setRateLimitBps(e.detail.value)}
                  inputMode="numeric"
                />
              </FormField>

              <FormField>
                <Checkbox
                  checked={cidrEnabled}
                  onChange={(e) => setCidrEnabled(e.detail.checked)}
                >
                  Enable CIDR IP Allowlist
                </Checkbox>
              </FormField>

              {cidrEnabled && (
                <FormField label="CIDR Allowlist (one per line)">
                  <Textarea
                    value={cidrList}
                    onChange={(e) => setCidrList(e.detail.value)}
                    placeholder="192.168.1.0/24&#10;10.0.0.0/8"
                    rows={4}
                  />
                </FormField>
              )}

              <FormField>
                <Checkbox
                  checked={requireAPIKey}
                  onChange={(e) => setRequireAPIKey(e.detail.checked)}
                >
                  Require API Key for all API requests
                </Checkbox>
              </FormField>

              <Box>
                <Button
                  variant="primary"
                  loading={updateSettings.isPending}
                  onClick={handleSaveSettings}
                >
                  Save Settings
                </Button>
              </Box>
            </SpaceBetween>
          )}
        </Container>

        {/* API Keys Section */}
        <Container
          header={
            <Header
              variant="h2"
              actions={
                <Button
                  variant="primary"
                  onClick={() => setShowCreateKeyModal(true)}
                >
                  Create API Key
                </Button>
              }
            >
              API Keys
            </Header>
          }
        >
          {keysQuery.isLoading ? (
            <Box textAlign="center" padding="l">
              <Spinner />
            </Box>
          ) : keysQuery.data && keysQuery.data.length > 0 ? (
            <Table
              columnDefinitions={[
                {
                  id: "name",
                  header: "Name",
                  cell: (item: APIKey) => item.name,
                },
                {
                  id: "created_at",
                  header: "Created",
                  cell: (item: APIKey) =>
                    new Date(item.created_at).toLocaleDateString(),
                },
                {
                  id: "last_used_at",
                  header: "Last Used",
                  cell: (item: APIKey) =>
                    item.last_used_at
                      ? new Date(item.last_used_at).toLocaleDateString()
                      : "Never",
                },
                {
                  id: "status",
                  header: "Status",
                  cell: (item: APIKey) => (
                    <Badge color={item.is_active ? "green" : "red"}>
                      {item.is_active ? "Active" : "Inactive"}
                    </Badge>
                  ),
                },
                {
                  id: "actions",
                  header: "Actions",
                  cell: (item: APIKey) => (
                    <SpaceBetween direction="horizontal" size="xs">
                      <Button
                        variant="inline-link"
                        onClick={() => handleRotateKey(item.id)}
                        loading={rotateKey.isPending}
                      >
                        Rotate
                      </Button>
                      <Button
                        variant="inline-link"
                        onClick={() => handleDeleteKey(item.id)}
                        loading={deleteKey.isPending}
                      >
                        Delete
                      </Button>
                    </SpaceBetween>
                  ),
                },
              ]}
              items={keysQuery.data || []}
              empty={
                <Box textAlign="center" padding="xl" color="text-status-info">
                  No API keys. Create one to get started.
                </Box>
              }
            />
          ) : (
            <Box textAlign="center" padding="xl" color="text-status-info">
              No API keys. Create one to get started.
            </Box>
          )}
        </Container>
      </SpaceBetween>

      {/* Create API Key Modal */}
      <Modal
        onDismiss={() => {
          setShowCreateKeyModal(false);
          setKeyName("");
          setShowSecret(false);
          setCreatedSecret("");
        }}
        visible={showCreateKeyModal}
        header={showSecret ? "API Key Created" : "Create API Key"}
        footer={
          <Box float="right">
            {showSecret ? (
              <Button
                variant="primary"
                onClick={() => {
                  setShowCreateKeyModal(false);
                  setKeyName("");
                  setShowSecret(false);
                  setCreatedSecret("");
                }}
              >
                Done
              </Button>
            ) : (
              <SpaceBetween direction="horizontal" size="xs">
                <Button
                  onClick={() => setShowCreateKeyModal(false)}
                >
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  loading={createKey.isPending}
                  onClick={handleCreateKey}
                  disabled={!keyName.trim()}
                >
                  Create
                </Button>
              </SpaceBetween>
            )}
          </Box>
        }
      >
        {showSecret ? (
          <SpaceBetween direction="vertical" size="m">
            <Alert type="warning">
              Save this secret somewhere safe. You won't be able to see it again.
            </Alert>
            <FormField label="Secret">
              <Input
                value={createdSecret}
                readOnly
                disabled
              />
            </FormField>
            <Button
              onClick={() => {
                navigator.clipboard.writeText(createdSecret);
              }}
            >
              Copy Secret
            </Button>
          </SpaceBetween>
        ) : (
          <FormField label="Key Name">
            <Input
              value={keyName}
              onChange={(e) => setKeyName(e.detail.value)}
              placeholder="e.g., Production API Key"
            />
          </FormField>
        )}
      </Modal>
    </Box>
  );
}
