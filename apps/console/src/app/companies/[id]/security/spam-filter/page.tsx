'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Select,
  Toggle,
  Box,
  Spinner,
  Flashbar,
  Badge,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useSpamFilter } from './useSpamFilter';
import { SpamFilterStats } from './SpamFilterStats';
import { SpamFilterEventsTable } from './SpamFilterEventsTable';
import { SpamFilterPolicyEditor } from './SpamFilterPolicyEditor';

export default function SpamFilterPage() {
  const { t, locale } = useI18n();
  const {
    policy,
    setPolicy,
    savedPolicyJson,
    loading,
    refreshing,
    saving,
    notifications,
    events,
    stats,
    loadingDomains,
    selectedScope,
    setSelectedScope,
    lastUpdated,
    fetchPolicy,
    handleSave,
    scopeOptions,
    activeDomainId,
    isDirty,
  } = useSpamFilter();

  const activeScopeLabel = activeDomainId
    ? t('pages.spam_filter_page.scope_domain')
    : t('pages.spam_filter_page.scope_company');

  if (loading) {
    return (
      <ContentLayout header={<Header variant="h1">{t('pages.spam_filter_page.title')}</Header>}>
        <Box textAlign="center" padding="xl"><Spinner /></Box>
      </ContentLayout>
    );
  }

  return (
    <ContentLayout
      header={
        <Header
          variant="h1"
          description={t('pages.spam_filter_page.description')}
          actions={
            <Button onClick={fetchPolicy} loading={refreshing}>
              {t('pages.spam_filter_page.refresh')}
            </Button>
          }
        >
          {t('pages.spam_filter_page.title')}
        </Header>
      }
    >
      <SpaceBetween size="l">
        {notifications.length > 0 && <Flashbar items={notifications} />}

        {/* Master toggle + scope selector */}
        <Container header={<Header variant="h2">{t('pages.spam_filter_page.general_section')}</Header>}>
          <SpaceBetween size="m">
            <FormField
              label={t('pages.spam_filter_page.scope_label')}
              description={t('pages.spam_filter_page.scope_desc')}
            >
              <SpaceBetween direction="horizontal" size="xs">
                <Select
                  selectedOption={selectedScope}
                  options={scopeOptions}
                  loadingText={t('pages.spam_filter_page.loading_domains')}
                  placeholder={t('pages.spam_filter_page.scope_placeholder')}
                  statusType={loadingDomains ? 'loading' : 'finished'}
                  onChange={event => setSelectedScope(event.detail.selectedOption)}
                />
                <Button onClick={fetchPolicy} loading={refreshing}>{t('pages.spam_filter_page.scope_load')}</Button>
                <Badge color={activeDomainId.trim() ? 'blue' : 'green'}>
                  {activeScopeLabel}
                </Badge>
                {isDirty && <Badge color="severity-medium">{t('pages.spam_filter_page.unsaved_changes')}</Badge>}
              </SpaceBetween>
            </FormField>
            <FormField label={t('pages.spam_filter_page.enabled_label')} description={t('pages.spam_filter_page.enabled_desc')}>
              <Toggle
                checked={policy.enabled}
                onChange={e => setPolicy(p => ({ ...p, enabled: e.detail.checked }))}
              >
                {policy.enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
          </SpaceBetween>
        </Container>

        {policy.enabled && (
          <>
            <SpamFilterStats stats={stats} lastUpdated={lastUpdated} t={t} />

            <SpamFilterPolicyEditor
              policy={policy}
              onPolicyChange={setPolicy}
              saving={saving}
              isDirty={isDirty}
              onSave={handleSave}
              savedPolicyJson={savedPolicyJson}
              t={t}
              locale={locale}
            />

            <SpamFilterEventsTable
              events={events}
              lastUpdated={lastUpdated}
              refreshing={refreshing}
              onRefresh={fetchPolicy}
              locale={locale}
              t={t}
            />
          </>
        )}

        {!policy.enabled && (
          <Box float="right">
            <SpaceBetween direction="horizontal" size="xs">
              {isDirty && <Box color="text-status-warning">{t('pages.spam_filter_page.unsaved_changes')}</Box>}
              <Button variant="normal" onClick={fetchPolicy} disabled={!isDirty || refreshing}>
                {t('pages.spam_filter_page.discard_changes')}
              </Button>
              <Button variant="primary" onClick={handleSave} loading={saving} disabled={!isDirty}>
                {t('pages.spam_filter_page.save')}
              </Button>
            </SpaceBetween>
          </Box>
        )}
      </SpaceBetween>
    </ContentLayout>
  );
}
