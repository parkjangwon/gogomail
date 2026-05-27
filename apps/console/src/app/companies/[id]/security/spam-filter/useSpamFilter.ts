'use client';

import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { FlashbarProps, SelectProps } from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import { useCompany } from '@/contexts/CompanyContext';
import {
  SpamFilterPolicy,
  SpamFilterEvent,
  SpamFilterStats,
  DomainOption,
  COMPANY_SCOPE_VALUE,
  defaultPolicy,
} from './spamFilterTypes';

export function useSpamFilter() {
  const { t } = useI18n();
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
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

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

  const isDirty = savedPolicyJson !== '' && JSON.stringify(policy) !== savedPolicyJson;

  return {
    policy,
    setPolicy,
    savedPolicyJson,
    loading,
    refreshing,
    saving,
    notifications,
    events,
    stats,
    domains,
    loadingDomains,
    selectedScope,
    setSelectedScope,
    lastUpdated,
    fetchDomains,
    fetchPolicy,
    handleSave,
    scopeOptions,
    activeDomainId,
    isDirty,
    companyScopeOption,
    t,
  };
}
