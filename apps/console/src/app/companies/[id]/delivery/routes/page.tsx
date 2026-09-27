'use client';
import { DataTable } from '@/components/DataTable';
import { ConfirmModal } from '@/components/ConfirmModal';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Badge,
  Modal,
  FormField,
  Input,
  Select,
  SelectProps,
  StatusIndicator,
  Alert,
  Flashbar,
  FlashbarProps,
} from '@cloudscape-design/components';
import { useState, useEffect } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';

interface DeliveryRoute {
  id: string;
  domain_pattern: string;
  hosts: string[];
  port: number;
  tls_mode: string;
  status: 'active' | 'disabled' | string;
  description: string;
  created_at: string;
}

type DeliveryRouteStatus = 'active' | 'disabled';

function isRouteActive(status: string): boolean {
  return status.trim().toLowerCase() === 'active';
}

function nextRouteStatus(status: string): DeliveryRouteStatus {
  return isRouteActive(status) ? 'disabled' : 'active';
}

const TLS_OPTIONS: SelectProps.Option[] = [
  { label: 'none', value: 'none' },
  { label: 'starttls', value: 'starttls' },
  { label: 'tls', value: 'tls' },
];

export default function DeliveryRoutesPage() {
  const { t } = useI18n();
  const { canMutate } = useCompany();
  const [routes, setRoutes] = useState<DeliveryRoute[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [flash, setFlash] = useState<FlashbarProps.MessageDefinition[]>([]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createError, setCreateError] = useState('');
  const [newRoute, setNewRoute] = useState({
    domain_pattern: '',
    hosts: '',
    port: '25',
    tls_mode: 'none',
    description: '',
  });
  const [creating, setCreating] = useState(false);

  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<DeliveryRoute | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [confirmToggle, setConfirmToggle] = useState<DeliveryRoute | null>(null);
  const [toggleError, setToggleError] = useState('');

  const extractError = async (res: Response, fallback: string): Promise<string> => {
    const data = (await res.json().catch(() => ({}))) as { error?: { message?: string } | string };
    const msg = typeof data.error === 'string' ? data.error : data.error?.message;
    return msg || fallback;
  };

  useEffect(() => {
    fetchRoutes();
  }, []);

  const fetchRoutes = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/admin/delivery-routes?limit=100', {
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setRoutes(data.routes || []);
      }
    } catch {
      // mutation error handled by caller
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newRoute.domain_pattern.trim() || !newRoute.hosts.trim()) return;
    setCreateError('');
    setCreating(true);
    try {
      const hosts = newRoute.hosts
        .split(',')
        .map((h) => h.trim())
        .filter(Boolean);
      const res = await fetch('/api/admin/delivery-routes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain_pattern: newRoute.domain_pattern.trim(),
          hosts,
          port: parseInt(newRoute.port) || 25,
          tls_mode: newRoute.tls_mode,
          description: newRoute.description.trim() || undefined,
        }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowCreateModal(false);
        setNewRoute({ domain_pattern: '', hosts: '', port: '25', tls_mode: 'none', description: '' });
        setFlash([{ type: 'success', content: t('pages.routes_page.created', 'Route created.'), dismissible: true, onDismiss: () => setFlash([]) }]);
        fetchRoutes();
      } else {
        setCreateError(await extractError(res, t('pages.routes_page.create_failed', 'Failed to create route.')));
      }
    } catch {
      setCreateError(t('pages.routes_page.create_failed', 'Failed to create route.'));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (route: DeliveryRoute) => {
    setDeletingId(route.id);
    setDeleteError('');
    try {
      const res = await fetch(`/api/admin/delivery-routes/${route.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        setConfirmDelete(null);
        fetchRoutes();
      } else {
        setDeleteError(await extractError(res, t('pages.routes_page.delete_failed', 'Failed to delete route.')));
      }
    } catch {
      setDeleteError(t('pages.routes_page.delete_failed', 'Failed to delete route.'));
    } finally {
      setDeletingId(null);
    }
  };

  const handleToggleStatus = async (route: DeliveryRoute) => {
    setTogglingId(route.id);
    setToggleError('');
    const nextStatus = nextRouteStatus(route.status);
    try {
      const res = await fetch(`/api/admin/delivery-routes/${route.id}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: nextStatus }),
        credentials: 'include',
      });
      if (res.ok) {
        setConfirmToggle(null);
        fetchRoutes();
      } else {
        setToggleError(await extractError(res, t('pages.routes_page.toggle_failed', 'Failed to update route status.')));
      }
    } catch {
      setToggleError(t('pages.routes_page.toggle_failed', 'Failed to update route status.'));
    } finally {
      setTogglingId(null);
    }
  };

  const filteredRoutes = routes.filter(
    (r) =>
      r.domain_pattern.toLowerCase().includes(filter.toLowerCase()) ||
      (r.description || '').toLowerCase().includes(filter.toLowerCase())
  );

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.routes_page.title')}</Header>}>
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
          description={t('pages.routes_page.description')}
          actions={
            canMutate ? (
              <Button variant="primary" onClick={() => { setCreateError(''); setShowCreateModal(true); }}>
                {t('pages.routes.create_route')}
              </Button>
            ) : undefined
          }
        >
          {t('pages.routes_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {flash.length > 0 && <Flashbar items={flash} />}
        <DataTable
          columnDefinitions={[
            {
              header: t('pages.routes_page.domain_pattern'),
              cell: (item: DeliveryRoute) => (
                <Box fontWeight="bold">{item.domain_pattern}</Box>
              ),
              width: '22%',
            },
            {
              header: t('pages.routes_page.hosts'),
              cell: (item: DeliveryRoute) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {(item.hosts || []).join(', ')}
                </Box>
              ),
              width: '25%',
            },
            {
              header: t('pages.routes_page.port'),
              cell: (item: DeliveryRoute) => item.port || 25,
              width: '8%',
            },
            {
              header: t('pages.routes_page.tls_mode'),
              cell: (item: DeliveryRoute) => (
                <Badge color={item.tls_mode === 'tls' ? 'green' : item.tls_mode === 'starttls' ? 'blue' : 'grey'}>
                  {item.tls_mode || 'none'}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.routes_page.status'),
              cell: (item: DeliveryRoute) => (
                <Badge color={isRouteActive(item.status) ? 'green' : 'grey'}>
                  {item.status}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.routes_page.created'),
              cell: (item: DeliveryRoute) =>
                new Date(item.created_at).toLocaleDateString(),
              width: '12%',
            },
            {
              header: t('pages.routes_page.actions'),
              cell: (item: DeliveryRoute) =>
                canMutate ? (
                  <SpaceBetween direction="horizontal" size="xs">
                    <Button
                      variant="inline-link"
                      onClick={() => { setToggleError(''); setConfirmToggle(item); }}
                      loading={togglingId === item.id}
                    >
                      {isRouteActive(item.status)
                        ? t('pages.routes_page.deactivate')
                        : t('pages.routes_page.activate')}
                    </Button>
                    <Button
                      variant="inline-link"
                      onClick={() => { setDeleteError(''); setConfirmDelete(item); }}
                      loading={deletingId === item.id}
                    >
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                ) : null,
              width: '13%',
            },
          ]}
          items={filteredRoutes}
          header={
            <Header variant="h2" counter={`(${filteredRoutes.length})`}>
              {t('pages.routes_page.routes')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('common.search')}
              onChange={(e) => setFilter(e.detail.filteringText)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              <StatusIndicator type="info">{t('pages.routes_page.no_routes')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      {/* Create Modal */}
      <Modal
        onDismiss={() => setShowCreateModal(false)}
        visible={showCreateModal}
        size="medium"
        footer={
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={() => setShowCreateModal(false)}>{t('common.cancel')}</Button>
              <Button
                variant="primary"
                onClick={handleCreate}
                loading={creating}
                disabled={!newRoute.domain_pattern.trim() || !newRoute.hosts.trim()}
              >
                {t('pages.routes_page.create_btn')}
              </Button>
            </SpaceBetween>
          </Box>
        }
        header={t('pages.routes_page.create_modal_title')}
      >
        <SpaceBetween size="m">
          {createError && <Alert type="error">{createError}</Alert>}
          <FormField label={t('pages.routes_page.domain_pattern_label')}>
            <Input
              value={newRoute.domain_pattern}
              onChange={(e) => setNewRoute({ ...newRoute, domain_pattern: e.detail.value })}
              placeholder="*.example.com"
            />
          </FormField>
          <FormField
            label={t('pages.routes_page.hosts_label')}
            constraintText={t('pages.routes_page.hosts_constraint')}
          >
            <Input
              value={newRoute.hosts}
              onChange={(e) => setNewRoute({ ...newRoute, hosts: e.detail.value })}
              placeholder="mail.example.com, 10.0.0.1"
            />
          </FormField>
          <FormField label={t('pages.routes_page.port_label')}>
            <Input
              type="number"
              value={newRoute.port}
              onChange={(e) => setNewRoute({ ...newRoute, port: e.detail.value })}
            />
          </FormField>
          <FormField label={t('pages.routes_page.tls_mode_label')}>
            <Select
              selectedOption={TLS_OPTIONS.find((o) => o.value === newRoute.tls_mode) ?? TLS_OPTIONS[0]}
              options={TLS_OPTIONS}
              onChange={(e: { detail: { selectedOption: SelectProps.Option } }) =>
                setNewRoute({ ...newRoute, tls_mode: e.detail.selectedOption.value ?? 'none' })
              }
              expandToViewport
            />
          </FormField>
          <FormField label={t('pages.routes_page.description_label')}>
            <Input
              value={newRoute.description}
              onChange={(e) => setNewRoute({ ...newRoute, description: e.detail.value })}
              placeholder={t('pages.routes_page.description_placeholder')}
            />
          </FormField>
        </SpaceBetween>
      </Modal>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        visible={!!confirmDelete}
        header={t('pages.routes_page.delete_modal_title')}
        onConfirm={() => confirmDelete && handleDelete(confirmDelete)}
        onDismiss={() => { setConfirmDelete(null); setDeleteError(''); }}
        loading={deletingId === confirmDelete?.id}
        error={deleteError || undefined}
      >
        {t('pages.routes_page.delete_confirm')} <strong>{confirmDelete?.domain_pattern}</strong>?
      </ConfirmModal>

      {/* Enable/Disable Confirmation Modal — disabling can halt mail flow */}
      <ConfirmModal
        visible={!!confirmToggle}
        header={
          confirmToggle && isRouteActive(confirmToggle.status)
            ? t('pages.routes_page.deactivate_modal_title', 'Disable route')
            : t('pages.routes_page.activate_modal_title', 'Enable route')
        }
        onConfirm={() => confirmToggle && handleToggleStatus(confirmToggle)}
        onDismiss={() => { setConfirmToggle(null); setToggleError(''); }}
        loading={togglingId === confirmToggle?.id}
        confirmLabel={
          confirmToggle && isRouteActive(confirmToggle.status)
            ? t('pages.routes_page.deactivate')
            : t('pages.routes_page.activate')
        }
        error={toggleError || undefined}
      >
        {confirmToggle && isRouteActive(confirmToggle.status)
          ? t('pages.routes_page.deactivate_confirm', 'Disabling this route can halt mail flow for')
          : t('pages.routes_page.activate_confirm', 'Enable mail routing for')}{' '}
        <strong>{confirmToggle?.domain_pattern}</strong>?
      </ConfirmModal>
    </ContentLayout>
  );
}
