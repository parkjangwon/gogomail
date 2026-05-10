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
  Box,
  Spinner,
  Alert,
  Modal,
  Badge,
} from '@cloudscape-design/components';
import { useDomains, useCreateDomain, useDeleteDomain } from '@/hooks/useDomains';

export default function DomainsPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [showModal, setShowModal] = useState(false);
  const [domainName, setDomainName] = useState('');

  const domainsQuery = useDomains(companyId);
  const createMutation = useCreateDomain();
  const deleteMutation = useDeleteDomain();

  const domains = domainsQuery.data || [];

  const handleCreate = async () => {
    if (!domainName) return;
    await createMutation.mutateAsync({
      companyId,
      domain: {
        name: domainName,
        status: 'pending',
        is_primary: domains.length === 0,
        dkim_configured: false,
        spf_configured: false,
        dmarc_configured: false,
        company_id: companyId,
      } as any,
    });
    setShowModal(false);
    setDomainName('');
  };

  const handleDelete = async (domainId: string) => {
    if (confirm('Delete this domain?')) {
      await deleteMutation.mutateAsync({ companyId, domainId });
    }
  };

  const columns = [
    { header: 'Domain', cell: (item: any) => item.name, width: 200 },
    {
      header: 'Status',
      cell: (item: any) => (
        <Badge color={item.status === 'active' ? 'green' : 'red'}>
          {item.status.toUpperCase()}
        </Badge>
      ),
    },
    {
      header: 'Primary',
      cell: (item: any) => (item.is_primary ? '✓' : '-'),
    },
    {
      header: 'DKIM',
      cell: (item: any) => (item.dkim_configured ? '✓' : '-'),
    },
    {
      header: 'SPF',
      cell: (item: any) => (item.spf_configured ? '✓' : '-'),
    },
    {
      header: 'DMARC',
      cell: (item: any) => (item.dmarc_configured ? '✓' : '-'),
    },
    {
      header: 'Actions',
      cell: (item: any) => (
        <Button
          onClick={() => handleDelete(item.id)}
          loading={deleteMutation.isPending}
        >
          Delete
        </Button>
      ),
    },
  ];

  return (
    <Container header={<Header>Domain Management</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button
            variant="primary"
            onClick={() => setShowModal(true)}
          >
            Add Domain
          </Button>
        </Box>

        {domainsQuery.isPending ? (
          <Spinner />
        ) : domains.length > 0 ? (
          <Table columnDefinitions={columns} items={domains} variant="full-page" />
        ) : (
          <Alert>No domains configured</Alert>
        )}

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          header="Add Domain"
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
                  Add
                </Button>
              </SpaceBetween>
            </Box>
          }
        >
          <FormField label="Domain Name">
            <Input
              value={domainName}
              onChange={(e) => setDomainName(e.detail.value)}
              placeholder="example.com"
            />
          </FormField>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
