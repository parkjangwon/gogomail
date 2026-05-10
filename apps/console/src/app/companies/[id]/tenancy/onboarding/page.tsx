'use client';

import {
  Wizard,
  FormField,
  Input,
  RadioGroup,
  Alert,
  Container,
  Header,
  SpaceBetween,
  Table,
  Button,
  StatusIndicator,
  Textarea,
  ColumnLayout,
  Box,
  Flashbar,
  FlashbarProps,
  Toggle,
} from '@cloudscape-design/components';
import { CopyToClipboard } from '@cloudscape-design/components';
import { useState, useCallback } from 'react';
import { useRouter, useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';

// ── Types ────────────────────────────────────────────────────────────────────

interface CreatedCompany {
  id: string;
  name: string;
  quota_limit: number;
  status: string;
}

interface CreatedDomain {
  id: string;
  name: string;
  display_name?: string;
}

interface DnsCheckResult {
  mx: boolean;
  spf: boolean;
  dkim: boolean;
  checked: boolean;
}

interface Step1Data {
  name: string;
  quota_gb: string;
  status: string;
}

interface Step2Data {
  domain_name: string;
  display_name: string;
}

interface Step4Data {
  selector: string;
  private_key_pem: string;
  public_key_dns: string;
  skip: boolean;
}

interface Step5Data {
  email: string;
  display_name: string;
  password: string;
  skip: boolean;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  const gb = bytes / 1073741824;
  return `${gb.toFixed(0)} GB`;
}

// ── Main Component ───────────────────────────────────────────────────────────

export default function OnboardingPage() {
  const { t } = useI18n();
  const router = useRouter();
  const params = useParams();
  const cid = params?.id as string;

  const [activeStep, setActiveStep] = useState(0);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

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

  // Step 4 state
  const [step4, setStep4] = useState<Step4Data>({ selector: 'default', private_key_pem: '', public_key_dns: '', skip: false });
  const [step4Errors, setStep4Errors] = useState<Partial<Step4Data>>({});
  const [step4Loading, setStep4Loading] = useState(false);
  const [dkimCreated, setDkimCreated] = useState(false);

  // Step 5 state
  const [step5, setStep5] = useState<Step5Data>({ email: '', display_name: '', password: '', skip: false });
  const [step5Errors, setStep5Errors] = useState<Partial<Step5Data>>({});
  const [createdUserCount, setCreatedUserCount] = useState(0);

  // ── Validation ─────────────────────────────────────────────────────────────

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

  // ── API Calls ──────────────────────────────────────────────────────────────

  const submitStep1 = useCallback(async (): Promise<boolean> => {
    if (createdCompany) return true; // already submitted
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
    if (!validateStep5()) return false;
    if (!createdDomain) return false;
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
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step5, createdDomain]);

  const handleCheckDns = async () => {
    if (!createdDomain) return;
    setDnsChecking(true);
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
      }
    } catch {
      // ignore
    } finally {
      setDnsChecking(false);
    }
  };

  // ── Navigation ─────────────────────────────────────────────────────────────

  const handleNavigate = async (detail: { requestedStepIndex: number; reason: string }) => {
    const { requestedStepIndex, reason } = detail;

    if (reason === 'next') {
      let ok = false;
      if (activeStep === 0) ok = await submitStep1();
      else if (activeStep === 1) ok = await submitStep2();
      else if (activeStep === 2) ok = true; // DNS check is optional
      else if (activeStep === 3) ok = await submitStep4();
      else if (activeStep === 4) ok = await submitStep5();
      else ok = true;
      if (!ok) return;
    }

    if (reason === 'previous') {
      setActiveStep(requestedStepIndex);
      return;
    }

    setActiveStep(requestedStepIndex);
  };

  const handleComplete = () => {
    setNotifications([
      {
        type: 'success',
        content: t('onboarding.success_message').replace('{name}', createdCompany?.name ?? ''),
        dismissible: true,
        onDismiss: () => setNotifications([]),
        id: 'onboarding-complete',
      },
    ]);
    if (createdCompany) {
      router.push(`/companies/${createdCompany.id}/tenancy/companies`);
    } else {
      router.push(`/companies/${cid}/tenancy/companies`);
    }
  };

  // ── DNS Records helper ─────────────────────────────────────────────────────

  const domainName = createdDomain?.name ?? step2.domain_name.trim() ?? 'example.com';
  const dnsRecords = [
    {
      type: 'MX',
      host: domainName,
      value: `10 mail.${domainName}`,
      checked: dnsCheck.checked ? (dnsCheck.mx ? 'ok' : 'fail') : 'pending',
    },
    {
      type: 'TXT (SPF)',
      host: domainName,
      value: `v=spf1 include:_spf.${domainName} ~all`,
      checked: dnsCheck.checked ? (dnsCheck.spf ? 'ok' : 'fail') : 'pending',
    },
    {
      type: 'TXT (DKIM)',
      host: `${step4.selector || 'default'}._domainkey.${domainName}`,
      value: step4.public_key_dns || '(configure DKIM in Step 4)',
      checked: dnsCheck.checked ? (dnsCheck.dkim ? 'ok' : 'fail') : 'pending',
    },
  ];

  // ── Step Content ───────────────────────────────────────────────────────────

  const step1Content = (
    <Container header={<Header variant="h2">{t('onboarding.step1_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step1_info')}</Alert>
        <FormField
          label={t('onboarding.company_name')}
          constraintText={t('onboarding.company_name_constraint')}
          errorText={step1Errors.name}
        >
          <Input
            value={step1.name}
            onChange={({ detail }) => setStep1({ ...step1, name: detail.value })}
            placeholder={t('onboarding.company_name_placeholder')}
            disabled={!!createdCompany || step1Loading}
          />
        </FormField>
        <FormField
          label={t('onboarding.quota_gb')}
          constraintText={t('onboarding.quota_gb_constraint')}
          errorText={step1Errors.quota_gb}
        >
          <Input
            type="number"
            value={step1.quota_gb}
            onChange={({ detail }) => setStep1({ ...step1, quota_gb: detail.value })}
            disabled={!!createdCompany || step1Loading}
          />
        </FormField>
        <FormField label={t('onboarding.status')}>
          <RadioGroup
            value={step1.status}
            onChange={({ detail }) => setStep1({ ...step1, status: detail.value })}
            items={[
              { value: 'active', label: t('onboarding.status_active') },
              { value: 'suspended', label: t('onboarding.status_suspended') },
            ]}
          />
        </FormField>
        {createdCompany && (
          <Alert type="success">
            {t('onboarding.company_created').replace('{id}', createdCompany.id)}
          </Alert>
        )}
      </SpaceBetween>
    </Container>
  );

  const step2Content = (
    <Container header={<Header variant="h2">{t('onboarding.step2_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step2_info')}</Alert>
        <FormField
          label={t('onboarding.domain_name')}
          constraintText={t('onboarding.domain_name_constraint')}
          errorText={step2Errors.domain_name}
        >
          <Input
            value={step2.domain_name}
            onChange={({ detail }) => setStep2({ ...step2, domain_name: detail.value })}
            placeholder="example.com"
            disabled={!!createdDomain || step2Loading}
          />
        </FormField>
        <FormField
          label={t('onboarding.display_name')}
          constraintText={t('onboarding.display_name_constraint')}
        >
          <Input
            value={step2.display_name}
            onChange={({ detail }) => setStep2({ ...step2, display_name: detail.value })}
            placeholder={t('onboarding.display_name_placeholder')}
            disabled={!!createdDomain || step2Loading}
          />
        </FormField>
        {createdDomain && (
          <Alert type="success">
            {t('onboarding.domain_created').replace('{id}', createdDomain.id)}
          </Alert>
        )}
      </SpaceBetween>
    </Container>
  );

  const step3Content = (
    <Container header={<Header variant="h2">{t('onboarding.step3_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step3_info')}</Alert>
        <Table
          header={<Header variant="h3">{t('onboarding.dns_records')}</Header>}
          columnDefinitions={[
            { id: 'type', header: t('onboarding.dns_type'), cell: (item) => item.type },
            { id: 'host', header: t('onboarding.dns_host'), cell: (item) => (
              <CopyToClipboard
                copyButtonAriaLabel={t('onboarding.copy')}
                copyErrorText={t('onboarding.copy_error')}
                copySuccessText={t('onboarding.copy_success')}
                textToCopy={item.host}
                variant="inline"
              />
            )},
            { id: 'value', header: t('onboarding.dns_value'), cell: (item) => (
              <CopyToClipboard
                copyButtonAriaLabel={t('onboarding.copy')}
                copyErrorText={t('onboarding.copy_error')}
                copySuccessText={t('onboarding.copy_success')}
                textToCopy={item.value}
                variant="inline"
              />
            )},
            { id: 'status', header: t('onboarding.dns_status'), cell: (item) => {
              if (item.checked === 'pending') return <StatusIndicator type="pending">{t('onboarding.dns_pending')}</StatusIndicator>;
              if (item.checked === 'ok') return <StatusIndicator type="success">{t('onboarding.dns_verified')}</StatusIndicator>;
              return <StatusIndicator type="error">{t('onboarding.dns_not_found')}</StatusIndicator>;
            }},
          ]}
          items={dnsRecords}
          variant="container"
        />
        <Button
          onClick={handleCheckDns}
          loading={dnsChecking}
          disabled={!createdDomain}
        >
          {t('onboarding.check_dns')}
        </Button>
      </SpaceBetween>
    </Container>
  );

  const step4Content = (
    <Container header={<Header variant="h2">{t('onboarding.step4_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step4_info')}</Alert>
        <Toggle
          checked={step4.skip}
          onChange={({ detail }) => setStep4({ ...step4, skip: detail.checked })}
        >
          {t('onboarding.skip_dkim')}
        </Toggle>
        {!step4.skip && (
          <SpaceBetween size="m">
            <FormField
              label={t('onboarding.dkim_selector')}
              constraintText={t('onboarding.dkim_selector_constraint')}
              errorText={step4Errors.selector}
            >
              <Input
                value={step4.selector}
                onChange={({ detail }) => setStep4({ ...step4, selector: detail.value })}
                disabled={dkimCreated || step4Loading}
              />
            </FormField>
            <FormField
              label={t('onboarding.private_key_pem')}
              constraintText={t('onboarding.private_key_pem_constraint')}
              errorText={step4Errors.private_key_pem}
            >
              <Textarea
                value={step4.private_key_pem}
                onChange={({ detail }) => setStep4({ ...step4, private_key_pem: detail.value })}
                rows={6}
                placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                disabled={dkimCreated || step4Loading}
              />
            </FormField>
            <FormField
              label={t('onboarding.public_key_dns')}
              constraintText={t('onboarding.public_key_dns_constraint')}
              errorText={step4Errors.public_key_dns}
            >
              <Textarea
                value={step4.public_key_dns}
                onChange={({ detail }) => setStep4({ ...step4, public_key_dns: detail.value })}
                rows={4}
                placeholder="v=DKIM1; k=rsa; p=..."
                disabled={dkimCreated || step4Loading}
              />
            </FormField>
            {dkimCreated && (
              <Alert type="success">{t('onboarding.dkim_created')}</Alert>
            )}
          </SpaceBetween>
        )}
      </SpaceBetween>
    </Container>
  );

  const step5Content = (
    <Container header={<Header variant="h2">{t('onboarding.step5_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.step5_info')}</Alert>
        <Toggle
          checked={step5.skip}
          onChange={({ detail }) => setStep5({ ...step5, skip: detail.checked })}
        >
          {t('onboarding.skip_user')}
        </Toggle>
        {!step5.skip && (
          <SpaceBetween size="m">
            <FormField
              label={t('onboarding.user_email')}
              constraintText={t('onboarding.user_email_constraint')}
              errorText={step5Errors.email}
            >
              <Input
                type="email"
                value={step5.email}
                onChange={({ detail }) => setStep5({ ...step5, email: detail.value })}
                placeholder={`admin@${domainName}`}
                disabled={createdUserCount > 0}
              />
            </FormField>
            <FormField label={t('onboarding.user_display_name')}>
              <Input
                value={step5.display_name}
                onChange={({ detail }) => setStep5({ ...step5, display_name: detail.value })}
                disabled={createdUserCount > 0}
              />
            </FormField>
            <FormField
              label={t('onboarding.user_password')}
              constraintText={t('onboarding.user_password_constraint')}
              errorText={step5Errors.password}
            >
              <Input
                type="password"
                value={step5.password}
                onChange={({ detail }) => setStep5({ ...step5, password: detail.value })}
                disabled={createdUserCount > 0}
              />
            </FormField>
            {createdUserCount > 0 && (
              <Alert type="success">{t('onboarding.user_created')}</Alert>
            )}
          </SpaceBetween>
        )}
      </SpaceBetween>
    </Container>
  );

  const reviewContent = (
    <Container header={<Header variant="h2">{t('onboarding.step_review_title')}</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">{t('onboarding.review_info')}</Alert>
        <ColumnLayout columns={2} variant="text-grid">
          <SpaceBetween size="s">
            <Box variant="awsui-key-label">{t('onboarding.review_company')}</Box>
            <div>{createdCompany?.name ?? '—'}</div>
            <Box variant="awsui-key-label">ID</Box>
            <div>{createdCompany?.id ?? '—'}</div>
            <Box variant="awsui-key-label">{t('onboarding.quota_gb')}</Box>
            <div>{createdCompany ? formatBytes(createdCompany.quota_limit) : '—'}</div>
            <Box variant="awsui-key-label">{t('onboarding.status')}</Box>
            <div>{createdCompany?.status ?? '—'}</div>
          </SpaceBetween>
          <SpaceBetween size="s">
            <Box variant="awsui-key-label">{t('onboarding.review_domain')}</Box>
            <div>{createdDomain?.name ?? '—'}</div>
            <Box variant="awsui-key-label">ID</Box>
            <div>{createdDomain?.id ?? '—'}</div>
            <Box variant="awsui-key-label">{t('onboarding.dns_status')}</Box>
            <div>
              {dnsCheck.checked
                ? (dnsCheck.mx && dnsCheck.spf
                  ? <StatusIndicator type="success">{t('onboarding.dns_verified')}</StatusIndicator>
                  : <StatusIndicator type="warning">{t('onboarding.dns_partial')}</StatusIndicator>)
                : <StatusIndicator type="pending">{t('onboarding.dns_pending')}</StatusIndicator>
              }
            </div>
            <Box variant="awsui-key-label">{t('onboarding.review_dkim')}</Box>
            <div>{dkimCreated ? `${step4.selector}._domainkey.${domainName}` : t('onboarding.not_configured')}</div>
            <Box variant="awsui-key-label">{t('onboarding.review_users')}</Box>
            <div>{step5.skip ? t('onboarding.not_configured') : `${createdUserCount}`}</div>
          </SpaceBetween>
        </ColumnLayout>
      </SpaceBetween>
    </Container>
  );

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <SpaceBetween size="m">
      <Flashbar items={notifications} />
      <Wizard
        i18nStrings={{
          stepNumberLabel: (stepNumber) => `${t('onboarding.step')} ${stepNumber}`,
          collapsedStepsLabel: (stepNumber, stepsCount) =>
            `${t('onboarding.step')} ${stepNumber} ${t('onboarding.of')} ${stepsCount}`,
          skipToButtonLabel: (step) => `${t('onboarding.skip_to')} ${step.title}`,
          navigationAriaLabel: t('onboarding.navigation'),
          cancelButton: t('common.cancel'),
          previousButton: t('common.previous'),
          nextButton: t('common.next'),
          submitButton: t('onboarding.complete_setup'),
          optional: t('onboarding.optional'),
        }}
        onNavigate={({ detail }) => handleNavigate(detail)}
        onCancel={() => router.push(`/companies/${cid}/tenancy/companies`)}
        onSubmit={handleComplete}
        activeStepIndex={activeStep}
        steps={[
          {
            title: t('onboarding.step1_title'),
            content: step1Content,
            isOptional: false,
          },
          {
            title: t('onboarding.step2_title'),
            content: step2Content,
            isOptional: false,
          },
          {
            title: t('onboarding.step3_title'),
            content: step3Content,
            isOptional: true,
          },
          {
            title: t('onboarding.step4_title'),
            content: step4Content,
            isOptional: true,
          },
          {
            title: t('onboarding.step5_title'),
            content: step5Content,
            isOptional: true,
          },
          {
            title: t('onboarding.step_review_title'),
            content: reviewContent,
            isOptional: false,
          },
        ]}
      />
    </SpaceBetween>
  );
}
