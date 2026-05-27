'use client';
import {
  Container,
  Header,
  Box,
  SpaceBetween,
  Spinner,
  Button,
  Alert,
  FormField,
  Toggle,
  Select,
  Checkbox,
  ColumnLayout,
  type SelectProps,
} from '@cloudscape-design/components';
import { DomainMCPPolicy, DomainMCPPolicyConfig, DEFAULT_MCP_SCOPES } from './domainDetailTypes';

const MCP_AUDIT_LEVEL_OPTIONS: SelectProps.Option[] = [
  { label: 'Off', value: 'off' },
  { label: 'Basic', value: 'basic' },
  { label: 'Full', value: 'full' },
];

interface Props {
  mcpPolicy: DomainMCPPolicy;
  mcpPolicyConfig: DomainMCPPolicyConfig | null;
  mcpPolicyLoading: boolean;
  mcpPolicySaving: boolean;
  mcpPolicyError: string;
  mcpPolicySaved: boolean;
  onPolicyChange: (patch: Partial<DomainMCPPolicy>) => void;
  onScopeChange: (scope: string, checked: boolean) => void;
  onRefresh: () => void;
  onSave: () => void;
  onDismissError: () => void;
  onDismissSaved: () => void;
  t: (key: string, fallback?: string) => string;
}

export function DomainMCPTab({
  mcpPolicy,
  mcpPolicyConfig,
  mcpPolicyLoading,
  mcpPolicySaving,
  mcpPolicyError,
  mcpPolicySaved,
  onPolicyChange,
  onScopeChange,
  onRefresh,
  onSave,
  onDismissError,
  onDismissSaved,
  t,
}: Props) {
  return (
    <SpaceBetween size="l">
      {mcpPolicyError ? (
        <Alert type="error" dismissible onDismiss={onDismissError}>
          {mcpPolicyError}
        </Alert>
      ) : null}
      {mcpPolicySaved ? (
        <Alert type="success" dismissible onDismiss={onDismissSaved}>
          {t('pages.domain_detail.mcp_policy_saved', 'MCP policy saved.')}
        </Alert>
      ) : null}

      <Container
        header={
          <Header
            variant="h2"
            description={t('pages.domain_detail.mcp_policy_desc', 'Control user-facing MCP automation for this domain.')}
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button iconName="refresh" onClick={onRefresh} loading={mcpPolicyLoading}>
                  {t('common.refresh')}
                </Button>
                <Button variant="primary" onClick={onSave} loading={mcpPolicySaving}>
                  {t('common.save')}
                </Button>
              </SpaceBetween>
            }
          >
            {t('pages.domain_detail.mcp_policy_title', 'Domain MCP policy')}
          </Header>
        }
      >
        {mcpPolicyLoading ? (
          <Box textAlign="center" padding="xl"><Spinner /></Box>
        ) : (
          <SpaceBetween size="m">
            <ColumnLayout columns={2}>
              <FormField
                label={t('pages.domain_detail.mcp_enabled', 'Domain MCP enabled')}
                description={t('pages.domain_detail.mcp_enabled_desc', 'Disable to block all user MCP automation for this domain.')}
              >
                <Toggle
                  checked={mcpPolicy.enabled}
                  onChange={(e) => onPolicyChange({ enabled: e.detail.checked })}
                >
                  {mcpPolicy.enabled ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>

              <FormField
                label={t('pages.domain_detail.mcp_user_keys', 'Allow user access keys')}
                description={t('pages.domain_detail.mcp_user_keys_desc', 'Permit users to issue user-scoped MCP keys.')}
              >
                <Toggle
                  checked={mcpPolicy.allow_user_access_keys}
                  onChange={(e) => onPolicyChange({ allow_user_access_keys: e.detail.checked })}
                >
                  {mcpPolicy.allow_user_access_keys ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>

              <FormField
                label={t('pages.domain_detail.mcp_bypass_mode', 'Allow bypass mode')}
                description={t('pages.domain_detail.mcp_bypass_mode_desc', 'Permit keys that bypass per-tool confirmations where users select bypass mode.')}
              >
                <Toggle
                  checked={mcpPolicy.allow_bypass_mode}
                  onChange={(e) => onPolicyChange({ allow_bypass_mode: e.detail.checked })}
                >
                  {mcpPolicy.allow_bypass_mode ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>

              <FormField
                label={t('pages.domain_detail.mcp_force_notice', 'Force generated-mail notice')}
                description={t('pages.domain_detail.mcp_force_notice_desc', 'Require MCP-written outbound mail to carry the generated-mail notice.')}
              >
                <Toggle
                  checked={mcpPolicy.force_generated_mail_notice}
                  onChange={(e) => onPolicyChange({ force_generated_mail_notice: e.detail.checked })}
                >
                  {mcpPolicy.force_generated_mail_notice ? t('common.enabled') : t('common.disabled')}
                </Toggle>
              </FormField>
            </ColumnLayout>

            <FormField
              label={t('pages.domain_detail.mcp_audit_level', 'Audit level')}
              description={t('pages.domain_detail.mcp_audit_level_desc', 'Set expected audit verbosity for MCP activity.')}
            >
              <Select
                selectedOption={MCP_AUDIT_LEVEL_OPTIONS.find((option) => option.value === mcpPolicy.audit_level) ?? {
                  label: mcpPolicy.audit_level,
                  value: mcpPolicy.audit_level,
                }}
                options={MCP_AUDIT_LEVEL_OPTIONS}
                onChange={(e) => onPolicyChange({ audit_level: e.detail.selectedOption.value ?? 'full' })}
              />
            </FormField>
          </SpaceBetween>
        )}
      </Container>

      <Container
        header={
          <Header
            variant="h2"
            counter={`(${mcpPolicy.allowed_scopes.length}/${DEFAULT_MCP_SCOPES.length})`}
            actions={
              <SpaceBetween direction="horizontal" size="xs">
                <Button onClick={() => onPolicyChange({ allowed_scopes: [] })}>
                  {t('pages.domain_detail.mcp_clear_scopes', 'Clear')}
                </Button>
                <Button onClick={() => onPolicyChange({ allowed_scopes: DEFAULT_MCP_SCOPES })}>
                  {t('pages.domain_detail.mcp_select_all_scopes', 'Select all')}
                </Button>
              </SpaceBetween>
            }
          >
            {t('pages.domain_detail.mcp_allowed_scopes', 'Allowed scopes')}
          </Header>
        }
      >
        <ColumnLayout columns={3}>
          {DEFAULT_MCP_SCOPES.map((scope) => (
            <Checkbox
              key={scope}
              checked={mcpPolicy.allowed_scopes.includes(scope)}
              onChange={(e) => onScopeChange(scope, e.detail.checked)}
            >
              <Box variant="code">{scope}</Box>
            </Checkbox>
          ))}
        </ColumnLayout>
      </Container>

      {mcpPolicyConfig?.UpdatedAt ? (
        <Box color="text-body-secondary" fontSize="body-s">
          {t('pages.domain_detail.mcp_policy_last_updated', 'Last updated')}: {new Date(mcpPolicyConfig.UpdatedAt).toLocaleString()}
          {typeof mcpPolicyConfig.Version === 'number' ? ` · v${mcpPolicyConfig.Version}` : ''}
        </Box>
      ) : null}
    </SpaceBetween>
  );
}
