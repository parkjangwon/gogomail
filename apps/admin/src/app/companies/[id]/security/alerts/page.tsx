'use client';

import {
  ContentLayout,
  Header,
  Table,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';

interface AlertRule {
  id: string;
  name: string;
  condition: string;
  action: string;
  enabled: boolean;
  created_at: string;
}

export default function AlertRulesPage() {
  const { t: _unused } = useI18n(); _unused;
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchAlertRules();
  }, []);

  const fetchAlertRules = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/alert-rules?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRules(data.rules || []);
      }
    } catch (error) {
      console.error('Failed to fetch alert rules:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredRules = rules.filter(r =>
    r.name.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Alert Rules</Header>}>
        <Box textAlign="center" padding="xl">
          <Spinner />
        </Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description="Configure alert rules and notifications"
          actions={
            <Button variant="primary" disabled>
              + Create Rule
            </Button>
          }
        >
          Alert Rules
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Name',
              cell: (item: AlertRule) => item.name,
              width: '25%',
            },
            {
              header: 'Condition',
              cell: (item: AlertRule) => item.condition,
              width: '25%',
            },
            {
              header: 'Action',
              cell: (item: AlertRule) => item.action,
              width: '20%',
            },
            {
              header: 'Enabled',
              cell: (item: AlertRule) => (
                <Badge color={item.enabled ? 'green' : 'grey'}>
                  {item.enabled ? 'Enabled' : 'Disabled'}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Created',
              cell: (item: AlertRule) => new Date(item.created_at).toLocaleDateString(),
              width: '15%',
            },
          ]}
          items={filteredRules}
          header={<Header variant="h2" counter={`(${filteredRules.length})`}>Rules</Header>}
          filter={
            <TextFilter
              filteringText={filter}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
        />
      </SpaceBetween>
    </ContentLayout>
  );
}
