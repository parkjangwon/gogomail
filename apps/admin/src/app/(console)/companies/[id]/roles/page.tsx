'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Textarea,
  Button,
  Table,
  Modal,
  Box,
  Spinner,
  Alert,
  Checkbox,
} from '@cloudscape-design/components';
import { useRoles, useCreateRole, useUpdateRole, useDeleteRole } from '@/hooks/useRoles';

const RESOURCES = ['users', 'domains', 'config', 'audit_logs', 'api_keys', 'alert_rules'];
const ACTIONS = ['create', 'read', 'update', 'delete'];
const SCOPES = ['self', 'company', 'domain'];

export default function RolesPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    permissions: [] as any[],
  });

  const rolesQuery = useRoles(companyId);
  const createMutation = useCreateRole();
  const updateMutation = useUpdateRole();
  const deleteMutation = useDeleteRole();

  const roles = rolesQuery.data || [];

  const handleSave = async () => {
    if (editingId) {
      await updateMutation.mutateAsync({
        companyId,
        roleId: editingId,
        data: {
          name: formData.name,
          description: formData.description,
          permissions: formData.permissions,
        } as any,
      });
    } else {
      await createMutation.mutateAsync({
        companyId,
        data: {
          name: formData.name,
          description: formData.description,
          is_builtin: false,
          permissions: formData.permissions,
          id: '',
          company_id: companyId,
          created_at: new Date().toISOString(),
        } as any,
      });
    }
    setShowModal(false);
    setFormData({ name: '', description: '', permissions: [] });
    setEditingId(null);
  };

  const handleDelete = async (roleId: string) => {
    if (confirm('Are you sure you want to delete this role?')) {
      await deleteMutation.mutateAsync({ companyId, roleId });
    }
  };

  const togglePermission = (resource: string, action: string, scope: string) => {
    const permKey = `${resource}:${action}:${scope}`;
    const existing = formData.permissions.findIndex(
      (p) => `${p.resource}:${p.action}:${p.scope}` === permKey
    );

    if (existing >= 0) {
      const newPerms = [...formData.permissions];
      newPerms.splice(existing, 1);
      setFormData({ ...formData, permissions: newPerms });
    } else {
      setFormData({
        ...formData,
        permissions: [
          ...formData.permissions,
          { resource, action, scope, conditions: {} },
        ],
      });
    }
  };

  const hasPermission = (resource: string, action: string, scope: string) => {
    return formData.permissions.some(
      (p) => p.resource === resource && p.action === action && p.scope === scope
    );
  };

  const columns = [
    { header: 'Name', cell: (item: any) => item.name },
    { header: 'Description', cell: (item: any) => item.description || '-' },
    {
      header: 'Permissions',
      cell: (item: any) => `${item.permissions?.length || 0}`,
    },
    {
      header: 'Type',
      cell: (item: any) => (item.is_builtin ? 'Built-in' : 'Custom'),
    },
    {
      header: 'Actions',
      cell: (item: any) => (
        <SpaceBetween direction="horizontal" size="xs">
          {!item.is_builtin && (
            <>
              <Button
                onClick={() => {
                  setEditingId(item.id);
                  setFormData({
                    name: item.name,
                    description: item.description,
                    permissions: item.permissions,
                  });
                  setShowModal(true);
                }}
              >
                Edit
              </Button>
              <Button
                onClick={() => handleDelete(item.id)}
                loading={deleteMutation.isPending}
              >
                Delete
              </Button>
            </>
          )}
        </SpaceBetween>
      ),
    },
  ];

  return (
    <Container header={<Header>Role Management</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button
            variant="primary"
            onClick={() => {
              setEditingId(null);
              setFormData({ name: '', description: '', permissions: [] });
              setShowModal(true);
            }}
          >
            Create Custom Role
          </Button>
        </Box>

        {rolesQuery.isPending ? (
          <Spinner />
        ) : roles.length > 0 ? (
          <Table columnDefinitions={columns} items={roles} variant="full-page" />
        ) : (
          <Alert>No roles configured</Alert>
        )}

        {/* Modal */}
        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          size="large"
          header={editingId ? 'Edit Role' : 'Create Custom Role'}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button variant="link" onClick={() => setShowModal(false)}>
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  onClick={handleSave}
                  loading={createMutation.isPending || updateMutation.isPending}
                >
                  Save
                </Button>
              </SpaceBetween>
            </Box>
          }
        >
          <SpaceBetween size="l">
            <FormField label="Role Name">
              <Input
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.detail.value })
                }
              />
            </FormField>
            <FormField label="Description">
              <Textarea
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.detail.value })
                }
              />
            </FormField>

            {/* Permission Matrix */}
            <SpaceBetween size="m">
              <Header variant="h3">Permissions</Header>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr>
                      <th style={{ border: '1px solid #ddd', padding: '8px' }}>
                        Resource
                      </th>
                      <th style={{ border: '1px solid #ddd', padding: '8px' }}>
                        Action
                      </th>
                      {SCOPES.map((scope) => (
                        <th
                          key={scope}
                          style={{
                            border: '1px solid #ddd',
                            padding: '8px',
                            textAlign: 'center',
                          }}
                        >
                          {scope}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {RESOURCES.map((resource) =>
                      ACTIONS.map((action) => (
                        <tr key={`${resource}-${action}`}>
                          <td
                            style={{
                              border: '1px solid #ddd',
                              padding: '8px',
                              fontWeight:
                                action === ACTIONS[0] ? 'bold' : 'normal',
                            }}
                          >
                            {action === ACTIONS[0] ? resource : ''}
                          </td>
                          <td style={{ border: '1px solid #ddd', padding: '8px' }}>
                            {action}
                          </td>
                          {SCOPES.map((scope) => (
                            <td
                              key={`${resource}-${action}-${scope}`}
                              style={{
                                border: '1px solid #ddd',
                                padding: '8px',
                                textAlign: 'center',
                              }}
                            >
                              <Checkbox
                                checked={hasPermission(resource, action, scope)}
                                onChange={() =>
                                  togglePermission(resource, action, scope)
                                }
                              />
                            </td>
                          ))}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </SpaceBetween>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
