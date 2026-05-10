'use client';

import { useParams } from 'next/navigation';
import {
  Container,
  Header,
  SpaceBetween,
  Table,
  Spinner,
  Alert,
  Box,
} from '@cloudscape-design/components';
import { useOrganizationStructure } from '@/hooks/useOrganization';

interface TreeNode {
  id: string;
  name: string;
  type: string;
  children?: TreeNode[];
  member_count?: number;
}

function TreeNodeComponent({ node, level = 0 }: { node: TreeNode; level?: number }) {
  return (
    <div style={{ marginLeft: `${level * 20}px`, marginBottom: '8px' }}>
      <Box>
        {node.type === 'user' ? '👤' : '📁'} <strong>{node.name}</strong>
        {node.member_count && ` (${node.member_count} members)`}
      </Box>
      {node.children?.map((child) => (
        <TreeNodeComponent key={child.id} node={child} level={level + 1} />
      ))}
    </div>
  );
}

export default function OrganizationPage() {
  const params = useParams();
  const companyId = params.id as string;

  const structureQuery = useOrganizationStructure(companyId);
  const structure = structureQuery.data;

  const columns = [
    {
      header: 'Name',
      cell: (item: any) => item.name,
      width: 200,
    },
    {
      header: 'Type',
      cell: (item: any) => (item.type === 'user' ? '👤 User' : '📁 Group'),
    },
    {
      header: 'Members',
      cell: (item: any) => item.member_count || '-',
    },
    {
      header: 'Parent',
      cell: (item: any) => item.parent_id || 'Root',
    },
  ];

  const nodes = structure?.nodes || [];
  const root = structure?.root;

  return (
    <Container header={<Header>Organization Structure</Header>}>
      <SpaceBetween size="xl">
        {/* Tree View */}
        {structureQuery.isPending ? (
          <Spinner />
        ) : root ? (
          <SpaceBetween size="m">
            <Header variant="h3">Hierarchy</Header>
            <TreeNodeComponent node={root} />
          </SpaceBetween>
        ) : (
          <Alert>No organization structure data</Alert>
        )}

        {/* All Nodes Table */}
        <SpaceBetween size="m">
          <Header variant="h3">All Nodes</Header>
          {structureQuery.isPending ? (
            <Spinner />
          ) : nodes.length > 0 ? (
            <Table columnDefinitions={columns} items={nodes} variant="full-page" />
          ) : (
            <Alert>No nodes found</Alert>
          )}
        </SpaceBetween>
      </SpaceBetween>
    </Container>
  );
}
