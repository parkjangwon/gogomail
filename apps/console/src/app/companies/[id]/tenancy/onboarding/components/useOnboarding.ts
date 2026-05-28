'use client';
import { useState, useCallback } from 'react';
import { useI18n } from '@/app/i18n-provider';
import type {
  CreatedCompany,
  CreatedDomain,
  DnsCheckResult,
  Step1Data,
  Step2Data,
  Step4Data,
  Step5Data,
} from './types';

export function useOnboarding() {
  const { t } = useI18n();

  // Step 1 state
  const [step1, setStep1] = useState<Step1Data>({ name: '', quota_gb: '10', status: 'active' });
  const [step1Errors, setStep1Errors] = useState<Partial<Step1Data>>({});
  const [createdCompany, setCreatedCompany] = useState<CreatedCompany | null>(null);
  const [step1Loading, setStep1Loading] = useState(false);

  // Step 2 state
  const [step2, setStep2] = useState<Step2Data>({ domain_name: '', display_name: '' });
  const [step2Errors, setStep2Errors] = useState<Partial<Step2Data>>({});
  const [createdDomain, setCreatedDomain] = useState<CreatedDomain | null>(null);
  const [step2Loading, setStep2Loading] = useState(false);

  // Step 3 state
  const [dnsCheck, setDnsCheck] = useState<DnsCheckResult>({ mx: false, spf: false, dkim: false, checked: false });
  const [dnsChecking, setDnsChecking] = useState(false);
  const [dnsCheckError, setDnsCheckError] = useState('');

  // Step 4 state
  const [step4, setStep4] = useState<Step4Data>({ selector: 'default', private_key_pem: '', public_key_dns: '', skip: false });
  const [step4Errors, setStep4Errors] = useState<Partial<Step4Data>>({});
  const [step4Loading, setStep4Loading] = useState(false);
  const [dkimCreated, setDkimCreated] = useState(false);

  // Step 5 state
  const [step5, setStep5] = useState<Step5Data>({ email: '', display_name: '', password: '', skip: false });
  const [step5Errors, setStep5Errors] = useState<Partial<Step5Data>>({});
  const [createdUserCount, setCreatedUserCount] = useState(0);
  const [step5Loading, setStep5Loading] = useState(false);

  // ── Validation ───────────────────────────────────────────────────────────────

  const validateStep1 = (): boolean => {
    const errors: Partial<Step1Data> = {};
    if (!step1.name.trim() || step1.name.trim().length < 2) {
      errors.name = t('onboarding.error_name_required');
    }
    const gb = parseFloat(step1.quota_gb);
    if (isNaN(gb) || gb <= 0) {
      errors.quota_gb = t('onboarding.error_quota_invalid');
    }
    setStep1Errors(errors);
    return Object.keys(errors).length === 0;
  };

  const validateStep2 = (): boolean => {
    const errors: Partial<Step2Data> = {};
    const domain = step2.domain_name.trim();
    if (!domain) {
      errors.domain_name = t('onboarding.error_domain_required');
    } else if (!domain.includes('.') || domain.includes(' ') || domain.length > 253) {
      errors.domain_name = t('onboarding.error_domain_invalid');
    }
    setStep2Errors(errors);
    return Object.keys(errors).length === 0;
  };

  const validateStep4 = (): boolean => {
    if (step4.skip) return true;
    const errors: Partial<Step4Data> = {};
    if (!step4.selector.trim()) errors.selector = t('onboarding.error_selector_required');
    if (!step4.private_key_pem.trim()) errors.private_key_pem = t('onboarding.error_private_key_required');
    if (!step4.public_key_dns.trim()) errors.public_key_dns = t('onboarding.error_public_key_required');
    setStep4Errors(errors);
    return Object.keys(errors).length === 0;
  };

  const validateStep5 = (): boolean => {
    if (step5.skip) return true;
    const errors: Partial<Step5Data> = {};
    const email = step5.email.trim();
    if (!email) {
      errors.email = t('onboarding.error_email_required');
    } else if (!email.includes('@')) {
      errors.email = t('onboarding.error_email_invalid');
    }
    if (!step5.password) errors.password = t('onboarding.error_password_required');
    setStep5Errors(errors);
    return Object.keys(errors).length === 0;
  };

  // ── API Calls ────────────────────────────────────────────────────────────────

  const submitStep1 = useCallback(async (): Promise<boolean> => {
    if (createdCompany) return true;
    if (!validateStep1()) return false;
    setStep1Loading(true);
    try {
      const quotaBytes = Math.round(parseFloat(step1.quota_gb) * 1073741824);
      const res = await fetch('/api/admin/companies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name: step1.name.trim(), quota_limit: quotaBytes, status: step1.status }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        setStep1Errors({ name: err?.error?.message || t('onboarding.error_create_company') });
        return false;
      }
      const data = await res.json();
      setCreatedCompany(data.company);
      return true;
    } catch {
      setStep1Errors({ name: t('onboarding.error_create_company') });
      return false;
    } finally {
      setStep1Loading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step1, createdCompany]);

  const submitStep2 = useCallback(async (): Promise<boolean> => {
    if (createdDomain) return true;
    if (!validateStep2()) return false;
    if (!createdCompany) return false;
    setStep2Loading(true);
    try {
      const body: Record<string, string> = {
        company_id: createdCompany.id,
        name: step2.domain_name.trim(),
      };
      if (step2.display_name.trim()) body.display_name = step2.display_name.trim();
      const res = await fetch('/api/admin/domains', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        setStep2Errors({ domain_name: err?.error?.message || t('onboarding.error_create_domain') });
        return false;
      }
      const data = await res.json();
      setCreatedDomain(data.domain);
      return true;
    } catch {
      setStep2Errors({ domain_name: t('onboarding.error_create_domain') });
      return false;
    } finally {
      setStep2Loading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step2, createdDomain, createdCompany]);

  const submitStep4 = useCallback(async (): Promise<boolean> => {
    if (step4.skip) return true;
    if (dkimCreated) return true;
    if (!validateStep4()) return false;
    if (!createdDomain) return false;
    setStep4Loading(true);
    try {
      const res = await fetch('/api/admin/dkim-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          domain_id: createdDomain.id,
          selector: step4.selector.trim(),
          private_key_pem: step4.private_key_pem.trim(),
          public_key_dns: step4.public_key_dns.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        setStep4Errors({ selector: err?.error?.message || t('onboarding.error_create_dkim') });
        return false;
      }
      setDkimCreated(true);
      return true;
    } catch {
      setStep4Errors({ selector: t('onboarding.error_create_dkim') });
      return false;
    } finally {
      setStep4Loading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step4, dkimCreated, createdDomain]);

  const submitStep5 = useCallback(async (): Promise<boolean> => {
    if (step5.skip) return true;
    if (createdUserCount > 0) return true;
    if (!validateStep5()) return false;
    if (!createdDomain) return false;
    setStep5Loading(true);
    try {
      const res = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          email: step5.email.trim(),
          display_name: step5.display_name.trim() || step5.email.trim(),
          password: step5.password,
          domain_id: createdDomain.id,
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        setStep5Errors({ email: err?.error?.message || t('onboarding.error_create_user') });
        return false;
      }
      setCreatedUserCount(1);
      return true;
    } catch {
      setStep5Errors({ email: t('onboarding.error_create_user') });
      return false;
    } finally {
      setStep5Loading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step5, createdDomain, createdUserCount]);

  const handleCheckDns = async () => {
    if (!createdDomain) return;
    setDnsChecking(true);
    setDnsCheckError('');
    try {
      const res = await fetch(`/api/admin/domains/${createdDomain.id}/dns-check`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        const checks = data.checks || data.results || {};
        setDnsCheck({
          mx: checks.mx === 'ok' || checks.mx === true,
          spf: checks.spf === 'ok' || checks.spf === true,
          dkim: checks.dkim === 'ok' || checks.dkim === true,
          checked: true,
        });
      } else {
        setDnsCheckError(t('common.error'));
      }
    } catch {
      setDnsCheckError(t('common.error'));
    } finally {
      setDnsChecking(false);
    }
  };

  const isLoading = step1Loading || step2Loading || step4Loading || step5Loading;

  return {
    // Step 1
    step1, setStep1, step1Errors, createdCompany, step1Loading, submitStep1,
    // Step 2
    step2, setStep2, step2Errors, createdDomain, step2Loading, submitStep2,
    // Step 3
    dnsCheck, dnsChecking, dnsCheckError, handleCheckDns,
    // Step 4
    step4, setStep4, step4Errors, step4Loading, dkimCreated, submitStep4,
    // Step 5
    step5, setStep5, step5Errors, step5Loading, createdUserCount, submitStep5,
    // Shared
    isLoading,
  };
}
