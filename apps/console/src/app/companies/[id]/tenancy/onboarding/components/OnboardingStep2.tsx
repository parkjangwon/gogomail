'use client';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Alert,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { Step2Data, CreatedDomain } from './types';

interface Props {
  step2: Step2Data;
  setStep2: (data: Step2Data) => void;
  step2Errors: Partial<Step2Data>;
  createdDomain: CreatedDomain | null;
  step2Loading: boolean;
}

export function OnboardingStep2({ step2, setStep2, step2Errors, createdDomain, step2Loading }: Props) {
  const { t } = useI18n();

  return (
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
}
