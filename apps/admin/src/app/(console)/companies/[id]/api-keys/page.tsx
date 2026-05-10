'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Button,
  Table,
  Modal,
  Box,
  Spinner,
  Alert,
  Checkbox,
  Textarea,
} from '@cloudscape-design/components';
import { useApiKeys, useCreateApiKey, useRotateApiKey, useDeleteApiKey } from '@/hooks/useApiKeys';

export default function ApiKeysPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [showModal, setShowModal] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    expires_in_days: '',
    cidr_allowlist: '',
    cidr_enabled: false,
  });

  // For demo, using domain ID as company ID (should be selected from dropdown)
  const domainId = companyId;

  const keysQuery = useApiKeys(domainId);
  const createMutation = useCreateApiKey();
  const rotateMutation = useRotateApiKey();
  const deleteMutation = useDeleteApiKey();

  const keys = keysQuery.data || [];

  const handleCreate = async () => {
    const cidrList = formData.cidr_allowlist
      .split('\n')
      .map((c) => c.trim())
      .filter((c) => c);

    await createMutation.mutateAsync({
      domainId,
      data: {
        name: formData.name,
        expires_in_days: formData.expires_in_days ? parseInt(formData.expires_in_days) : undefined,
        cidr_allowlist: formData.cidr_enabled ? cidrList : undefined,
      },
    });
    setShowModal(false);
    setFormData({ name: '', expires_in_days: '', cidr_allowlist: '', cidr_enabled: false });
  };

  const handleRotate = async (keyId: string) => {
    if (confirm('Rotate this API key? The old key will be revoked.')) {
      await rotateMutation.mutateAsync({ domainId, keyId });
    }
  };

  const handleDelete = async (keyId: string) => {
    if (confirm('Delete this API key? This cannot be undone.')) {
      await deleteMutation.mutateAsync({ domainId, keyId });
    }
  };

  const columns = [
    { header: 'Name', cell: (item: any) => item.name, width: 150 },
    { header: 'Prefix', cell: (item: any) => item.prefix },
    {
      header: 'Created',
      cell: (item: any) => new Date(item.created_at).toLocaleDateString(),
    },
    {
      header: 'Expires',
      cell: (item: any) => (item.expires_at ? new Date(item.expires_at).toLocaleDateString() : 'Never'),
    },
    {
      header: 'Requests',
      cell: (item: any) => item.request_count.toLocaleString(),
    },
    {
      header: 'Last Used',
      cell: (item: any) => (item.last_used_at ? new Date(item.last_used_at).toLocaleString() : '-'),
    },
    {
      header: 'Actions',
      cell: (item: any) => (
        <SpaceBetween direction="horizontal" size="xs">
          <Button
            onClick={() => handleRotate(item.id)}
            loading={rotateMutation.isPending}
          >
            Rotate
          </Button>
          <Button
            onClick={() => handleDelete(item.id)}
            loading={deleteMutation.isPending}
          >
            Delete
          </Button>
        </SpaceBetween>
      ),
    },
  ];

  return (
    <Container header={<Header>API Key Management</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button
            variant="primary"
            onClick={() => setShowModal(true)}
          >
            Create New Key
          </Button>
        </Box>

        {keysQuery.isPending ? (
          <Spinner />
        ) : keys.length > 0 ? (
          <Table columnDefinitions={columns} items={keys} variant="full-page" />
        ) : (
          <Alert>No API keys created yet</Alert>
        )}

        {/* Create Key Modal */}
        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          header="Create New API Key"
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button variant="link" onClick={() => setShowModal(false)}>
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  onClick={handleCreate}
                  loading={createMutation.isPending}
                >
                  Create
                </Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="m">
            <FormField label="Key Name">
              <Input
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.detail.value })
                }
                placeholder="e.g., Production API, Integration Test"
              />
            </FormField>

            <FormField label="Expiration (days)">
              <Input
                type="number"
                value={formData.expires_in_days}
                onChange={(e) =>
                  setFormData({ ...formData, expires_in_days: e.detail.value })
                }
                placeholder="Leave empty for no expiration"
              />
            </FormField>

            <Checkbox
              checked={formData.cidr_enabled}
              onChange={(e) =>
                setFormData({ ...formData, cidr_enabled: e.detail.checked })
              }
            >
              Restrict by IP (CIDR)
            </Checkbox>

            {formData.cidr_enabled && (
              <FormField label="Allowed IPs (CIDR format, one per line)">
                <Textarea
                  value={formData.cidr_allowlist}
                  onChange={(e) =>
                    setFormData({ ...formData, cidr_allowlist: e.detail.value })
                  }
                  placeholder="192.168.1.0/24&#10;10.0.0.0/8"
                />
              </FormField>
            )}
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
