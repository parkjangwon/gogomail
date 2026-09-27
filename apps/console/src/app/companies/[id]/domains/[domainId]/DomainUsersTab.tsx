'use client';
import { DataTable } from '@/components/DataTable';
import {
  Header,
  Box,
  SpaceBetween,
  Badge,
  StatusIndicator,
  Button,
  Alert,
} from '@cloudscape-design/components';
import { useRouter } from 'next/navigation';
import { formatDate } from '@/lib/format';
import { User } from './domainDetailTypes';

interface Props {
  users: User[];
  hasMore?: boolean;
  companyId: string;
  domainName: string;
  usersError?: string;
  onRetry?: () => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainUsersTab({ users, hasMore, companyId, domainName, usersError, onRetry, t }: Props) {
  const router = useRouter();
  const counterLabel = hasMore ? `(${users.length}+)` : `(${users.length})`;

  if (usersError) {
    return (
      <Alert
        type="error"
        header={t('pages.domain_detail.users_load_error', 'Failed to load users for this domain.')}
        action={onRetry ? <Button onClick={onRetry}>{t('common.retry', 'Retry')}</Button> : undefined}
      >
        {usersError}
      </Alert>
    );
  }

  return (
    <DataTable
      columnDefinitions={[
        { header: t('pages.domain_detail.username'), cell: (u: User) => u.username, width: '30%' },
        { header: t('pages.domain_detail.display_name'), cell: (u: User) => u.display_name, width: '25%' },
        { header: t('pages.domain_detail.status'), cell: (u: User) => <Badge color={u.status === 'active' ? 'green' : 'grey'}>{u.status}</Badge>, width: '15%' },
        {
          header: t('pages.domain_detail.storage_col'),
          cell: (u: User) => u.quota_limit > 0
            ? `${(u.quota_used / 1073741824).toFixed(1)} / ${(u.quota_limit / 1073741824).toFixed(1)} GB`
            : `${(u.quota_used / 1073741824).toFixed(1)} GB`,
          width: '20%',
        },
        { header: t('pages.domain_detail.joined'), cell: (u: User) => formatDate(u.created_at), width: '10%' },
      ]}
      items={users}
      header={
        <Header variant="h2" counter={counterLabel} actions={<Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>{t('pages.domain_detail.add_user')}</Button>}>
          {t('pages.domain_detail.users_in')} {domainName}
        </Header>
      }
      empty={
        <Box textAlign="center" padding="l">
          <SpaceBetween size="m" alignItems="center">
            <StatusIndicator type="info">{t('pages.domain_detail.no_users')}</StatusIndicator>
            <Button variant="primary" onClick={() => router.push(`/companies/${companyId}/users`)}>{t('pages.domain_detail.create_first_user')}</Button>
          </SpaceBetween>
        </Box>
      }
    />
  );
}
