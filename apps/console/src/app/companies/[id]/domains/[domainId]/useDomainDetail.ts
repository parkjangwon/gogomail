'use client';
import { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { buildMailFlowLogsQuery } from '@/lib/mailFlowLogs';
import {
  DomainDetail,
  User,
  DomainSetting,
  DailyCount,
  DomainMCPPolicy,
  DomainMCPPolicyConfig,
  DEFAULT_MCP_POLICY,
  DEFAULT_MCP_SCOPES,
  normalizeMCPPolicy,
} from './domainDetailTypes';

export function useDomainDetail() {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const companyId = params?.id as string;
  const domainId = params?.domainId as string;

  const [domain, setDomain] = useState<DomainDetail | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [settings, setSettings] = useState<DomainSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [verifying, setVerifying] = useState(false);

  // Add Setting modal
  const [showAddSetting, setShowAddSetting] = useState(false);
  const [newSetting, setNewSetting] = useState({ key: '', value: '' });
  const [savingSetting, setSavingSetting] = useState(false);

  // Edit modal
  const [showEdit, setShowEdit] = useState(false);
  const [editForm, setEditForm] = useState({ quota_gb: '', status: 'active' });
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  // Delete modal
  const [showDelete, setShowDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Mail stats
  const [mailStats, setMailStats] = useState<DailyCount[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsFetched, setStatsFetched] = useState(false);

  // MCP policy
  const [mcpPolicy, setMcpPolicy] = useState<DomainMCPPolicy>(DEFAULT_MCP_POLICY);
  const [mcpPolicyConfig, setMcpPolicyConfig] = useState<DomainMCPPolicyConfig | null>(null);
  const [mcpPolicyLoading, setMcpPolicyLoading] = useState(false);
  const [mcpPolicySaving, setMcpPolicySaving] = useState(false);
  const [mcpPolicyError, setMcpPolicyError] = useState('');
  const [mcpPolicySaved, setMcpPolicySaved] = useState(false);

  useEffect(() => {
    Promise.all([
      fetch(`/api/admin/domains/${domainId}`, { credentials: 'include' }).then(r => r.ok ? r.json() : null),
      fetch(`/api/admin/users?domain_id=${domainId}&limit=100`, { credentials: 'include' }).then(r => r.ok ? r.json() : { users: [] }),
      fetch(`/api/admin/domains/${domainId}/config`, { credentials: 'include' }).then(r => r.ok ? r.json() : { config: [] }),
      fetch(`/api/admin/domains/${domainId}/mcp-policy`, { credentials: 'include' }).then(r => r.ok ? r.json() : null).catch(() => null),
    ]).then(([domainData, usersData, settingsData, mcpPolicyData]) => {
      if (domainData?.domain) {
        setDomain(domainData.domain);
        setEditForm({
          quota_gb: domainData.domain.quota_limit > 0 ? String(Math.round(domainData.domain.quota_limit / 1073741824)) : '',
          status: domainData.domain.status,
        });
      }
      setUsers(usersData.users || []);
      setSettings(settingsData.config || []);
      setMcpPolicy(normalizeMCPPolicy(mcpPolicyData?.mcp_policy));
      setMcpPolicyConfig(mcpPolicyData?.config ?? null);
      if (!mcpPolicyData) {
        setMcpPolicyError(t('pages.domain_detail.mcp_policy_load_error', 'Failed to load MCP policy. Defaults are shown until refreshed.'));
      }
    }).catch(() => {
      setLoadError('Failed to load domain details. Please refresh the page.');
    }).finally(() => setLoading(false));
  }, [domainId]);

  const handleVerifyDNS = async () => {
    setVerifying(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/dns-check`, {
        method: 'POST',
        credentials: 'include',
      });
      if (res.ok) {
        const data = await res.json();
        setDomain(prev => prev ? { ...prev, last_dns_check_status: data.dns_check?.status ?? prev.last_dns_check_status } : prev);
      }
    } finally {
      setVerifying(false);
    }
  };

  const handleSaveEdit = async () => {
    setSaving(true);
    setSaveError('');
    try {
      const quotaBytes = editForm.quota_gb ? parseInt(editForm.quota_gb, 10) * 1073741824 : 0;
      const statusChanged = domain?.status !== editForm.status;

      const calls: Promise<Response>[] = [
        fetch(`/api/admin/domains/${domainId}/quota`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ quota_limit: isNaN(quotaBytes) ? 0 : quotaBytes }),
          credentials: 'include',
        }),
      ];
      if (statusChanged) {
        calls.push(fetch(`/api/admin/domains/${domainId}/status`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: editForm.status }),
          credentials: 'include',
        }));
      }

      const results = await Promise.all(calls);
      const failed = results.find(r => !r.ok);
      if (failed) {
        const errData = await failed.json().catch(() => ({})) as { error?: { message?: string } };
        setSaveError(errData.error?.message ?? '저장 실패');
        return;
      }
      const refreshed = await fetch(`/api/admin/domains/${domainId}`, { credentials: 'include' });
      if (refreshed.ok) {
        const d = await refreshed.json();
        setDomain(d.domain);
      }
      setShowEdit(false);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    setDeleteError('');
    try {
      const res = await fetch(`/api/admin/domains/${domainId}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        router.push(`/companies/${companyId}/tenancy/domains`);
      } else {
        const data = await res.json().catch(() => ({})) as { error?: { message?: string } };
        setDeleteError(data.error?.message ?? '삭제 실패');
      }
    } finally {
      setDeleting(false);
    }
  };

  const handleAddSetting = async () => {
    if (!newSetting.key.trim()) return;
    setSavingSetting(true);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/config/${encodeURIComponent(newSetting.key.trim())}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: newSetting.value }),
        credentials: 'include',
      });
      if (res.ok) {
        setShowAddSetting(false);
        setNewSetting({ key: '', value: '' });
        const r = await fetch(`/api/admin/domains/${domainId}/config`, { credentials: 'include' });
        if (r.ok) { const d = await r.json(); setSettings(d.config || []); }
      }
    } finally {
      setSavingSetting(false);
    }
  };

  const refreshMCPPolicy = async () => {
    setMcpPolicyLoading(true);
    setMcpPolicyError('');
    setMcpPolicySaved(false);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/mcp-policy`, { credentials: 'include' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({})) as { error?: { message?: string } };
        setMcpPolicyError(data.error?.message ?? t('pages.domain_detail.mcp_policy_load_error', 'Failed to load MCP policy.'));
        return;
      }
      const data = await res.json();
      setMcpPolicy(normalizeMCPPolicy(data.mcp_policy));
      setMcpPolicyConfig(data.config ?? null);
    } finally {
      setMcpPolicyLoading(false);
    }
  };

  const handleSaveMCPPolicy = async () => {
    setMcpPolicySaving(true);
    setMcpPolicyError('');
    setMcpPolicySaved(false);
    try {
      const res = await fetch(`/api/admin/domains/${domainId}/mcp-policy`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          value: mcpPolicy,
          locked: mcpPolicyConfig?.Locked ?? false,
          ...(mcpPolicyConfig?.Version ? { version: mcpPolicyConfig.Version } : {}),
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({})) as { error?: { message?: string } };
        setMcpPolicyError(data.error?.message ?? t('pages.domain_detail.mcp_policy_save_error', 'Failed to save MCP policy.'));
        return;
      }
      const data = await res.json();
      setMcpPolicy(normalizeMCPPolicy(data.mcp_policy));
      setMcpPolicyConfig(data.config ?? null);
      setMcpPolicySaved(true);
    } finally {
      setMcpPolicySaving(false);
    }
  };

  const setMCPPolicyScope = (scope: string, checked: boolean) => {
    setMcpPolicy((current) => {
      const scopes = new Set(current.allowed_scopes);
      if (checked) scopes.add(scope);
      else scopes.delete(scope);
      return { ...current, allowed_scopes: DEFAULT_MCP_SCOPES.filter((item) => scopes.has(item)) };
    });
    setMcpPolicySaved(false);
  };

  const updateMCPPolicy = (patch: Partial<DomainMCPPolicy>) => {
    setMcpPolicy((current) => ({ ...current, ...patch }));
    setMcpPolicySaved(false);
  };

  const fetchMailStats = async (_domainName: string, force = false) => {
    if (statsFetched && !force) return;
    setStatsLoading(true);
    try {
      const since = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString();
      const qs = buildMailFlowLogsQuery({
        companyId,
        domainId,
        since,
        limit: 500,
      });
      const res = await fetch(`/api/admin/mail-flow-logs${qs ? `?${qs}` : ''}`, { credentials: 'include' });
      if (!res.ok) return;
      const data = await res.json();
      const logs: Array<{ created_at: string; status: string }> = data.mail_flow_logs ?? [];

      const countMap = new Map<string, { total: number; success: number; failed: number }>();
      for (let i = 6; i >= 0; i--) {
        const d = new Date(Date.now() - i * 24 * 60 * 60 * 1000);
        const key = d.toISOString().slice(0, 10);
        countMap.set(key, { total: 0, success: 0, failed: 0 });
      }
      for (const log of logs) {
        const key = log.created_at?.slice(0, 10);
        if (key && countMap.has(key)) {
          const entry = countMap.get(key)!;
          entry.total++;
          if (log.status === 'delivered' || log.status === 'sent') entry.success++;
          else entry.failed++;
        }
      }

      const days: DailyCount[] = Array.from(countMap.entries()).map(([date, counts]) => ({
        date,
        label: new Date(date + 'T12:00:00').toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        ...counts,
      }));
      setMailStats(days);
      setStatsFetched(true);
    } finally {
      setStatsLoading(false);
    }
  };

  return {
    companyId,
    domainId,
    domain,
    users,
    settings,
    loading,
    loadError,
    verifying,
    handleVerifyDNS,
    showEdit,
    setShowEdit,
    editForm,
    setEditForm,
    saving,
    saveError,
    setSaveError,
    handleSaveEdit,
    showDelete,
    setShowDelete,
    deleting,
    deleteError,
    setDeleteError,
    handleDelete,
    showAddSetting,
    setShowAddSetting,
    newSetting,
    setNewSetting,
    savingSetting,
    handleAddSetting,
    mailStats,
    statsLoading,
    statsFetched,
    fetchMailStats,
    mcpPolicy,
    setMcpPolicy,
    mcpPolicyConfig,
    mcpPolicyLoading,
    mcpPolicySaving,
    mcpPolicyError,
    setMcpPolicyError,
    mcpPolicySaved,
    setMcpPolicySaved,
    handleSaveMCPPolicy,
    refreshMCPPolicy,
    setMCPPolicyScope,
    updateMCPPolicy,
  };
}
