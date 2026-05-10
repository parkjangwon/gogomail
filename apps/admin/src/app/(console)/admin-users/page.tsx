'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Table,
  Button,
  Modal,
  FormField,
  Input,
  Select,
  Box,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';

interface AdminUser {
  id: string;
  username: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [selectedItems, setSelectedItems] = useState<AdminUser[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [formData, setFormData] = useState({
    username: '',
    email: '',
    password: '',
    role: 'admin',
  });

  useEffect(() => {
    // Fetch admin users
    fetchAdminUsers();
  }, []);

  const fetchAdminUsers = async () => {
    try {
      const res = await fetch('/admin/v1/admin-users', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      }
    } catch (error) {
      console.error('Failed to fetch admin users:', error);
    }
  };

  const handleCreate = async () => {
    try {
      const res = await fetch('/admin/v1/admin-users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
        credentials: 'include',
      });

      if (res.ok) {
        setShowModal(false);
        setFormData({ username: '', email: '', password: '', role: 'admin' });
        fetchAdminUsers();
      }
    } catch (error) {
      console.error('Failed to create admin user:', error);
    }
  };

  const handleDelete = async (userId: string) => {
    try {
      await fetch(`/admin/v1/admin-users/${userId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      fetchAdminUsers();
    } catch (error) {
      console.error('Failed to delete admin user:', error);
    }
  };

  return (
    <Container header={<Header>관리자 사용자 관리</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button variant="primary" onClick={() => setShowModal(true)}>
            + 관리자 추가
          </Button>
        </Box>

        <Table
          columnDefinitions={[
            { header: '사용자명', cell: (user) => user.username },
            { header: '이메일', cell: (user) => user.email },
            { header: '역할', cell: (user) => user.role },
            { header: '상태', cell: (user) => user.status },
            { header: '생성일', cell: (user) => new Date(user.created_at).toLocaleDateString('ko-KR') },
            {
              header: '작업',
              cell: (user) => (
                <Button
                  variant="inline-link"
                  onClick={() => handleDelete(user.id)}
                >
                  삭제
                </Button>
              ),
            },
          ]}
          items={users}
          selectionType="multi"
          selectedItems={selectedItems}
          onSelectionChange={(e) => setSelectedItems(e.detail.selectedItems)}
        />

        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          footer={
            <Box float="right">
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => setShowModal(false)}>취소</Button>
                <Button variant="primary" onClick={handleCreate}>
                  추가
                </Button>
              </SpaceBetween>
            </Box>
          }
          header="새 관리자 사용자"
        >
          <SpaceBetween size="m">
            <FormField label="사용자명">
              <Input
                value={formData.username}
                onChange={(e) =>
                  setFormData({ ...formData, username: e.detail.value })
                }
              />
            </FormField>
            <FormField label="이메일">
              <Input
                type="email"
                value={formData.email}
                onChange={(e) =>
                  setFormData({ ...formData, email: e.detail.value })
                }
              />
            </FormField>
            <FormField label="비밀번호">
              <Input
                type="password"
                value={formData.password}
                onChange={(e) =>
                  setFormData({ ...formData, password: e.detail.value })
                }
              />
            </FormField>
            <FormField label="역할">
              <Select
                selectedOption={{ label: '관리자', value: 'admin' }}
                options={[
                  { label: '시스템 관리자', value: 'system_admin' },
                  { label: '관리자', value: 'admin' },
                  { label: '읽기 전용', value: 'readonly' },
                ]}
                onChange={(e) =>
                  setFormData({ ...formData, role: e.detail.selectedOption.value || 'admin' })
                }
              />
            </FormField>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
