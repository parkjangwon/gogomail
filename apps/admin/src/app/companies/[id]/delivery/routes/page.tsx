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

interface DeliveryRoute {
  id: string;
  destination: string;
  priority: number;
  status: string;
  message_count: number;
  created_at: string;
}

export default function DeliveryRoutesPage() {
  const { t } = useI18n();
  const [routes, setRoutes] = useState<DeliveryRoute[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchRoutes();
  }, []);

  const fetchRoutes = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/delivery-routes?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setRoutes(data.routes || []);
      }
    } catch (error) {
      console.error('Failed to fetch delivery routes:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredRoutes = routes.filter(r =>
    r.destination.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.routes.title')}</Header>}>
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
          description="Configure mail delivery routes"
          actions={
            <Button variant="primary" disabled>
              {t('pages.routes.create_route')}
            </Button>
          }
        >
          Delivery Routes
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Destination',
              cell: (item: DeliveryRoute) => item.destination,
              width: '30%',
            },
            {
              header: 'Priority',
              cell: (item: DeliveryRoute) => item.priority,
              width: '15%',
            },
            {
              header: 'Status',
              cell: (item: DeliveryRoute) => (
                <Badge color={item.status === 'active' ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '15%',
            },
            {
              header: 'Messages',
              cell: (item: DeliveryRoute) => item.message_count.toLocaleString(),
              width: '15%',
            },
            {
              header: 'Created',
              cell: (item: DeliveryRoute) => new Date(item.created_at).toLocaleDateString(),
              width: '25%',
            },
          ]}
          items={filteredRoutes}
          header={<Header variant="h2" counter={`(${filteredRoutes.length})`}>Routes</Header>}
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
