'use client';

import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Select,
  Checkbox,
  Button,
  Table,
  Modal,
  Box,
  Spinner,
  Alert,
} from '@cloudscape-design/components';
import {
  useReportSchedules,
  useCreateReportSchedule,
  useUpdateReportSchedule,
  useDeleteReportSchedule,
} from '@/hooks/useReports';

export default function ReportsPage() {
  const params = useParams();
  const companyId = params.id as string;

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    frequency: 'daily',
    template_type: 'summary',
    recipients: '',
    is_enabled: true,
  });

  const schedulesQuery = useReportSchedules(companyId);
  const createMutation = useCreateReportSchedule();
  const updateMutation = useUpdateReportSchedule();
  const deleteMutation = useDeleteReportSchedule();

  const schedules = schedulesQuery.data || [];

  const handleSave = async () => {
    const recipients = formData.recipients
      .split(',')
      .map((r) => r.trim())
      .filter((r) => r);

    if (editingId) {
      await updateMutation.mutateAsync({
        companyId,
        scheduleId: editingId,
        data: {
          name: formData.name,
          frequency: formData.frequency as any,
          template_type: formData.template_type,
          recipients,
          is_enabled: formData.is_enabled,
        } as any,
      });
    } else {
      await createMutation.mutateAsync({
        companyId,
        data: {
          name: formData.name,
          frequency: formData.frequency as any,
          template_type: formData.template_type,
          recipients,
          is_enabled: formData.is_enabled,
          next_run: new Date().toISOString(),
          id: '',
          company_id: companyId,
        } as any,
      });
    }
    setShowModal(false);
    setFormData({
      name: '',
      frequency: 'daily',
      template_type: 'summary',
      recipients: '',
      is_enabled: true,
    });
    setEditingId(null);
  };

  const handleDelete = async (scheduleId: string) => {
    if (confirm('Are you sure you want to delete this report schedule?')) {
      await deleteMutation.mutateAsync({ companyId, scheduleId });
    }
  };

  const columns = [
    { header: 'Name', cell: (item: any) => item.name },
    { header: 'Frequency', cell: (item: any) => item.frequency },
    { header: 'Template', cell: (item: any) => item.template_type },
    {
      header: 'Recipients',
      cell: (item: any) => item.recipients.join(', '),
    },
    {
      header: 'Next Run',
      cell: (item: any) => new Date(item.next_run).toLocaleDateString(),
    },
    {
      header: 'Status',
      cell: (item: any) => (item.is_enabled ? 'Enabled' : 'Disabled'),
    },
    {
      header: 'Actions',
      cell: (item: any) => (
        <SpaceBetween direction="horizontal" size="xs">
          <Button
            onClick={() => {
              setEditingId(item.id);
              setFormData({
                name: item.name,
                frequency: item.frequency,
                template_type: item.template_type,
                recipients: item.recipients.join(', '),
                is_enabled: item.is_enabled,
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
        </SpaceBetween>
      ),
    },
  ];

  return (
    <Container header={<Header>Reports & Exports</Header>}>
      <SpaceBetween size="l">
        <Box float="right">
          <Button
            variant="primary"
            onClick={() => {
              setEditingId(null);
              setFormData({
                name: '',
                frequency: 'daily',
                template_type: 'summary',
                recipients: '',
                is_enabled: true,
              });
              setShowModal(true);
            }}
          >
            Create Report Schedule
          </Button>
        </Box>

        {schedulesQuery.isPending ? (
          <Spinner />
        ) : schedules.length > 0 ? (
          <Table columnDefinitions={columns} items={schedules} variant="full-page" />
        ) : (
          <Alert>No report schedules configured</Alert>
        )}

        {/* Modal */}
        <Modal
          onDismiss={() => setShowModal(false)}
          visible={showModal}
          header={editingId ? 'Edit Report Schedule' : 'Create Report Schedule'}
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
          <SpaceBetween size="m">
            <FormField label="Schedule Name">
              <Input
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.detail.value })
                }
              />
            </FormField>
            <FormField label="Frequency">
              <Select
                selectedOption={{
                  label: formData.frequency.toUpperCase(),
                  value: formData.frequency,
                }}
                options={[
                  { label: 'Daily', value: 'daily' },
                  { label: 'Weekly', value: 'weekly' },
                  { label: 'Monthly', value: 'monthly' },
                ]}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    frequency: e.detail.selectedOption.value || 'daily',
                  })
                }
              />
            </FormField>
            <FormField label="Template Type">
              <Select
                selectedOption={{
                  label: formData.template_type,
                  value: formData.template_type,
                }}
                options={[
                  { label: 'Summary', value: 'summary' },
                  { label: 'Detailed', value: 'detailed' },
                  { label: 'Executive', value: 'executive' },
                ]}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    template_type: e.detail.selectedOption.value || 'summary',
                  })
                }
              />
            </FormField>
            <FormField label="Recipients (comma-separated)">
              <Input
                value={formData.recipients}
                onChange={(e) =>
                  setFormData({ ...formData, recipients: e.detail.value })
                }
                placeholder="email@example.com, another@example.com"
              />
            </FormField>
            <Checkbox
              checked={formData.is_enabled}
              onChange={(e) =>
                setFormData({ ...formData, is_enabled: e.detail.checked })
              }
            >
              Enabled
            </Checkbox>
          </SpaceBetween>
        </Modal>
      </SpaceBetween>
    </Container>
  );
}
