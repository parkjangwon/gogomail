'use client';
import { DataTable } from '@/components/DataTable';
import { User, STATUS_COLORS } from '@/lib/users/userUtils';
import { formatStorage } from '@/lib/users/userPageUtils';
import { CreateUserModal } from '@/components/users/CreateUserModal';
import { EditUserModal } from '@/components/users/EditUserModal';
import { OffboardUserModal } from '@/components/users/OffboardUserModal';
import { ImportUsersModal } from '@/components/users/ImportUsersModal';
import {
  ContentLayout,
  Header,
  Button,
  SpaceBetween,
  Box,
  Spinner,
  Badge,
  TextFilter,
  Select,
  ColumnLayout,
  Container,
  StatusIndicator,
  Flashbar,
  Pagination,
  ButtonDropdown,
} from '@cloudscape-design/components';
import { useUsersPage } from './useUsersPage';

export default function UsersPage() {
  const p = useUsersPage();

  if (p.loading) {
    return (
      <ContentLayout header={<Header variant="h1">{p.t('pages.users_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          counter={`(${p.totalUsers})`}
          actions={
            <SpaceBetween direction="horizontal" size="xs">
              <Button onClick={p.handleExportCSV}>
                {p.t('pages.users_page.users_bulk.export_btn')}
              </Button>
              <Button onClick={() => { p.setShowImportModal(true); p.setImportResult(null); }}>
                {p.t('pages.users_page.users_bulk.import_btn')}
              </Button>
              <Button variant="primary" onClick={p.openCreateModal}>
                {p.t('pages.users_page.create_user_btn')}
              </Button>
            </SpaceBetween>
          }
        >
          {p.t('pages.users_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {p.flashItems.length > 0 && <Flashbar items={p.flashItems} />}
        {p.fetchError && (
          <Flashbar items={[{ type: 'error', content: p.fetchError, id: 'fetch-error', dismissible: true, onDismiss: () => p.setFetchError('') }]} />
        )}

        {/* KPI Summary */}
        <ColumnLayout columns={3} variant="text-grid" minColumnWidth={140}>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{p.totalUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{p.t('pages.users_page.total_label')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{p.activeUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{p.t('pages.users_page.active_label')}</Box>
            </SpaceBetween>
          </Container>
          <Container>
            <SpaceBetween size="xxs">
              <Box fontSize="display-l" fontWeight="bold">{p.suspendedUsers}</Box>
              <Box color="text-body-secondary" fontSize="body-s">{p.t('pages.users_page.suspended_label')}</Box>
            </SpaceBetween>
          </Container>
        </ColumnLayout>

        {/* User Table */}
        <DataTable
          selectionType="multi"
          selectedItems={p.selectedUsers}
          onSelectionChange={e => p.setSelectedUsers(e.detail.selectedItems)}
          trackBy="id"
          columnDefinitions={[
            {
              header: p.t('pages.users_page.username'),
              cell: (u: User) => (
                <SpaceBetween size="xxxs">
                  <Box fontWeight="bold">{u.username}</Box>
                  {u.display_name && (
                    <Box color="text-body-secondary" fontSize="body-s">{u.display_name}</Box>
                  )}
                  {u.must_change_password && (
                    <Badge color="severity-medium">{p.t('pages.users_page.must_change_pw')}</Badge>
                  )}
                </SpaceBetween>
              ),
              width: '22%',
            },
            {
              header: p.t('pages.users_page.domain'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {p.domains.find(d => d.id === u.domain_id)?.name ?? u.domain_id.slice(0, 8) + '…'}
                </Box>
              ),
              width: '18%',
            },
            {
              header: p.t('pages.users_page.recovery_email_label'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {u.recovery_email || '—'}
                </Box>
              ),
              width: '18%',
            },
            {
              header: p.t('pages.users_page.role'),
              cell: (u: User) => {
                const roleColor = u.role === 'system_admin' ? 'red' : u.role === 'company_admin' ? 'green' : 'grey';
                return u.role ? <Badge color={roleColor}>{u.role}</Badge> : <Box color="text-body-secondary">—</Box>;
              },
              width: '12%',
            },
            {
              header: p.t('pages.users_page.status'),
              cell: (u: User) => (
                <Badge color={STATUS_COLORS[u.status] ?? 'grey'}>{u.status}</Badge>
              ),
              width: '10%',
            },
            {
              header: p.t('pages.users_page.storage'),
              cell: (u: User) => (
                <Box fontSize="body-s" color={
                  u.quota_limit > 0 && u.quota_used / u.quota_limit > 0.8
                    ? 'text-status-error'
                    : 'text-body-secondary'
                }>
                  {formatStorage(u.quota_used ?? 0, u.quota_limit ?? 0)}
                </Box>
              ),
              width: '16%',
            },
            {
              header: p.t('pages.users_page.created'),
              cell: (u: User) => (
                <Box color="text-body-secondary" fontSize="body-s">
                  {new Date(u.created_at).toLocaleDateString()}
                </Box>
              ),
              width: '8%',
            },
            {
              header: p.t('pages.users_page.actions'),
              cell: (u: User) => (
                <SpaceBetween direction="horizontal" size="xs">
                  <Button variant="inline-link" onClick={() => p.openEdit(u)}>
                    {p.t('pages.users_page.edit')}
                  </Button>
                  <Button
                    variant="inline-link"
                    onClick={() => p.handleToggleStatus(u)}
                    loading={p.togglingId === u.id}
                  >
                    {u.status === 'active'
                      ? p.t('pages.users_page.toggle_suspend')
                      : p.t('pages.users_page.toggle_activate')}
                  </Button>
                </SpaceBetween>
              ),
              width: '10%',
            },
          ]}
          items={p.pagedUsers}
          header={
            <Header
              variant="h2"
              counter={p.selectedUsers.length > 0 ? `(${p.selectedUsers.length}/${p.filteredUsers.length})` : `(${p.filteredUsers.length})`}
              actions={
                p.selectedUsers.length > 0 ? (
                  <SpaceBetween direction="horizontal" size="xs">
                    <Box color="text-status-inactive" padding={{ top: 'xs' }}>
                      {p.t('pages.users_page.selected_count').replace('{n}', String(p.selectedUsers.length))}
                    </Box>
                    <ButtonDropdown
                      loading={p.bulkLoading}
                      items={[
                        { id: 'activate', text: p.t('pages.users_page.activate_selected') },
                        { id: 'suspend', text: p.t('pages.users_page.suspend_selected') },
                      ]}
                      onItemClick={({ detail }) => p.handleBulkAction(detail.id as 'activate' | 'suspend')}
                    >
                      {p.t('pages.users_page.bulk_actions')}
                    </ButtonDropdown>
                  </SpaceBetween>
                ) : undefined
              }
            >
              {p.t('pages.users_page.user_list')}
            </Header>
          }
          filter={
            <SpaceBetween direction="horizontal" size="xs">
              <TextFilter
                filteringText={p.filter}
                filteringPlaceholder={p.t('pages.users_page.search_placeholder')}
                onChange={(e) => { p.setFilter(e.detail.filteringText); p.setCurrentPage(1); }}
              />
              <Select
                selectedOption={p.statusOptions.find(o => o.value === p.statusFilter) ?? p.statusOptions[0]}
                options={p.statusOptions}
                onChange={(e) => { p.setStatusFilter(e.detail.selectedOption.value ?? ''); p.setCurrentPage(1); }}
                expandToViewport
              />
            </SpaceBetween>
          }
          pagination={
            p.pageCount > 1 ? (
              <Pagination
                currentPageIndex={p.currentPage}
                pagesCount={p.pageCount}
                onChange={e => p.setCurrentPage(e.detail.currentPageIndex)}
              />
            ) : undefined
          }
          empty={
            <Box textAlign="center" padding="l">
              <StatusIndicator type="info">{p.t('pages.users_page.no_users')}</StatusIndicator>
            </Box>
          }
        />
      </SpaceBetween>

      <CreateUserModal
        visible={p.showCreateModal}
        newUser={p.newUser}
        setNewUser={p.setNewUser}
        inviteLink={p.inviteLink}
        createError={p.createError}
        loadingDomainSettings={p.loadingDomainSettings}
        creating={p.creating}
        registrationMode={p.registrationMode}
        domainOptions={p.domainOptions}
        autoAddress={p.autoAddress}
        onDismiss={p.resetCreateUserForm}
        onCloseAfterInvite={p.resetCreateUserForm}
        onDomainChange={p.handleDomainChange}
        onCreate={p.handleCreateUser}
      />

      <EditUserModal
        visible={!!p.editUser}
        userId={p.editUser?.id ?? ''}
        companyId={p.companyId}
        username={p.editUser?.username ?? ''}
        editForm={p.editForm}
        setEditForm={p.setEditForm}
        saving={p.saving}
        saveError={p.editSaveError}
        roleOptions={p.ROLE_OPTIONS}
        onDismiss={() => { p.setEditUser(null); p.setEditSaveError(''); }}
        onSave={p.handleEditSave}
      />

      <OffboardUserModal
        visible={!!p.offboardTarget}
        targetUser={p.offboardTarget}
        offboarding={p.offboarding}
        onDismiss={() => p.setOffboardTarget(null)}
        onConfirm={p.handleOffboard}
      />

      <ImportUsersModal
        visible={p.showImportModal}
        importing={p.importing}
        importResult={p.importResult}
        fileInputRef={p.fileInputRef}
        onDismiss={() => { p.setShowImportModal(false); p.setImportResult(null); }}
        onImportFile={p.handleImportCSV}
      />
    </ContentLayout>
  );
}
