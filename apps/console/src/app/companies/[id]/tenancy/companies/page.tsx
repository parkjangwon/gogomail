'use client';
import { DataTable } from '@/components/DataTable';


import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  TextFilter,
  Pagination,
  Badge,
  StatusIndicator,
  Alert,
  ProgressBar,
} from '@cloudscape-design/components';
import { useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useParams } from 'next/navigation';
import { useCompanies, useCreateCompany, useUpdateCompany, useDeleteCompany, useDomains, type Company } from '@/hooks';
import { CompanyManagementModals } from './company-management-modals';
import { CompanyDetailModal } from './company-detail-modal';

export default function CompaniesPage() {
  const { t } = useI18n();
  const router = useRouter();
  const params = useParams();
  const cid = params?.id as string;
  const companiesQuery = useCompanies(200);
  const createCompany = useCreateCompany();
  const updateCompany = useUpdateCompany();
  const deleteCompany = useDeleteCompany();
  const companies = companiesQuery.data ?? [];
  const loading = companiesQuery.isLoading;
  const [filter, setFilter] = useState('');
  const [currentPage, setCurrentPage] = useState(1);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newCompany, setNewCompany] = useState({ name: '', quota_gb: '' });
  const [creating, setCreating] = useState(false);
  const [createdCompany, setCreatedCompany] = useState<Company | null>(null);
  const [showPostCreateGuide, setShowPostCreateGuide] = useState(false);
  const [selectedCompany, setSelectedCompany] = useState<Company | null>(null);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const domainsQuery = useDomains(selectedCompany?.id);
  const companyDomains = domainsQuery.data ?? [];
  const loadingDomains = domainsQuery.isLoading;

  // Edit modal
  const [editTarget, setEditTarget] = useState<Company | null>(null);
  const [editForm, setEditForm] = useState({ name: '', quota_gb: '' });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  // Delete modal
  const [deleteTarget, setDeleteTarget] = useState<Company | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const itemsPerPage = 20;

  const handleViewCompany = (company: Company) => {
    setSelectedCompany(company);
    setShowDetailModal(true);
  };

  const handleCreateCompany = async () => {
    if (!newCompany.name.trim()) return;
    setCreating(true);
    try {
      const company = await createCompany.mutateAsync({
        name: newCompany.name,
        quota_limit: newCompany.quota_gb ? parseInt(newCompany.quota_gb) * 1073741824 : 0,
      });
      setCreatedCompany(company.company ?? null);
      setShowCreateModal(false);
      setNewCompany({ name: '', quota_gb: '' });
      setShowPostCreateGuide(true);
    } finally {
      setCreating(false);
    }
  };

  const openEdit = useCallback((c: Company) => {
    setEditTarget(c);
    setEditForm({
      name: c.name,
      quota_gb: (c.quota_limit ?? 0) > 0 ? String(Math.round((c.quota_limit ?? 0) / 1073741824)) : '',
    });
    setSaveError('');
  }, []);

  const handleSaveEdit = async () => {
    if (!editTarget) return;
    setSaving(true);
    setSaveError('');
    try {
      await updateCompany.mutateAsync({
        companyId: editTarget.id,
        data: {
          name: editForm.name.trim(),
          quota_limit: editForm.quota_gb ? parseInt(editForm.quota_gb) * 1073741824 : 0,
        },
      });
      setEditTarget(null);
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteCompany = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await deleteCompany.mutateAsync({ companyId: deleteTarget.id });
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const filteredCompanies = companies.filter(c =>
    c.name.toLowerCase().includes(filter.toLowerCase())
  );

  const paginatedCompanies = filteredCompanies.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );

  const getQuotaPercent = (used: number, limit: number) =>
    limit > 0 ? Math.round((used / limit) * 100) : 0;

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.companies.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.companies.description')}
          counter={`(${companies.length})`}
          actions={
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              {t('pages.companies.create_company')}
            </Button>
          }
        >
          {t('pages.companies.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {showPostCreateGuide && createdCompany && (
          <Alert
            type="success"
            dismissible
            onDismiss={() => setShowPostCreateGuide(false)}
            header={`"${createdCompany.name}"`}
            action={
              <Button
                variant="primary"
                onClick={() => {
                  setShowPostCreateGuide(false);
                  router.push(`/companies/${cid}/tenancy/domains`);
                }}
              >
                {t('pages.companies.add_domain_now')}
              </Button>
            }
          >
            {t('pages.companies.created_next_step')}
          </Alert>
        )}

        <DataTable
          columnDefinitions={[
            {
              header: t('pages.companies.company_name'),
              cell: (c: Company) => (
                <Button variant="inline-link" onClick={() => router.push(`/companies/${c.id}`)}>
                  {c.name}
                </Button>
              ),
              width: '25%',
            },
            {
              header: t('pages.companies.status'),
              cell: (c: Company) => (
                <Badge color={c.status === 'active' ? 'green' : c.status === 'suspended' ? 'severity-high' : 'grey'}>
                  {c.status}
                </Badge>
              ),
              width: '10%',
            },
            {
              header: t('pages.companies.storage_quota'),
              cell: (c: Company) => {
                const limit = c.quota_limit ?? 0;
                const used = c.quota_used ?? 0;
                const pct = getQuotaPercent(used, limit);
                return limit > 0 ? (
                  <ProgressBar
                    value={pct}
                    status={c.over_allocated ? 'error' : pct > 80 ? 'in-progress' : 'success'}
                    resultText={`${pct}%`}
                    additionalInfo={`${(used / 1073741824).toFixed(1)} / ${(limit / 1073741824).toFixed(1)} GB`}
                  />
                ) : <Box color="text-body-secondary">{t('pages.companies.unlimited')}</Box>;
              },
              width: '30%',
            },
            {
              header: t('pages.companies.created'),
              cell: (c: Company) => new Date(c.created_at).toLocaleDateString(),
              width: '15%',
            },
            {
              header: t('pages.companies.actions'),
              cell: (c: Company) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => handleViewCompany(c)}>
                    {t('pages.companies.view')}
                  </Button>
                  <Button variant="inline-link" onClick={() => openEdit(c)}>
                    {t('common.edit') || '수정'}
                  </Button>
                  <Button variant="inline-link" onClick={() => { setDeleteTarget(c); setDeleteError(''); }}>
                    {t('common.delete') || '삭제'}
                  </Button>
                </SpaceBetween>
              ),
              width: '25%',
            },
          ]}
          items={paginatedCompanies}
          header={
            <Header variant="h2" counter={`(${filteredCompanies.length})`}>
              {t('pages.companies.company_list')}
            </Header>
          }
          filter={
            <TextFilter
              filteringText={filter}
              filteringPlaceholder={t('pages.companies.search_placeholder')}
              onChange={(e) => { setFilter(e.detail.filteringText); setCurrentPage(1); }}
            />
          }
          pagination={
            <Pagination
              currentPageIndex={currentPage}
              pagesCount={Math.max(1, Math.ceil(filteredCompanies.length / itemsPerPage))}
              onChange={(e) => setCurrentPage(e.detail.currentPageIndex)}
            />
          }
          empty={
            <Box textAlign="center" padding="l">
              <SpaceBetween size="m" alignItems="center">
                <StatusIndicator type="info">{t('pages.companies.no_companies')}</StatusIndicator>
                <Button variant="primary" onClick={() => setShowCreateModal(true)}>
                  {t('pages.companies.create_first')}
                </Button>
              </SpaceBetween>
            </Box>
          }
        />
      </SpaceBetween>

      <CompanyManagementModals
        t={t}
        showCreateModal={showCreateModal}
        onDismissCreate={() => setShowCreateModal(false)}
        newCompany={newCompany}
        onChangeNewCompany={setNewCompany}
        onCreate={handleCreateCompany}
        creating={creating}
        editTarget={editTarget}
        onDismissEdit={() => { setEditTarget(null); setSaveError(''); }}
        editForm={editForm}
        onChangeEditForm={setEditForm}
        onSaveEdit={handleSaveEdit}
        saving={saving}
        saveError={saveError}
        deleteTarget={deleteTarget}
        onDismissDelete={() => { setDeleteTarget(null); setDeleteError(''); }}
        onDelete={handleDeleteCompany}
        deleting={deleting}
        deleteError={deleteError}
      />

      {selectedCompany && (
        <CompanyDetailModal
          t={t}
          company={selectedCompany}
          open={showDetailModal}
          loadingDomains={loadingDomains}
          domains={companyDomains}
          onClose={() => setShowDetailModal(false)}
          onOpenDomains={() => {
            setShowDetailModal(false);
            router.push(`/companies/${selectedCompany.id}/tenancy/domains`);
          }}
          onOpenDomain={(domainId) => {
            setShowDetailModal(false);
            router.push(`/companies/${selectedCompany.id}/domains/${domainId}`);
          }}
        />
      )}
    </ContentLayout>
  );
}
