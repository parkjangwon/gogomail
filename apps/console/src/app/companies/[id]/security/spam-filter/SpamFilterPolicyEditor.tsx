'use client';

import {
  SpaceBetween,
  Button,
  SelectProps,
  Box,
} from '@cloudscape-design/components';
import { useState, useMemo } from 'react';
import { SpamFilterPolicy, FilterPack, FilterRule } from './spamFilterTypes';
import { SpamFilterAttachmentSettings } from './SpamFilterAttachmentSettings';
import { SpamFilterSenderLists } from './SpamFilterSenderLists';
import { SpamFilterRiskSection } from './SpamFilterRiskSection';
import { SpamFilterDetectionSection } from './SpamFilterDetectionSection';
import { SpamFilterRblSection } from './SpamFilterRblSection';
import { SpamFilterPacksSection } from './SpamFilterPacksSection';

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

export function SpamFilterPolicyEditor({ policy, onPolicyChange, saving, isDirty, onSave, savedPolicyJson, t }: SpamFilterPolicyEditorProps) {
  const updatePolicy = (updater: SpamFilterPolicy | ((prev: SpamFilterPolicy) => SpamFilterPolicy)) => {
    if (typeof updater === 'function') {
      onPolicyChange(updater(policy));
    } else {
      onPolicyChange(updater);
    }
  };

  // RBL zone form state
  const [newRBLZone, setNewRBLZone] = useState('');

  // Filter pack form state
  const [newPackId, setNewPackId] = useState('');
  const [newPackName, setNewPackName] = useState('');
  const [newPackPhrase, setNewPackPhrase] = useState('');
  const [newPackScore, setNewPackScore] = useState('4');
  const [selectedCustomPackId, setSelectedCustomPackId] = useState('');

  // Rule editor form state
  const [newRuleId, setNewRuleId] = useState('');
  const [newRuleType, setNewRuleType] = useState<SelectProps.Option>({ value: 'phrase', label: t('pages.spam_filter_page.rule_type_phrase') });
  const [newRuleTarget, setNewRuleTarget] = useState<SelectProps.Option>({ value: 'subject_body', label: t('pages.spam_filter_page.rule_target_subject_body') });
  const [newRulePatterns, setNewRulePatterns] = useState('');
  const [newRuleScore, setNewRuleScore] = useState('4');
  const [newRuleAction, setNewRuleAction] = useState<SelectProps.Option>({ value: '', label: t('pages.spam_filter_page.rule_action_score') });

  // --- List helpers ---
  const removeFromList = (field: keyof SpamFilterPolicy, index: number) => {
    updatePolicy(p => ({ ...p, [field]: (p[field] as string[]).filter((_, i) => i !== index) }));
  };

  // --- Filter pack handlers ---
  const setFilterPackEnabled = (packId: string, enabled: boolean) => {
    updatePolicy(p => {
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
    const existing = (policy.filter_packs?.custom_packs ?? []).find(p => p.id === id);
    if (existing) {
      alert(t('pages.spam_filter_page.pack_id_duplicate', `Pack ID "${id}" already exists. Use a unique ID.`));
      return;
    }
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
    updatePolicy(p => ({
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
    updatePolicy(p => ({
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
    const rawRuleId = newRuleId.trim().toLowerCase();
    const ruleID = rawRuleId || `rule-${Date.now()}`;
    if (!packId) return;
    if (rawRuleId) {
      const targetPack = (policy.filter_packs?.custom_packs ?? []).find(p => p.id === packId);
      if (targetPack?.rules.some(r => r.id === rawRuleId.replace(/[^a-z0-9._-]/g, '-').slice(0, 80))) {
        alert(t('pages.spam_filter_page.rule_id_duplicate', `Rule ID "${rawRuleId}" already exists in this pack.`));
        return;
      }
    }
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
    updatePolicy(p => ({
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
    updatePolicy(p => ({
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
    updatePolicy(p => ({
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

  // --- Computed values ---
  const activePackCount = (policy.filter_packs?.enabled_pack_ids ?? []).length;
  const customPackCount = (policy.filter_packs?.custom_packs ?? []).length;

  const changedFields = useMemo(() => {
    if (!savedPolicyJson) return [];
    let saved: SpamFilterPolicy;
    try {
      saved = JSON.parse(savedPolicyJson) as SpamFilterPolicy;
    } catch {
      return [];
    }
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

  const formatRulesCount = (count: number) => `${count} ${t('pages.spam_filter_page.rules_count')}`;

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

  return (
    <SpaceBetween size="l">
      <SpamFilterRiskSection
        riskItems={riskItems}
        postureLabel={postureLabel}
        changedFields={changedFields}
        t={t}
      />

      <SpamFilterDetectionSection
        policy={policy}
        onUpdatePolicy={updatePolicy}
        t={t}
      />

      <SpamFilterRblSection
        policy={policy}
        onUpdatePolicy={updatePolicy}
        newRBLZone={newRBLZone}
        onNewRBLZoneChange={setNewRBLZone}
        onAddRBLZone={() => {
          const trimmed = newRBLZone.trim();
          if (!trimmed) return;
          updatePolicy(p => ({ ...p, rbl_zones: [...p.rbl_zones, trimmed] }));
          setNewRBLZone('');
        }}
        onRemoveRBLZone={i => removeFromList('rbl_zones', i)}
        t={t}
      />

      <SpamFilterPacksSection
        policy={policy}
        activePackCount={activePackCount}
        customPackCount={customPackCount}
        newPackId={newPackId}
        newPackName={newPackName}
        newPackPhrase={newPackPhrase}
        newPackScore={newPackScore}
        onNewPackIdChange={setNewPackId}
        onNewPackNameChange={setNewPackName}
        onNewPackPhraseChange={setNewPackPhrase}
        onNewPackScoreChange={setNewPackScore}
        onAddCustomPack={addCustomPack}
        onRemoveCustomPack={removeCustomPack}
        onSetFilterPackEnabled={setFilterPackEnabled}
        selectedCustomPackId={selectedCustomPackId}
        onSelectedCustomPackIdChange={setSelectedCustomPackId}
        newRuleId={newRuleId}
        newRuleType={newRuleType}
        newRuleTarget={newRuleTarget}
        newRulePatterns={newRulePatterns}
        newRuleScore={newRuleScore}
        newRuleAction={newRuleAction}
        onNewRuleIdChange={setNewRuleId}
        onNewRuleTypeChange={setNewRuleType}
        onNewRuleTargetChange={setNewRuleTarget}
        onNewRulePatternsChange={setNewRulePatterns}
        onNewRuleScoreChange={setNewRuleScore}
        onNewRuleActionChange={setNewRuleAction}
        onAddRuleToSelectedPack={addRuleToSelectedPack}
        onToggleRuleInPack={toggleRuleInPack}
        onRemoveRuleFromPack={removeRuleFromPack}
        ruleTypeOptions={ruleTypeOptions}
        ruleTargetOptions={ruleTargetOptions}
        ruleActionOptions={ruleActionOptions}
        formatRulesCount={formatRulesCount}
        t={t}
      />

      <SpamFilterAttachmentSettings
        maxAttachmentMb={policy.max_attachment_mb}
        onMaxChange={mb => updatePolicy(p => ({ ...p, max_attachment_mb: mb }))}
        blockedExtensions={policy.blocked_extensions}
        onRemoveExtension={i => removeFromList('blocked_extensions', i)}
        onAddExtension={value => updatePolicy(p => ({ ...p, blocked_extensions: [...p.blocked_extensions, value] }))}
        t={t}
      />

      <SpamFilterSenderLists
        blockedSenders={policy.blocked_senders}
        allowedSenders={policy.allowed_senders}
        onRemoveBlockedSender={i => removeFromList('blocked_senders', i)}
        onAddBlockedSender={value => updatePolicy(p => ({ ...p, blocked_senders: [...p.blocked_senders, value] }))}
        onRemoveAllowedSender={i => removeFromList('allowed_senders', i)}
        onAddAllowedSender={value => updatePolicy(p => ({ ...p, allowed_senders: [...p.allowed_senders, value] }))}
        t={t}
      />

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
