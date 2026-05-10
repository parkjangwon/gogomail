'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Select,
  Button,
  Table,
  Box,
  Spinner,
  Alert,
  Checkbox,
  Modal,
} from '@cloudscape-design/components';
import { useSSOConfigs, useCreateSSOConfig } from '@/hooks/useSSOConfig';

export default function SSOConfigPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [showModal, setShowModal] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    provider_type: 'ldap',
    is_enabled: true,
  });

  const configsQuery = useSSOConfigs(companyId);
  const createMutation = useCreateSSOConfig();

  const configs = configsQuery.data || [];

  const handleCreate = async () => {
    await createMutation.mutateAsync({
      companyId,
      config: {
        name: formData.name,
        provider_type: formData.provider_type as any,
        is_enabled: formData.is_enabled,
        config: {},
        attribute_mapping: {},
        company_id: companyId,
      } as any,
    });
    setShowModal(false);
    setFormData({ name: '', provider_type: 'ldap', is_enabled: true });
  };

  const columns = [
    { header: 'Name', cell: (item: any) => item.name },
    { header: 'Provider', cell: (item: any) => item.provider_type.toUpperCase() },
    { header: 'Status', cell: (item: any) => (item.is_enabled ? 'Enabled' : 'Disabled') },
    {
      header: 'Created',
      cell: (item: any) => new Date(item.created_at).toLocaleDateString(),
    },
  ];

  return (
    <Container header={<Header>SSO & Identity Provider</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button
            variant="primary"
            onClick={() => setShowModal(true)}
          >
            Add SSO Provider
          </Button>
        </Box>

        {configsQuery.isPending ? (
          <Spinner />
        ) : configs.length > 0 ? (
          <Table columnDefinitions={columns} items={configs} variant="full-page" />
        ) : (
          <Alert>No SSO providers configured</Alert>
        )}

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          header="Add SSO Provider"
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
            <FormField label="Provider Name">
              <Input
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.detail.value })
                }
              />
            </FormField>
            <FormField label="Provider Type">
              <Select
                selectedOption={{
                  label: formData.provider_type.toUpperCase(),
                  value: formData.provider_type,
                }}
                options={[
                  { label: 'LDAP', value: 'ldap' },
                  { label: 'OIDC', value: 'oidc' },
                  { label: 'SAML', value: 'saml' },
                ]}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    provider_type: e.detail.selectedOption.value || 'ldap',
                  })
                }
              />
            </FormField>
            <Checkbox
              checked={formData.is_enabled}
              onChange={(e) =>
                setFormData({ ...formData, is_enabled: e.detail.checked })
              }
            >
              Enabled
            </Checkbox>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
