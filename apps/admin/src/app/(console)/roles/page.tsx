"use client";

import { useState } from "react";
import {
  useRoles,
  useCreateRole,
  useUpdateRole,
  useDeleteRole,
  useAvailablePermissions,
  type Role,
} from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import Button from "@cloudscape-design/components/button";
import Table from "@cloudscape-design/components/table";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Modal from "@cloudscape-design/components/modal";
import FormField from "@cloudscape-design/components/form-field";
import Input from "@cloudscape-design/components/input";
import Checkbox from "@cloudscape-design/components/checkbox";
import Spinner from "@cloudscape-design/components/spinner";
import Alert from "@cloudscape-design/components/alert";

const DEMO_COMPANY_ID = "demo-company";

export default function RolesPage() {
  const { data: roles = [], isLoading } = useRoles(DEMO_COMPANY_ID);
  const availablePermissions = useAvailablePermissions();
  const createMutation = useCreateRole();
  const updateMutation = useUpdateRole("");
  const deleteMutation = useDeleteRole();

  const [showModal, setShowModal] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [roleName, setRoleName] = useState("");
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [selectedItems, setSelectedItems] = useState<Role[]>([]);

  const handleOpenCreate = () => {
    setEditingRole(null);
    setRoleName("");
    setSelectedPermissions([]);
    setShowModal(true);
  };

  const handleOpenEdit = (role: Role) => {
    setEditingRole(role);
    setRoleName(role.name);
    setSelectedPermissions(role.permissions.map((p) => p.id));
    setShowModal(true);
  };

  const handleSaveRole = () => {
    if (!roleName.trim()) return;

    if (editingRole) {
      updateMutation.mutate({
        name: roleName,
        permission_ids: selectedPermissions,
      });
    } else {
      createMutation.mutate({
        company_id: DEMO_COMPANY_ID,
        name: roleName,
        permission_ids: selectedPermissions,
      });
    }

    setShowModal(false);
  };

  const handleDeleteRoles = () => {
    selectedItems.forEach((role) => {
      deleteMutation.mutate(role.id);
    });
    setSelectedItems([]);
  };

  const isLoading_update = updateMutation.isPending || createMutation.isPending;

  return (
    <Box padding="l">
      <Container
        header={
          <Header
            variant="h1"
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button
                  disabled={selectedItems.length === 0}
                  onClick={handleDeleteRoles}
                >
                  Delete Selected
                </Button>
                <Button variant="primary" onClick={handleOpenCreate}>
                  Create Role
                </Button>
              </SpaceBetween>
            }
          >
            Roles &amp; Permissions
          </Header>
        }
      >
        <SpaceBetween direction="vertical" size="l">
          {createMutation.isSuccess && (
            <Alert type="success">Role created successfully</Alert>
          )}
          {updateMutation.isSuccess && (
            <Alert type="success">Role updated successfully</Alert>
          )}
          {(createMutation.isError || updateMutation.isError) && (
            <Alert type="error">Failed to save role</Alert>
          )}

          {isLoading ? (
            <Box textAlign="center" padding="l">
              <Spinner />
            </Box>
          ) : (
            <Table
              columnDefinitions={[
                {
                  id: "name",
                  header: "Role Name",
                  cell: (item: Role) => item.name,
                  sortingField: "name",
                },
                {
                  id: "permissions",
                  header: "Permissions",
                  cell: (item: Role) => `${item.permissions.length} permissions`,
                },
                {
                  id: "created_at",
                  header: "Created",
                  cell: (item: Role) =>
                    new Date(item.created_at).toLocaleDateString(),
                  sortingField: "created_at",
                },
                {
                  id: "actions",
                  header: "Actions",
                  cell: (item: Role) => (
                    <Button
                      variant="inline-link"
                      onClick={() => handleOpenEdit(item)}
                    >
                      Edit
                    </Button>
                  ),
                },
              ]}
              items={roles}
              selectionType="multi"
              selectedItems={selectedItems}
              onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
              variant="full-page"
              empty={
                <Box textAlign="center" padding="xl">
                  No roles found. Create your first role.
                </Box>
              }
            />
          )}
        </SpaceBetween>
      </Container>

      <Modal
        onDismiss={() => setShowModal(false)}
        visible={showModal}
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button variant="link" onClick={() => setShowModal(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                loading={isLoading_update}
                onClick={handleSaveRole}
              >
                {editingRole ? "Update" : "Create"} Role
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={editingRole ? "Edit Role" : "Create Role"}
      >
        <SpaceBetween direction="vertical" size="m">
          <FormField label="Role Name">
            <Input
              value={roleName}
              onChange={(e) => setRoleName(e.detail.value)}
              placeholder="e.g., Administrator, Viewer, Moderator"
            />
          </FormField>

          <FormField label="Permissions">
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                gap: "12px",
              }}
            >
              {availablePermissions.map((permission) => (
                <Checkbox
                  key={permission.id}
                  checked={selectedPermissions.includes(permission.id)}
                  onChange={(e) => {
                    if (e.detail.checked) {
                      setSelectedPermissions([
                        ...selectedPermissions,
                        permission.id,
                      ]);
                    } else {
                      setSelectedPermissions(
                        selectedPermissions.filter((p) => p !== permission.id)
                      );
                    }
                  }}
                >
                  {permission.action.toUpperCase()} {permission.resource}
                </Checkbox>
              ))}
            </div>
          </FormField>
        </SpaceBetween>
      </Modal>
    </Box>
  );
}
