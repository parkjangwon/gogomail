'use client';

import {
  ContentLayout,
  Header,
  Container,
  Box,
  Spinner,
  SpaceBetween,
  ColumnLayout,
  StatusIndicator,
  CopyToClipboard,
  Alert,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';

interface SCIMStatus {
  endpoint: string;
  supported_resources: string[];
  domain_id: string;
  user_count: number;
  status: string;
}

export default function SCIMStatusPage() {
  const params = useParams();
  const companyId = params?.id as string;
  const [data, setData] = useState<SCIMStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!companyId) return;
    fetch(`/api/admin/companies/${companyId}/scim/status`, { credentials: 'include' })
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(setData)
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false));
  }, [companyId]);

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">SCIM Provisioning</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  if (error || !data) {
    return (
      <ContentLayout header={<Header variant="h1">SCIM Provisioning</Header>}>
        <StatusIndicator type="error">{error || 'Failed to load SCIM status'}</StatusIndicator>
      </ContentLayout>
    );
  }

  const backendBase = process.env.NEXT_PUBLIC_GOGOMAIL_BACKEND_URL || 'http://localhost:8080';
  const scimEndpointUrl = `${backendBase}${data.endpoint}`;

  return (
    <ContentLayout
      header={
        <Header variant="h1" description="System for Cross-domain Identity Management — provision users from your identity provider.">
          SCIM Provisioning
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Alert type="info" header="Setup Instructions">
          Configure your identity provider (Okta, Azure AD, Google Workspace, etc.) with the SCIM endpoint URL and a bearer token generated from <strong>Security → API Keys</strong>.
        </Alert>

        <Container header={<Header variant="h2">Connection Details</Header>}>
          <SpaceBetween size="m">
            <ColumnLayout columns={2} variant="text-grid">
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Status</Box>
                <StatusIndicator type={data.status === 'active' ? 'success' : 'warning'}>
                  {data.status === 'active' ? 'Active' : data.status}
                </StatusIndicator>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Supported Resources</Box>
                <Box>{data.supported_resources?.join(', ') || '—'}</Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Domain ID</Box>
                <Box fontSize="body-s"><code>{data.domain_id || '—'}</code></Box>
              </SpaceBetween>
              <SpaceBetween size="xxs">
                <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">Provisioned Users</Box>
                <Box fontSize="display-l" fontWeight="bold">{data.user_count}</Box>
              </SpaceBetween>
            </ColumnLayout>

            <SpaceBetween size="xxs">
              <Box fontWeight="bold" fontSize="body-s" color="text-body-secondary">SCIM Endpoint URL</Box>
              <CopyToClipboard
                copyButtonText="Copy"
                copySuccessText="Copied"
                copyErrorText="Failed to copy"
                textToCopy={scimEndpointUrl}
                variant="inline"
              />
              <Box fontSize="body-s" color="text-body-secondary"><code>{scimEndpointUrl}</code></Box>
            </SpaceBetween>
          </SpaceBetween>
        </Container>

        <Container header={<Header variant="h2">Supported Operations</Header>}>
          <ColumnLayout columns={3} variant="text-grid">
            {[
              { op: 'GET /Users', desc: 'List provisioned users' },
              { op: 'POST /Users', desc: 'Create a new user' },
              { op: 'GET /Users/{id}', desc: 'Retrieve user details' },
              { op: 'PUT /Users/{id}', desc: 'Replace user attributes' },
              { op: 'PATCH /Users/{id}', desc: 'Partial update (activate/deactivate)' },
              { op: 'DELETE /Users/{id}', desc: 'Deprovision user' },
            ].map(({ op, desc }) => (
              <SpaceBetween key={op} size="xxs">
                <Box fontSize="body-s" fontWeight="bold"><code>{op}</code></Box>
                <Box color="text-body-secondary" fontSize="body-s">{desc}</Box>
              </SpaceBetween>
            ))}
          </ColumnLayout>
        </Container>
      </SpaceBetween>
    </ContentLayout>
  );
}
