'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Select,
  SelectProps,
  Textarea,
  Toggle,
  RadioGroup,
  Box,
  Alert,
  ColumnLayout,
  Badge,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { DataTable } from '@/components/DataTable';
import { SpamFilterPolicy, FilterPack, FilterRule, builtinFilterPacks } from './spamFilterTypes';

interface SpamFilterPolicyEditorProps {
  policy: SpamFilterPolicy;
  onPolicyChange: (p: SpamFilterPolicy) => void;
  saving: boolean;
  isDirty: boolean;
  onSave: () => void;
  savedPolicyJson: string;
  t: (key: string, defaultValue?: string) => string;
  locale: string;
}

export function SpamFilterPolicyEditor({ policy, onPolicyChange, saving, isDirty, onSave, savedPolicyJson, t, locale }: SpamFilterPolicyEditorProps) {
  const setPolicy = (updater: SpamFilterPolicy | ((prev: SpamFilterPolicy) => SpamFilterPolicy)) => {
    if (typeof updater === 'function') {
      onPolicyChange(updater(policy));
    } else {
      onPolicyChange(updater);
    }
  };

  const [newBlockedExt, setNewBlockedExt] = useState('');
  const [newBlockedSender, setNewBlockedSender] = useState('');
  const [newAllowedSender, setNewAllowedSender] = useState('');
  const [newRBLZone, setNewRBLZone] = useState('');
  const [newPackId, setNewPackId] = useState('');
  const [newPackName, setNewPackName] = useState('');
  const [newPackPhrase, setNewPackPhrase] = useState('');
  const [newPackScore, setNewPackScore] = useState('4');
  const [selectedCustomPackId, setSelectedCustomPackId] = useState('');
  const [newRuleId, setNewRuleId] = useState('');
  const [newRuleType, setNewRuleType] = useState<SelectProps.Option>({ value: 'phrase', label: t('pages.spam_filter_page.rule_type_phrase') });
  const [newRuleTarget, setNewRuleTarget] = useState<SelectProps.Option>({ value: 'subject_body', label: t('pages.spam_filter_page.rule_target_subject_body') });
  const [newRulePatterns, setNewRulePatterns] = useState('');
  const [newRuleScore, setNewRuleScore] = useState('4');
  const [newRuleAction, setNewRuleAction] = useState<SelectProps.Option>({ value: '', label: t('pages.spam_filter_page.rule_action_score') });

  const addToList = (field: keyof SpamFilterPolicy, value: string, setter: (v: string) => void) => {
    const trimmed = value.trim();
    if (!trimmed) return;
    setPolicy(p => ({ ...p, [field]: [...(p[field] as string[]), trimmed] }));
    setter('');
  };

  const removeFromList = (field: keyof SpamFilterPolicy, index: number) => {
    setPolicy(p => ({ ...p, [field]: (p[field] as string[]).filter((_, i) => i !== index) }));
  };

  const setFilterPackEnabled = (packId: string, enabled: boolean) => {
    setPolicy(p => {
      const current = p.filter_packs?.enabled_pack_ids ?? [];
      const nextIds = enabled
        ? Array.from(new Set([...current, packId]))
        : current.filter(id => id !== packId);
      return {
        ...p,
        filter_packs: {
          enabled_pack_ids: nextIds,
          custom_packs: p.filter_packs?.custom_packs ?? [],
        },
      };
    });
  };

  const addCustomPack = () => {
    const id = newPackId.trim().toLowerCase();
    const phrase = newPackPhrase.trim();
    if (!id) return;
    const safeRuleId = `${id}-phrase`.replace(/[^a-z0-9._-]/g, '-').slice(0, 80);
    const score = Math.max(0.5, Math.min(20, parseFloat(newPackScore) || 4));
    const pack: FilterPack = {
      id,
      version: 'custom',
      name: newPackName.trim() || id,
      description: 'Tenant managed custom filter pack.',
      category: 'custom',
      source: 'custom',
      enabled: true,
      rules: phrase ? [{
        id: safeRuleId,
        type: 'phrase',
        target: 'subject_body',
        patterns: [phrase],
        score,
        enabled: true,
      }] : [],
    };
    setPolicy(p => ({
      ...p,
      filter_packs: {
        enabled_pack_ids: Array.from(new Set([...(p.filter_packs?.enabled_pack_ids ?? []), id])),
        custom_packs: [...(p.filter_packs?.custom_packs ?? []).filter(existing => existing.id !== id), pack],
      },
    }));
    setSelectedCustomPackId(id);
    setNewPackId('');
    setNewPackName('');
    setNewPackPhrase('');
    setNewPackScore('4');
  };

  const removeCustomPack = (packId: string) => {
    setPolicy(p => ({
      ...p,
      filter_packs: {
        enabled_pack_ids: (p.filter_packs?.enabled_pack_ids ?? []).filter(id => id !== packId),
        custom_packs: (p.filter_packs?.custom_packs ?? []).filter(pack => pack.id !== packId),
      },
    }));
    if (selectedCustomPackId === packId) {
      setSelectedCustomPackId('');
    }
  };

  const addRuleToSelectedPack = () => {
    const packId = selectedCustomPackId.trim();
    const ruleID = newRuleId.trim().toLowerCase() || `rule-${Date.now()}`;
    if (!packId) return;
    const patterns = newRulePatterns
      .split('\n')
      .flatMap(line => line.split(','))
      .map(value => value.trim())
      .filter(Boolean);
    const type = String(newRuleType.value ?? 'phrase');
    if (type !== 'bulk_recipient' && patterns.length === 0) return;
    const rule: FilterRule = {
      id: ruleID.replace(/[^a-z0-9._-]/g, '-').slice(0, 80),
      type,
      target: String(newRuleTarget.value ?? 'subject_body'),
      patterns,
      score: Math.max(0.5, Math.min(20, parseFloat(newRuleScore) || 4)),
      enabled: true,
      action: String(newRuleAction.value ?? '') || undefined,
    };
    setPolicy(p => ({
      ...p,
      filter_packs: {
        enabled_pack_ids: p.filter_packs?.enabled_pack_ids ?? [],
        custom_packs: (p.filter_packs?.custom_packs ?? []).map(pack => (
          pack.id === packId
            ? { ...pack, rules: [...pack.rules.filter(existing => existing.id !== rule.id), rule] }
            : pack
        )),
      },
    }));
    setNewRuleId('');
    setNewRulePatterns('');
    setNewRuleScore('4');
    setNewRuleAction({ value: '', label: t('pages.spam_filter_page.rule_action_score') });
  };

  const removeRuleFromPack = (packId: string, ruleId: string) => {
    setPolicy(p => ({
      ...p,
      filter_packs: {
        enabled_pack_ids: p.filter_packs?.enabled_pack_ids ?? [],
        custom_packs: (p.filter_packs?.custom_packs ?? []).map(pack => (
          pack.id === packId
            ? { ...pack, rules: pack.rules.filter(rule => rule.id !== ruleId) }
            : pack
        )),
      },
    }));
  };

  const toggleRuleInPack = (packId: string, ruleId: string) => {
    setPolicy(p => ({
      ...p,
      filter_packs: {
        enabled_pack_ids: p.filter_packs?.enabled_pack_ids ?? [],
        custom_packs: (p.filter_packs?.custom_packs ?? []).map(pack => (
          pack.id === packId
            ? { ...pack, rules: pack.rules.map(rule => rule.id === ruleId ? { ...rule, enabled: !rule.enabled } : rule) }
            : pack
        )),
      },
    }));
  };

  const activePackCount = (policy.filter_packs?.enabled_pack_ids ?? []).length;
  const customPackCount = (policy.filter_packs?.custom_packs ?? []).length;

  const changedFields = useMemo(() => {
    if (!savedPolicyJson) return [];
    const saved = JSON.parse(savedPolicyJson) as SpamFilterPolicy;
    return [
      ['enabled', t('pages.spam_filter_page.enabled_label')],
      ['spam_threshold', t('pages.spam_filter_page.threshold_label')],
      ['virus_scan_enabled', t('pages.spam_filter_page.virus_scan_label')],
      ['strict_auth_enabled', t('pages.spam_filter_page.strict_auth_label')],
      ['rbl_check_enabled', t('pages.spam_filter_page.rbl_lookup_label')],
      ['rbl_reject_enabled', t('pages.spam_filter_page.rbl_reject_label')],
      ['rbl_zones', t('pages.spam_filter_page.rbl_zones_label')],
      ['blocked_extensions', t('pages.spam_filter_page.blocked_ext_label')],
      ['blocked_senders', t('pages.spam_filter_page.blocked_senders_label')],
      ['allowed_senders', t('pages.spam_filter_page.allowed_senders_label')],
      ['quarantine_enabled', t('pages.spam_filter_page.action_label')],
      ['max_attachment_mb', t('pages.spam_filter_page.max_attachment_label')],
      ['bulk_recipient_limit', t('pages.spam_filter_page.bulk_limit_label')],
      ['filter_packs', t('pages.spam_filter_page.filter_packs_section')],
    ].filter(([key]) => JSON.stringify(saved[key as keyof SpamFilterPolicy]) !== JSON.stringify(policy[key as keyof SpamFilterPolicy]))
      .map(([, label]) => String(label));
  }, [policy, savedPolicyJson, t]);

  const riskItems = useMemo(() => {
    const risks: string[] = [];
    if (!policy.quarantine_enabled) risks.push(t('pages.spam_filter_page.risk_reject_mode'));
    if (policy.rbl_check_enabled && policy.rbl_reject_enabled) risks.push(t('pages.spam_filter_page.risk_rbl_reject'));
    if (policy.spam_threshold <= 3) risks.push(t('pages.spam_filter_page.risk_low_threshold'));
    if (policy.allowed_senders.length > 0) risks.push(t('pages.spam_filter_page.risk_allowlist'));
    if ((policy.filter_packs?.custom_packs ?? []).some(pack => pack.rules.some(rule => rule.action === 'reject'))) risks.push(t('pages.spam_filter_page.risk_pack_reject'));
    return risks;
  }, [policy.allowed_senders.length, policy.filter_packs?.custom_packs, policy.quarantine_enabled, policy.rbl_check_enabled, policy.rbl_reject_enabled, policy.spam_threshold, t]);

  const postureLabel = riskItems.length === 0
    ? t('pages.spam_filter_page.posture_balanced')
    : riskItems.length >= 3
      ? t('pages.spam_filter_page.posture_high_risk')
      : t('pages.spam_filter_page.posture_review');

  const formatRulesCount = (count: number) => {
    if (locale === 'ko') return `${count}개 규칙`;
    if (locale === 'ja') return `${count}件のルール`;
    if (locale === 'zh-CN') return `${count}条规则`;
    return `${count} ${count === 1 ? 'rule' : 'rules'}`;
  };

  const ruleTypeOptions: SelectProps.Option[] = [
    { value: 'phrase', label: t('pages.spam_filter_page.rule_type_phrase') },
    { value: 'attachment_extension', label: t('pages.spam_filter_page.rule_type_attachment') },
    { value: 'bulk_recipient', label: t('pages.spam_filter_page.rule_type_bulk') },
    { value: 'auth_failure', label: t('pages.spam_filter_page.rule_type_auth') },
    { value: 'sender_domain', label: t('pages.spam_filter_page.rule_type_sender_domain') },
    { value: 'url_host', label: t('pages.spam_filter_page.rule_type_url_host') },
    { value: 'header_anomaly', label: t('pages.spam_filter_page.rule_type_header_anomaly') },
  ];
  const ruleTargetOptions: SelectProps.Option[] = [
    { value: 'subject_body', label: t('pages.spam_filter_page.rule_target_subject_body') },
    { value: 'subject', label: t('pages.spam_filter_page.rule_target_subject') },
    { value: 'body', label: t('pages.spam_filter_page.rule_target_body') },
  ];
  const ruleActionOptions: SelectProps.Option[] = [
    { value: '', label: t('pages.spam_filter_page.rule_action_score') },
    { value: 'quarantine', label: t('pages.spam_filter_page.rule_action_quarantine') },
    { value: 'reject', label: t('pages.spam_filter_page.rule_action_reject') },
  ];
  const customPackOptions: SelectProps.Option[] = (policy.filter_packs?.custom_packs ?? []).map(pack => ({
    value: pack.id,
    label: pack.name,
    description: pack.id,
  }));
  const selectedCustomPack = (policy.filter_packs?.custom_packs ?? []).find(pack => pack.id === selectedCustomPackId) ?? null;

  return (
    <SpaceBetween size="l">
      {/* Risk section */}
      <Container
        header={
          <Header
            variant="h2"
            actions={<Badge color={riskItems.length >= 3 ? 'red' : riskItems.length > 0 ? 'severity-medium' : 'green'}>{postureLabel}</Badge>}
          >
            {t('pages.spam_filter_page.risk_section')}
          </Header>
        }
      >
        <SpaceBetween size="s">
          {riskItems.length === 0 ? (
            <Alert type="success">{t('pages.spam_filter_page.risk_clear')}</Alert>
          ) : (
            <Alert type={riskItems.length >= 3 ? 'error' : 'warning'}>
              {t('pages.spam_filter_page.risk_intro')}
            </Alert>
          )}
          {riskItems.length > 0 && (
            <ColumnLayout columns={2} minColumnWidth={240}>
              {riskItems.map(item => (
                <Box key={item}>
                  <Badge color="severity-medium">{t('pages.spam_filter_page.review_required')}</Badge> {item}
                </Box>
              ))}
            </ColumnLayout>
          )}
          {changedFields.length > 0 && (
            <FormField label={t('pages.spam_filter_page.changed_fields_label')}>
              <SpaceBetween direction="horizontal" size="xs">
                {changedFields.map(field => <Badge key={field} color="blue">{field}</Badge>)}
              </SpaceBetween>
            </FormField>
          )}
        </SpaceBetween>
      </Container>

      {/* Spam detection */}
      <Container header={<Header variant="h2">{t('pages.spam_filter_page.detection_section')}</Header>}>
        <SpaceBetween size="m">
          <ColumnLayout columns={2}>
            <FormField
              label={t('pages.spam_filter_page.threshold_label')}
              constraintText={t('pages.spam_filter_page.threshold_hint')}
            >
              <Input
                type="number"
                value={String(policy.spam_threshold)}
                onChange={e => {
                  const v = parseInt(e.detail.value) || 1;
                  setPolicy(p => ({ ...p, spam_threshold: Math.max(1, Math.min(10, v)) }));
                }}
              />
            </FormField>
            <FormField label={t('pages.spam_filter_page.virus_scan_label')} description={t('pages.spam_filter_page.virus_scan_desc')}>
              <Toggle
                checked={policy.virus_scan_enabled}
                onChange={e => setPolicy(p => ({ ...p, virus_scan_enabled: e.detail.checked }))}
              >
                {policy.virus_scan_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
            <FormField label={t('pages.spam_filter_page.strict_auth_label')} description={t('pages.spam_filter_page.strict_auth_desc')}>
              <Toggle
                checked={policy.strict_auth_enabled}
                onChange={e => setPolicy(p => ({ ...p, strict_auth_enabled: e.detail.checked }))}
              >
                {policy.strict_auth_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
            <FormField
              label={t('pages.spam_filter_page.bulk_limit_label')}
              constraintText={t('pages.spam_filter_page.bulk_limit_hint')}
            >
              <Input
                type="number"
                value={String(policy.bulk_recipient_limit)}
                onChange={e => {
                  const v = parseInt(e.detail.value) || 1;
                  setPolicy(p => ({ ...p, bulk_recipient_limit: Math.max(1, Math.min(500, v)) }));
                }}
              />
            </FormField>
          </ColumnLayout>

          <FormField label={t('pages.spam_filter_page.action_label')} description={t('pages.spam_filter_page.action_desc')}>
            <RadioGroup
              value={policy.quarantine_enabled ? 'quarantine' : 'reject'}
              onChange={e => setPolicy(p => ({ ...p, quarantine_enabled: e.detail.value === 'quarantine' }))}
              items={[
                { value: 'quarantine', label: t('pages.spam_filter_page.action_quarantine') },
                { value: 'reject', label: t('pages.spam_filter_page.action_reject') },
              ]}
            />
          </FormField>
        </SpaceBetween>
      </Container>

      {/* RBL */}
      <Container header={<Header variant="h2">{t('pages.spam_filter_page.rbl_section')}</Header>}>
        <SpaceBetween size="m">
          <ColumnLayout columns={2}>
            <FormField label={t('pages.spam_filter_page.rbl_lookup_label')} description={t('pages.spam_filter_page.rbl_lookup_desc')}>
              <Toggle
                checked={policy.rbl_check_enabled}
                onChange={e => setPolicy(p => ({ ...p, rbl_check_enabled: e.detail.checked }))}
              >
                {policy.rbl_check_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
            <FormField label={t('pages.spam_filter_page.rbl_reject_label')} description={t('pages.spam_filter_page.rbl_reject_desc')}>
              <Toggle
                checked={policy.rbl_reject_enabled}
                onChange={e => setPolicy(p => ({ ...p, rbl_reject_enabled: e.detail.checked }))}
              >
                {policy.rbl_reject_enabled ? t('pages.spam_filter_page.enabled_on') : t('pages.spam_filter_page.enabled_off')}
              </Toggle>
            </FormField>
          </ColumnLayout>
          <FormField label={t('pages.spam_filter_page.rbl_zones_label')} description={t('pages.spam_filter_page.rbl_zones_desc')}>
            <SpaceBetween size="xs">
              {policy.rbl_zones.length === 0 && (
                <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_rbl_zones')}</Box>
              )}
              <SpaceBetween direction="horizontal" size="xs">
                {policy.rbl_zones.map((zone, i) => (
                  <SpaceBetween key={zone} direction="horizontal" size="xs">
                    <Badge color="blue">{zone}</Badge>
                    <Button variant="inline-link" onClick={() => removeFromList('rbl_zones', i)}>
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                ))}
              </SpaceBetween>
              <SpaceBetween direction="horizontal" size="xs">
                <Input
                  value={newRBLZone}
                  onChange={e => setNewRBLZone(e.detail.value)}
                  placeholder="zen.example-rbl.test"
                />
                <Button onClick={() => addToList('rbl_zones', newRBLZone, setNewRBLZone)}>
                  {t('common.add')}
                </Button>
              </SpaceBetween>
            </SpaceBetween>
          </FormField>
        </SpaceBetween>
      </Container>

      {/* Filter packs */}
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
                    <Toggle checked={enabled} onChange={e => setFilterPackEnabled(pack.id, e.detail.checked)}>
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
                    <Button variant="inline-link" onClick={() => setFilterPackEnabled(pack.id, !enabled)}>
                      {enabled ? t('pages.spam_filter_page.disable') : t('pages.spam_filter_page.enable')}
                    </Button>
                    <Button variant="inline-link" onClick={() => removeCustomPack(pack.id)}>
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                );
              })}
              <ColumnLayout columns={4}>
                <Input value={newPackId} onChange={e => setNewPackId(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_id_placeholder')} />
                <Input value={newPackName} onChange={e => setNewPackName(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_name_placeholder')} />
                <Input value={newPackPhrase} onChange={e => setNewPackPhrase(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_phrase_placeholder')} />
                <Input type="number" value={newPackScore} onChange={e => setNewPackScore(e.detail.value)} placeholder={t('pages.spam_filter_page.pack_score_placeholder')} />
              </ColumnLayout>
              <Button onClick={addCustomPack}>{t('pages.spam_filter_page.add_custom_pack')}</Button>
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
                      onChange={event => setSelectedCustomPackId(String(event.detail.selectedOption.value ?? ''))}
                    />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.rule_type_label')}>
                    <Select
                      selectedOption={newRuleType}
                      options={ruleTypeOptions}
                      onChange={event => setNewRuleType(event.detail.selectedOption)}
                    />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.rule_target_label')}>
                    <Select
                      selectedOption={newRuleTarget}
                      options={ruleTargetOptions}
                      disabled={newRuleType.value !== 'phrase'}
                      onChange={event => setNewRuleTarget(event.detail.selectedOption)}
                    />
                  </FormField>
                </ColumnLayout>
                <ColumnLayout columns={3}>
                  <FormField label={t('pages.spam_filter_page.rule_id_label')}>
                    <Input value={newRuleId} onChange={event => setNewRuleId(event.detail.value)} placeholder={t('pages.spam_filter_page.rule_id_placeholder')} />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.rule_score_label')}>
                    <Input type="number" value={newRuleScore} onChange={event => setNewRuleScore(event.detail.value)} />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.rule_action_label')}>
                    <Select
                      selectedOption={newRuleAction}
                      options={ruleActionOptions}
                      onChange={event => setNewRuleAction(event.detail.selectedOption)}
                    />
                  </FormField>
                </ColumnLayout>
                <FormField
                  label={t('pages.spam_filter_page.rule_patterns_label')}
                  description={t('pages.spam_filter_page.rule_patterns_desc')}
                >
                  <Textarea
                    value={newRulePatterns}
                    onChange={event => setNewRulePatterns(event.detail.value)}
                    placeholder={t('pages.spam_filter_page.rule_patterns_placeholder')}
                    rows={4}
                  />
                </FormField>
                <Button onClick={addRuleToSelectedPack} disabled={!selectedCustomPackId}>
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
                            <Button variant="inline-link" onClick={() => toggleRuleInPack(selectedCustomPack.id, rule.id)}>
                              {rule.enabled ? t('pages.spam_filter_page.disable') : t('pages.spam_filter_page.enable')}
                            </Button>
                            <Button variant="inline-link" onClick={() => removeRuleFromPack(selectedCustomPack.id, rule.id)}>
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

      {/* Attachments */}
      <Container header={<Header variant="h2">{t('pages.spam_filter_page.attachments_section')}</Header>}>
        <SpaceBetween size="m">
          <FormField
            label={t('pages.spam_filter_page.max_attachment_label')}
            constraintText={t('pages.spam_filter_page.max_attachment_hint')}
          >
            <Input
              type="number"
              value={String(policy.max_attachment_mb)}
              onChange={e => setPolicy(p => ({ ...p, max_attachment_mb: parseInt(e.detail.value) || 0 }))}
            />
          </FormField>

          <FormField label={t('pages.spam_filter_page.blocked_ext_label')} description={t('pages.spam_filter_page.blocked_ext_desc')}>
            <SpaceBetween size="xs">
              <SpaceBetween direction="horizontal" size="xs">
                {policy.blocked_extensions.map((ext, i) => (
                  <SpaceBetween key={i} direction="horizontal" size="xs">
                    <Badge color="red">{ext}</Badge>
                    <Button variant="inline-link" onClick={() => removeFromList('blocked_extensions', i)}>
                      {t('common.delete')}
                    </Button>
                  </SpaceBetween>
                ))}
              </SpaceBetween>
              <SpaceBetween direction="horizontal" size="xs">
                <Input
                  value={newBlockedExt}
                  onChange={e => setNewBlockedExt(e.detail.value)}
                  placeholder=".exe"
                />
                <Button onClick={() => addToList('blocked_extensions', newBlockedExt, setNewBlockedExt)}>
                  {t('common.add')}
                </Button>
              </SpaceBetween>
            </SpaceBetween>
          </FormField>
        </SpaceBetween>
      </Container>

      {/* Sender lists */}
      <Container header={<Header variant="h2">{t('pages.spam_filter_page.senders_section')}</Header>}>
        <SpaceBetween size="m">
          <FormField label={t('pages.spam_filter_page.blocked_senders_label')} description={t('pages.spam_filter_page.blocked_senders_desc')}>
            <SpaceBetween size="xs">
              {policy.blocked_senders.length === 0 && (
                <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_blocked_senders')}</Box>
              )}
              {policy.blocked_senders.map((s, i) => (
                <SpaceBetween key={i} direction="horizontal" size="xs">
                  <Badge color="red">{s}</Badge>
                  <Button variant="inline-link" onClick={() => removeFromList('blocked_senders', i)}>
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ))}
              <SpaceBetween direction="horizontal" size="xs">
                <Input
                  value={newBlockedSender}
                  onChange={e => setNewBlockedSender(e.detail.value)}
                  placeholder="spam@example.com or @domain.com"
                />
                <Button onClick={() => addToList('blocked_senders', newBlockedSender, setNewBlockedSender)}>
                  {t('common.add')}
                </Button>
              </SpaceBetween>
            </SpaceBetween>
          </FormField>

          <FormField label={t('pages.spam_filter_page.allowed_senders_label')} description={t('pages.spam_filter_page.allowed_senders_desc')}>
            <SpaceBetween size="xs">
              {policy.allowed_senders.length > 0 && (
                <Alert type="info">{t('pages.spam_filter_page.allowed_senders_warning')}</Alert>
              )}
              {policy.allowed_senders.length === 0 && (
                <Box color="text-body-secondary" fontSize="body-s">{t('pages.spam_filter_page.no_allowed_senders')}</Box>
              )}
              {policy.allowed_senders.map((s, i) => (
                <SpaceBetween key={i} direction="horizontal" size="xs">
                  <Badge color="green">{s}</Badge>
                  <Button variant="inline-link" onClick={() => removeFromList('allowed_senders', i)}>
                    {t('common.delete')}
                  </Button>
                </SpaceBetween>
              ))}
              <SpaceBetween direction="horizontal" size="xs">
                <Input
                  value={newAllowedSender}
                  onChange={e => setNewAllowedSender(e.detail.value)}
                  placeholder="trusted@partner.com or @trusted.com"
                />
                <Button onClick={() => addToList('allowed_senders', newAllowedSender, setNewAllowedSender)}>
                  {t('common.add')}
                </Button>
              </SpaceBetween>
            </SpaceBetween>
          </FormField>
        </SpaceBetween>
      </Container>

      {/* Save bar */}
      <Box float="right">
        <SpaceBetween direction="horizontal" size="xs">
          {isDirty && <Box color="text-status-warning">{t('pages.spam_filter_page.unsaved_changes')}</Box>}
          <Button variant="primary" onClick={onSave} loading={saving} disabled={!isDirty}>
            {t('pages.spam_filter_page.save')}
          </Button>
        </SpaceBetween>
      </Box>
    </SpaceBetween>
  );
}
