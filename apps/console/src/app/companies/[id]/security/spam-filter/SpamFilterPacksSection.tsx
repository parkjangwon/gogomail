'use client';

import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Select,
  SelectProps,
  Textarea,
  Toggle,
  Button,
  Box,
  Alert,
  ColumnLayout,
  Badge,
} from '@cloudscape-design/components';
import { DataTable } from '@/components/DataTable';
import { SpamFilterPolicy, FilterRule, builtinFilterPacks } from './spamFilterTypes';

interface SpamFilterPacksSectionProps {
  policy: SpamFilterPolicy;
  activePackCount: number;
  customPackCount: number;
  // new pack form
  newPackId: string;
  newPackName: string;
  newPackPhrase: string;
  newPackScore: string;
  onNewPackIdChange: (v: string) => void;
  onNewPackNameChange: (v: string) => void;
  onNewPackPhraseChange: (v: string) => void;
  onNewPackScoreChange: (v: string) => void;
  onAddCustomPack: () => void;
  onRemoveCustomPack: (packId: string) => void;
  onSetFilterPackEnabled: (packId: string, enabled: boolean) => void;
  // rule editor
  selectedCustomPackId: string;
  onSelectedCustomPackIdChange: (id: string) => void;
  newRuleId: string;
  newRuleType: SelectProps.Option;
  newRuleTarget: SelectProps.Option;
  newRulePatterns: string;
  newRuleScore: string;
  newRuleAction: SelectProps.Option;
  onNewRuleIdChange: (v: string) => void;
  onNewRuleTypeChange: (opt: SelectProps.Option) => void;
  onNewRuleTargetChange: (opt: SelectProps.Option) => void;
  onNewRulePatternsChange: (v: string) => void;
  onNewRuleScoreChange: (v: string) => void;
  onNewRuleActionChange: (opt: SelectProps.Option) => void;
  onAddRuleToSelectedPack: () => void;
  onToggleRuleInPack: (packId: string, ruleId: string) => void;
  onRemoveRuleFromPack: (packId: string, ruleId: string) => void;
  ruleTypeOptions: SelectProps.Option[];
  ruleTargetOptions: SelectProps.Option[];
  ruleActionOptions: SelectProps.Option[];
  formatRulesCount: (count: number) => string;
  t: (key: string, defaultValue?: string) => string;
}

export function SpamFilterPacksSection({
  policy,
  activePackCount,
  customPackCount,
  newPackId,
  newPackName,
  newPackPhrase,
  newPackScore,
  onNewPackIdChange,
  onNewPackNameChange,
  onNewPackPhraseChange,
  onNewPackScoreChange,
  onAddCustomPack,
  onRemoveCustomPack,
  onSetFilterPackEnabled,
  selectedCustomPackId,
  onSelectedCustomPackIdChange,
  newRuleId,
  newRuleType,
  newRuleTarget,
  newRulePatterns,
  newRuleScore,
  newRuleAction,
  onNewRuleIdChange,
  onNewRuleTypeChange,
  onNewRuleTargetChange,
  onNewRulePatternsChange,
  onNewRuleScoreChange,
  onNewRuleActionChange,
  onAddRuleToSelectedPack,
  onToggleRuleInPack,
  onRemoveRuleFromPack,
  ruleTypeOptions,
  ruleTargetOptions,
  ruleActionOptions,
  formatRulesCount,
  t,
}: SpamFilterPacksSectionProps) {
  const customPackOptions: SelectProps.Option[] = (policy.filter_packs?.custom_packs ?? []).map(pack => ({
    value: pack.id,
    label: pack.name,
    description: pack.id,
  }));
  const selectedCustomPack = (policy.filter_packs?.custom_packs ?? []).find(pack => pack.id === selectedCustomPackId) ?? null;

  return (
    <Container
      header={
        <Header
          variant="h2"
          counter={`(${activePackCount}/${builtinFilterPacks.length + customPackCount})`}
        >
          {t('pages.spam_filter_page.filter_packs_section')}
        </Header>
      }
    >
      <SpaceBetween size="m">
        <Alert type="info">
          {t('pages.spam_filter_page.filter_packs_notice')}
        </Alert>
        <ColumnLayout columns={2}>
          {builtinFilterPacks.map(pack => {
            const enabled = (policy.filter_packs?.enabled_pack_ids ?? []).includes(pack.id);
            return (
              <FormField key={pack.id} label={t(`pages.spam_filter_page.${pack.nameKey}`, pack.name)} description={t(`pages.spam_filter_page.${pack.descriptionKey}`, pack.description)}>
                <SpaceBetween size="xs">
                  <Toggle checked={enabled} onChange={e => onSetFilterPackEnabled(pack.id, e.detail.checked)}>
                    {enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
                  </Toggle>
                  <SpaceBetween direction="horizontal" size="xs">
                    <Badge color="blue">{t(`pages.spam_filter_page.${pack.categoryKey}`, pack.category)}</Badge>
                    <Badge color="grey">{formatRulesCount(pack.rules.length)}</Badge>
                  </SpaceBetween>
                </SpaceBetween>
              </FormField>
            );
          })}
        </ColumnLayout>

        <FormField label={t('pages.spam_filter_page.custom_packs_label')} description={t('pages.spam_filter_page.custom_packs_desc')}>
          <SpaceBetween size="s">
            {(policy.filter_packs?.custom_packs ?? []).length === 0 && (
              <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_custom_packs')}</Box>
            )}
            {(policy.filter_packs?.custom_packs ?? []).map(pack => {
              const enabled = (policy.filter_packs?.enabled_pack_ids ?? []).includes(pack.id);
              return (
                <SpaceBetween key={pack.id} direction="horizontal" size="xs">
                  <Badge color={enabled ? 'green' : 'grey'}>{pack.id}</Badge>
                  <Box>{pack.name}</Box>
                  <Button variant="inline-link" onClick={() => onSetFilterPackEnabled(pack.id, !enabled)}>
                    {enabled ? t('pages.spam_filter_page.disable') : t('pages.spam_filter_page.enable')}
                  </Button>
                  <Button variant="inline-link" onClick={() => onRemoveCustomPack(pack.id)}>
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              );
            })}
            <ColumnLayout columns={4}>
              <Input value={newPackId} onChange={e => onNewPackIdChange(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_id_placeholder')} />
              <Input value={newPackName} onChange={e => onNewPackNameChange(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_name_placeholder')} />
              <Input value={newPackPhrase} onChange={e => onNewPackPhraseChange(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_phrase_placeholder')} />
              <Input type="number" value={newPackScore} onChange={e => onNewPackScoreChange(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_score_placeholder')} />
            </ColumnLayout>
            <Button onClick={onAddCustomPack}>{t('pages.spam_filter_page.add_custom_pack')}</Button>
          </SpaceBetween>
        </FormField>

        {(policy.filter_packs?.custom_packs ?? []).length > 0 && (
          <Container header={<Header variant="h3">{t('pages.spam_filter_page.rule_editor_section')}</Header>}>
            <SpaceBetween size="m">
              <ColumnLayout columns={3}>
                <FormField label={t('pages.spam_filter_page.rule_pack_label')}>
                  <Select
                    selectedOption={customPackOptions.find(option => option.value === selectedCustomPackId) ?? null}
                    options={customPackOptions}
                    placeholder={t('pages.spam_filter_page.rule_pack_placeholder')}
                    onChange={event => onSelectedCustomPackIdChange(String(event.detail.selectedOption.value ?? ''))}
                  />
                </FormField>
                <FormField label={t('pages.spam_filter_page.rule_type_label')}>
                  <Select
                    selectedOption={newRuleType}
                    options={ruleTypeOptions}
                    onChange={event => onNewRuleTypeChange(event.detail.selectedOption)}
                  />
                </FormField>
                <FormField label={t('pages.spam_filter_page.rule_target_label')}>
                  <Select
                    selectedOption={newRuleTarget}
                    options={ruleTargetOptions}
                    disabled={newRuleType.value !== 'phrase'}
                    onChange={event => onNewRuleTargetChange(event.detail.selectedOption)}
                  />
                </FormField>
              </ColumnLayout>
              <ColumnLayout columns={3}>
                <FormField label={t('pages.spam_filter_page.rule_id_label')}>
                  <Input value={newRuleId} onChange={event => onNewRuleIdChange(event.detail.value)} placeholder={t('pages.spam_filter_page.rule_id_placeholder')} />
                </FormField>
                <FormField label={t('pages.spam_filter_page.rule_score_label')}>
                  <Input type="number" value={newRuleScore} onChange={event => onNewRuleScoreChange(event.detail.value)} />
                </FormField>
                <FormField label={t('pages.spam_filter_page.rule_action_label')}>
                  <Select
                    selectedOption={newRuleAction}
                    options={ruleActionOptions}
                    onChange={event => onNewRuleActionChange(event.detail.selectedOption)}
                  />
                </FormField>
              </ColumnLayout>
              <FormField
                label={t('pages.spam_filter_page.rule_patterns_label')}
                description={t('pages.spam_filter_page.rule_patterns_desc')}
              >
                <Textarea
                  value={newRulePatterns}
                  onChange={event => onNewRulePatternsChange(event.detail.value)}
                  placeholder={t('pages.spam_filter_page.rule_patterns_placeholder')}
                  rows={4}
                />
              </FormField>
              <Button onClick={onAddRuleToSelectedPack} disabled={!selectedCustomPackId}>
                {t('pages.spam_filter_page.add_rule')}
              </Button>

              {selectedCustomPack && (
                <DataTable
                  pageSize={10}
                  searchPlaceholder={t('pages.spam_filter_page.rules_search')}
                  columnDefinitions={[
                    {
                      header: t('pages.spam_filter_page.col_rule'),
                      cell: (rule: FilterRule) => (
                        <SpaceBetween size="xxs">
                          <Box>{rule.id}</Box>
                          <SpaceBetween direction="horizontal" size="xs">
                            <Badge color={rule.enabled ? 'green' : 'grey'}>{rule.enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}</Badge>
                            <Badge color="blue">{rule.type}</Badge>
                          </SpaceBetween>
                        </SpaceBetween>
                      ),
                      width: '24%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_patterns'),
                      cell: (rule: FilterRule) => (rule.patterns ?? []).slice(0, 3).join(', ') || '—',
                      width: '34%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_score'),
                      cell: (rule: FilterRule) => rule.score.toFixed(1),
                      width: '10%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_action'),
                      cell: (rule: FilterRule) => rule.action || t('pages.spam_filter_page.rule_action_score_short'),
                      width: '14%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_manage'),
                      cell: (rule: FilterRule) => (
                        <SpaceBetween direction="horizontal" size="xs">
                          <Button variant="inline-link" onClick={() => onToggleRuleInPack(selectedCustomPack.id, rule.id)}>
                            {rule.enabled ? t('pages.spam_filter_page.disable') : t('pages.spam_filter_page.enable')}
                          </Button>
                          <Button variant="inline-link" onClick={() => onRemoveRuleFromPack(selectedCustomPack.id, rule.id)}>
                            {t('common.delete')}
                          </Button>
                        </SpaceBetween>
                      ),
                      width: '18%',
                    },
                  ]}
                  items={selectedCustomPack.rules}
                  header={<Header variant="h3" counter={`(${selectedCustomPack.rules.length})`}>{selectedCustomPack.name}</Header>}
                />
              )}
            </SpaceBetween>
          </Container>
        )}
      </SpaceBetween>
    </Container>
  );
}
