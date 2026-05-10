'use client';

import {
  ContentLayout,
  Header,
  Table,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface DriveNode {
  id: string;
  name: string;
  type: string;
  size_bytes: number;
  owner: string;
  created_at: string;
}

export default function DrivePage() {
  const [nodes, setNodes] = useState<DriveNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    fetchDriveNodes();
  }, []);

  const fetchDriveNodes = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/drive-nodes?limit=100', {
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        setNodes(data.nodes || []);
      }
    } catch (error) {
      console.error('Failed to fetch drive nodes:', error);
    } finally {
      setLoading(false);
    }
  };

  const filteredNodes = nodes.filter(n =>
    n.name.toLowerCase().includes(filter.toLowerCase()) ||
    n.owner.toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">Drive Storage</Header>}>
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
          description="Browse and manage drive storage nodes"
        >
          Drive Storage
        </Header>
      }
    >
      <SpaceBetween size="l">
        <Table
          columnDefinitions={[
            {
              header: 'Name',
              cell: (item: DriveNode) => item.name,
              width: '25%',
            },
            {
              header: 'Type',
              cell: (item: DriveNode) => item.type,
              width: '15%',
            },
            {
              header: 'Size',
              cell: (item: DriveNode) => `${(item.size_bytes / 1024 / 1024).toFixed(2)} MB`,
              width: '15%',
            },
            {
              header: 'Owner',
              cell: (item: DriveNode) => item.owner,
              width: '25%',
            },
            {
              header: 'Created',
              cell: (item: DriveNode) => new Date(item.created_at).toLocaleString(),
              width: '20%',
            },
          ]}
          items={filteredNodes}
          header={<Header variant="h2" counter={`(${filteredNodes.length})`}>Nodes</Header>}
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
