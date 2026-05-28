'use client';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  RadioGroup,
  Alert,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { Step1Data, CreatedCompany } from './types';

interface Props {
  step1: Step1Data;
  setStep1: (data: Step1Data) => void;
  step1Errors: Partial<Step1Data>;
  createdCompany: CreatedCompany | null;
  step1Loading: boolean;
}

export function OnboardingStep1({ step1, setStep1, step1Errors, createdCompany, step1Loading }: Props) {
  const { t } = useI18n();

  return (
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
}
