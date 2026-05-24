'use client';

import {
  ContentLayout,
  Header,
  Container,
  SpaceBetween,
  Button,
  FormField,
  Input,
  Modal,
  Select,
  SelectProps,
  Textarea,
  Toggle,
  RadioGroup,
  Box,
  Spinner,
  Alert,
  Flashbar,
  FlashbarProps,
  ColumnLayout,
  Badge,
} from '@cloudscape-design/components';
import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import { DataTable } from '@/components/DataTable';

interface SpamFilterPolicy {
  enabled: boolean;
  spam_threshold: number;
  virus_scan_enabled: boolean;
  strict_auth_enabled: boolean;
  rbl_check_enabled: boolean;
  rbl_reject_enabled: boolean;
  rbl_zones: string[];
  blocked_extensions: string[];
  blocked_senders: string[];
  allowed_senders: string[];
  quarantine_enabled: boolean;
  max_attachment_mb: number;
  bulk_recipient_limit: number;
  filter_packs: FilterPackBundle;
}

interface FilterPackBundle {
  enabled_pack_ids: string[];
  custom_packs: FilterPack[];
}

interface FilterPack {
  id: string;
  version: string;
  name: string;
  description: string;
  category: string;
  source: 'system' | 'custom' | string;
  enabled: boolean;
  rules: FilterRule[];
}

interface FilterRule {
  id: string;
  type: 'phrase' | 'attachment_extension' | 'bulk_recipient' | 'auth_failure' | 'sender_domain' | 'url_host' | 'header_anomaly' | string;
  target?: 'subject' | 'body' | 'subject_body' | string;
  patterns: string[];
  score: number;
  enabled: boolean;
  action?: 'quarantine' | 'reject' | string;
}

interface SpamFilterEvent {
  id: string;
  created_at: string;
  from_addr?: string;
  mail_from?: string;
  rcpt_to?: string;
  subject?: string;
  flow_status: string;
  enhanced_status?: string;
  error_message?: string;
  spam_score?: number;
  spf_result?: string;
  dkim_result?: string;
  dmarc_result?: string;
}

interface SpamFilterStats {
  total_messages: number;
  filtered: number;
  rejected: number;
  delivered: number;
}

type EventFilter = 'all' | 'filtered' | 'rejected' | 'delivered';

interface DomainOption {
  value: string;
  label: string;
}

const COMPANY_SCOPE_VALUE = '__company__';

const defaultPolicy = (): SpamFilterPolicy => ({
  enabled: true,
  spam_threshold: 5,
  virus_scan_enabled: true,
  strict_auth_enabled: true,
  rbl_check_enabled: false,
  rbl_reject_enabled: true,
  rbl_zones: [],
  blocked_extensions: ['.exe', '.bat', '.cmd', '.scr', '.vbs', '.js', '.ps1', '.jar', '.docm', '.xlsm'],
  blocked_senders: [],
  allowed_senders: [],
  quarantine_enabled: true,
  max_attachment_mb: 25,
  bulk_recipient_limit: 50,
  filter_packs: {
    enabled_pack_ids: ['gogomail-core-auth', 'gogomail-core-malware', 'gogomail-core-phishing-ko', 'gogomail-core-bulk', 'gogomail-core-url', 'gogomail-core-sender'],
    custom_packs: [],
  },
});

const builtinFilterPacks: Array<FilterPack & { nameKey: string; descriptionKey: string; categoryKey: string }> = [
  {
    id: 'gogomail-core-auth',
    nameKey: 'pack_auth_name',
    descriptionKey: 'pack_auth_desc',
    categoryKey: 'pack_category_authentication',
    version: '2026.05.17',
    name: 'Core authentication defense',
    description: 'Scores suspicious SPF, DKIM, and DMARC failure combinations.',
    category: 'authentication',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'no-auth-pass', type: 'auth_failure', patterns: ['no_auth_pass'], score: 1.5, enabled: true },
      { id: 'dmarc-fail', type: 'auth_failure', patterns: ['dmarc_fail'], score: 1.5, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-malware',
    nameKey: 'pack_malware_name',
    descriptionKey: 'pack_malware_desc',
    categoryKey: 'pack_category_malware',
    version: '2026.05.17',
    name: 'Core malware attachment defense',
    description: 'Scores high-risk executable and macro attachment extensions.',
    category: 'malware',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'dangerous-extension', type: 'attachment_extension', patterns: ['.exe', '.scr', '.js', '.vbs', '.ps1', '.jar', '.docm', '.xlsm'], score: 2, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-phishing-ko',
    nameKey: 'pack_phishing_name',
    descriptionKey: 'pack_phishing_desc',
    categoryKey: 'pack_category_phishing',
    version: '2026.05.17',
    name: 'Korean and global phishing phrases',
    description: 'Scores common credential theft, urgency, and payment-lure phrases.',
    category: 'phishing',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'credential-lures', type: 'phrase', target: 'subject_body', patterns: ['verify your account', 'password expired', 'login immediately', '계정 확인', '비밀번호 만료', '긴급 로그인'], score: 1.5, enabled: true },
      { id: 'payment-lures', type: 'phrase', target: 'subject_body', patterns: ['wire transfer', 'gift card', 'crypto giveaway', '송금', '상품권', '당첨'], score: 1, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-bulk',
    nameKey: 'pack_bulk_name',
    descriptionKey: 'pack_bulk_desc',
    categoryKey: 'pack_category_bulk',
    version: '2026.05.17',
    name: 'Bulk receive pressure defense',
    description: 'Scores messages above the tenant bulk recipient threshold.',
    category: 'bulk',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'recipient-fanout', type: 'bulk_recipient', patterns: [], score: 1.5, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-url',
    nameKey: 'pack_url_name',
    descriptionKey: 'pack_url_desc',
    categoryKey: 'pack_category_phishing',
    version: '2026.05.25',
    name: 'Core URL and credential phishing defense',
    description: 'Scores disguised links, credential forms, raw IP links, and IDN/punycode link lures.',
    category: 'phishing',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'link-text-mismatch', type: 'header_anomaly', patterns: ['url_mismatch'], score: 3, enabled: true },
      { id: 'credential-form', type: 'header_anomaly', patterns: ['html_form'], score: 3, enabled: true },
      { id: 'raw-ip-url', type: 'header_anomaly', patterns: ['raw_ip_url'], score: 2, enabled: true },
      { id: 'punycode-url', type: 'header_anomaly', patterns: ['punycode_url'], score: 2, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-sender',
    nameKey: 'pack_sender_name',
    descriptionKey: 'pack_sender_desc',
    categoryKey: 'pack_category_impersonation',
    version: '2026.05.25',
    name: 'Core sender impersonation defense',
    description: 'Scores envelope/header sender mismatches and obfuscated credential-lure text.',
    category: 'impersonation',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'from-envelope-mismatch', type: 'header_anomaly', patterns: ['from_envelope_mismatch'], score: 2, enabled: true },
      { id: 'text-obfuscation', type: 'header_anomaly', patterns: ['text_obfuscation'], score: 2, enabled: true },
    ],
  },
];

export default function SpamFilterPage() {
  const { t, locale } = useI18n();
  const { currentCompany } = useCompany();
  const cid = currentCompany?.id ?? 'default';

  const companyScopeOption = useMemo<SelectProps.Option>(() => ({
    label: t('pages.spam_filter_page.scope_company'),
    value: COMPANY_SCOPE_VALUE,
    description: t('pages.spam_filter_page.scope_company_desc'),
  }), [t]);

  const [policy, setPolicy] = useState<SpamFilterPolicy>(defaultPolicy());
  const [savedPolicyJson, setSavedPolicyJson] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const hasLoadedRef = useRef(false);
  const [saving, setSaving] = useState(false);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);
  const [events, setEvents] = useState<SpamFilterEvent[]>([]);
  const [stats, setStats] = useState<SpamFilterStats | null>(null);
  const [domains, setDomains] = useState<DomainOption[]>([]);
  const [loadingDomains, setLoadingDomains] = useState(false);
  const [selectedScope, setSelectedScope] = useState<SelectProps.Option>(companyScopeOption);

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
  const [eventFilter, setEventFilter] = useState<EventFilter>('all');
  const [eventFrom, setEventFrom] = useState('');
  const [eventTo, setEventTo] = useState('');
  const [eventMinScore, setEventMinScore] = useState('');
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [detailEvent, setDetailEvent] = useState<SpamFilterEvent | null>(null);

  const scopeOptions = useMemo<SelectProps.Option[]>(() => [
    companyScopeOption,
    ...domains.map(domain => ({
      label: domain.label,
      value: domain.value,
      description: t('pages.spam_filter_page.scope_domain_desc'),
    })),
  ], [companyScopeOption, domains, t]);

  useEffect(() => {
    setSelectedScope(current => current.value === COMPANY_SCOPE_VALUE ? companyScopeOption : current);
  }, [companyScopeOption]);

  const activeDomainId = selectedScope.value === COMPANY_SCOPE_VALUE ? '' : String(selectedScope.value ?? '');

  const fetchDomains = useCallback(async () => {
    setLoadingDomains(true);
    try {
      const res = await fetch(`/api/admin/domains?limit=200&company_id=${encodeURIComponent(cid)}`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setDomains((data.domains ?? []).map((domain: { id: string; name: string }) => ({
          value: domain.id,
          label: domain.name,
        })));
      }
    } finally {
      setLoadingDomains(false);
    }
  }, [cid]);

  useEffect(() => { fetchDomains(); }, [fetchDomains]);

  const fetchPolicy = useCallback(async () => {
    if (!hasLoadedRef.current) {
      setLoading(true);
    } else {
      setRefreshing(true);
    }
    try {
      const domainId = activeDomainId.trim();
      const policyPath = domainId
        ? `/api/admin/domains/${encodeURIComponent(domainId)}/security/spam-filter`
        : `/api/admin/companies/${cid}/security/spam-filter`;
      const domainQuery = domainId ? `&domain_id=${encodeURIComponent(domainId)}` : '';
      const statsPath = domainId
        ? `/api/admin/companies/${cid}/security/spam-filter/stats?domain_id=${encodeURIComponent(domainId)}`
        : `/api/admin/companies/${cid}/security/spam-filter/stats`;
      const [policyRes, eventsRes, statsRes] = await Promise.all([
        fetch(policyPath, { credentials: 'include' }),
        fetch(`/api/admin/companies/${cid}/security/spam-filter/events?limit=25${domainQuery}`, { credentials: 'include' }),
        fetch(statsPath, { credentials: 'include' }),
      ]);
      if (policyRes.ok) {
        const data = await policyRes.json();
        const p = data.policy ?? {};
        const nextPolicy = {
          ...defaultPolicy(),
          ...p,
          blocked_extensions: p.blocked_extensions ?? [],
          blocked_senders: p.blocked_senders ?? [],
          allowed_senders: p.allowed_senders ?? [],
          rbl_zones: p.rbl_zones ?? [],
          filter_packs: {
            enabled_pack_ids: p.filter_packs?.enabled_pack_ids ?? defaultPolicy().filter_packs.enabled_pack_ids,
            custom_packs: p.filter_packs?.custom_packs ?? [],
          },
        };
        setPolicy(nextPolicy);
        setSavedPolicyJson(JSON.stringify(nextPolicy));
      }
      if (eventsRes.ok) {
        const data = await eventsRes.json();
        setEvents(data.spam_filter_events ?? []);
      }
      if (statsRes.ok) {
        const data = await statsRes.json();
        setStats(data.spam_filter_stats ?? null);
      }
      setLastUpdated(new Date());
      hasLoadedRef.current = true;
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [cid, activeDomainId]);

  useEffect(() => { fetchPolicy(); }, [fetchPolicy]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const domainId = activeDomainId.trim();
      const policyPath = domainId
        ? `/api/admin/domains/${encodeURIComponent(domainId)}/security/spam-filter`
        : `/api/admin/companies/${cid}/security/spam-filter`;
      const res = await fetch(policyPath, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json().catch(() => null);
        const savedPolicy = data?.policy ? {
          ...defaultPolicy(),
          ...data.policy,
          blocked_extensions: data.policy.blocked_extensions ?? [],
          blocked_senders: data.policy.blocked_senders ?? [],
          allowed_senders: data.policy.allowed_senders ?? [],
          rbl_zones: data.policy.rbl_zones ?? [],
          filter_packs: {
            enabled_pack_ids: data.policy.filter_packs?.enabled_pack_ids ?? defaultPolicy().filter_packs.enabled_pack_ids,
            custom_packs: data.policy.filter_packs?.custom_packs ?? [],
          },
        } : policy;
        setPolicy(savedPolicy);
        setSavedPolicyJson(JSON.stringify(savedPolicy));
        setNotifications([{ type: 'success', content: t('pages.spam_filter_page.save_success'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-ok' }]);
      } else {
        setNotifications([{ type: 'error', content: t('pages.spam_filter_page.save_error'), dismissible: true, onDismiss: () => setNotifications([]), id: 'save-err' }]);
      }
    } finally {
      setSaving(false);
    }
  };

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

  const activeScopeLabel = activeDomainId
    ? t('pages.spam_filter_page.scope_domain')
    : t('pages.spam_filter_page.scope_company');
  const activePackCount = (policy.filter_packs?.enabled_pack_ids ?? []).length;
  const customPackCount = (policy.filter_packs?.custom_packs ?? []).length;
  const filteredRate = stats?.total_messages
    ? Math.round(((stats.filtered ?? 0) / stats.total_messages) * 100)
    : 0;
  const filteredEvents = useMemo(() => {
    const minScore = eventMinScore.trim() === '' ? null : Number(eventMinScore);
    const fromTime = eventFrom ? Date.parse(eventFrom) : null;
    const toTime = eventTo ? Date.parse(eventTo) : null;
    return events.filter(event => {
      const created = event.created_at ? Date.parse(event.created_at) : 0;
      if (fromTime && created && created < fromTime) return false;
      if (toTime && created && created > toTime) return false;
      if (minScore !== null && !Number.isNaN(minScore) && (event.spam_score ?? 0) < minScore) return false;
      if (eventFilter === 'all') return true;
      const status = `${event.flow_status ?? ''} ${event.enhanced_status ?? ''} ${event.error_message ?? ''}`.toLowerCase();
      if (eventFilter === 'rejected') return status.includes('reject') || status.includes('blocked');
      if (eventFilter === 'delivered') return status.includes('deliver') || status.includes('accept');
      return status.includes('filter') || status.includes('spam') || status.includes('quarantine');
    });
  }, [eventFilter, eventFrom, eventMinScore, eventTo, events]);
  const formatRulesCount = (count: number) => {
    if (locale === 'ko') return `${count}개 규칙`;
    if (locale === 'ja') return `${count}件のルール`;
    if (locale === 'zh-CN') return `${count}条规则`;
    return `${count} ${count === 1 ? 'rule' : 'rules'}`;
  };
  const selectedCustomPack = (policy.filter_packs?.custom_packs ?? []).find(pack => pack.id === selectedCustomPackId) ?? null;
  const isDirty = savedPolicyJson !== '' && JSON.stringify(policy) !== savedPolicyJson;
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

        {/* Master toggle */}
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

            <ColumnLayout columns={5} variant="text-grid" minColumnWidth={140}>
              <Container>
                <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_total')}</Box>
                <Box variant="h2">{(stats?.total_messages ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_filtered')}</Box>
                <Box variant="h2">{(stats?.filtered ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_rejected')}</Box>
                <Box variant="h2">{(stats?.rejected ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_delivered')}</Box>
                <Box variant="h2">{(stats?.delivered ?? 0).toLocaleString()}</Box>
              </Container>
              <Container>
                <Box variant="awsui-key-label">{t('pages.spam_filter_page.metric_filter_rate')}</Box>
                <Box variant="h2">{filteredRate}%</Box>
              </Container>
            </ColumnLayout>

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

            <Container
              header={
                <Header
                  variant="h2"
                  counter={`(${filteredEvents.length})`}
                  actions={
                    <SpaceBetween direction="horizontal" size="xs">
                      {lastUpdated && <Box color="text-body-secondary">{t('pages.spam_filter_page.last_updated')}: {lastUpdated.toLocaleTimeString()}</Box>}
                      <Button onClick={fetchPolicy} loading={refreshing}>{t('pages.spam_filter_page.refresh')}</Button>
                    </SpaceBetween>
                  }
                >
                  {t('pages.spam_filter_page.events_section')}
                </Header>
              }
            >
              <SpaceBetween size="m">
                <RadioGroup
                  value={eventFilter}
                  onChange={e => setEventFilter(e.detail.value as EventFilter)}
                  items={[
                    { value: 'all', label: t('pages.spam_filter_page.event_filter_all') },
                    { value: 'filtered', label: t('pages.spam_filter_page.event_filter_filtered') },
                    { value: 'rejected', label: t('pages.spam_filter_page.event_filter_rejected') },
                    { value: 'delivered', label: t('pages.spam_filter_page.event_filter_delivered') },
                  ]}
                />
                <ColumnLayout columns={3}>
                  <FormField label={t('pages.spam_filter_page.event_from_label')}>
                    <Input value={eventFrom} onChange={event => setEventFrom(event.detail.value)} placeholder="2026-05-17T00:00:00Z" />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.event_to_label')}>
                    <Input value={eventTo} onChange={event => setEventTo(event.detail.value)} placeholder="2026-05-17T23:59:59Z" />
                  </FormField>
                  <FormField label={t('pages.spam_filter_page.event_min_score_label')}>
                    <Input type="number" value={eventMinScore} onChange={event => setEventMinScore(event.detail.value)} placeholder="5" />
                  </FormField>
                </ColumnLayout>
                <DataTable
                  searchPlaceholder={t('pages.spam_filter_page.events_search')}
                  columnDefinitions={[
                    {
                      header: t('pages.spam_filter_page.col_time'),
                      cell: (item: SpamFilterEvent) => item.created_at ? new Date(item.created_at).toLocaleString() : '—',
                      width: '16%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_from'),
                      cell: (item: SpamFilterEvent) => item.from_addr || item.mail_from || '—',
                      width: '18%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_subject'),
                      cell: (item: SpamFilterEvent) => item.subject || '—',
                      width: '24%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_action'),
                      cell: (item: SpamFilterEvent) => item.enhanced_status || item.flow_status,
                      width: '10%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_score'),
                      cell: (item: SpamFilterEvent) => item.spam_score?.toFixed(1) ?? '—',
                      width: '8%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_reason'),
                      cell: (item: SpamFilterEvent) => item.error_message || '—',
                      width: '24%',
                    },
                    {
                      header: t('pages.spam_filter_page.col_manage'),
                      cell: (item: SpamFilterEvent) => (
                        <Button variant="inline-link" onClick={() => setDetailEvent(item)}>
                          {t('pages.spam_filter_page.view_details')}
                        </Button>
                      ),
                      width: '10%',
                    },
                  ]}
                  items={filteredEvents}
                  header={<Header variant="h3">{t('pages.spam_filter_page.events_table_title')}</Header>}
                />
              </SpaceBetween>
            </Container>
          </>
        )}

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
      </SpaceBetween>

      <Modal
        visible={detailEvent !== null}
        onDismiss={() => setDetailEvent(null)}
        header={t('pages.spam_filter_page.event_detail_title')}
        footer={<Button onClick={() => setDetailEvent(null)}>{t('common.close', 'Close')}</Button>}
      >
        {detailEvent && (
          <SpaceBetween size="s">
            <ColumnLayout columns={2}>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_time')}</Box>{detailEvent.created_at ? new Date(detailEvent.created_at).toLocaleString() : '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_score')}</Box>{detailEvent.spam_score?.toFixed(1) ?? '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_from')}</Box>{detailEvent.from_addr || detailEvent.mail_from || '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_action')}</Box>{detailEvent.enhanced_status || detailEvent.flow_status}</Box>
              <Box><Box variant="awsui-key-label">SPF</Box>{detailEvent.spf_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">DKIM</Box>{detailEvent.dkim_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">DMARC</Box>{detailEvent.dmarc_result || '—'}</Box>
              <Box><Box variant="awsui-key-label">{t('pages.spam_filter_page.col_subject')}</Box>{detailEvent.subject || '—'}</Box>
            </ColumnLayout>
            <FormField label={t('pages.spam_filter_page.col_reason')}>
              <Textarea value={detailEvent.error_message || '—'} readOnly rows={5} />
            </FormField>
          </SpaceBetween>
        )}
      </Modal>
    </ContentLayout>
  );
}
