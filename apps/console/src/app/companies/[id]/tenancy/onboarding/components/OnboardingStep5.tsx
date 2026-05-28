'use client';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Toggle,
  Alert,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { Step5Data } from './types';

interface Props {
  step5: Step5Data;
  setStep5: (data: Step5Data) => void;
  step5Errors: Partial<Step5Data>;
  createdUserCount: number;
  domainName: string;
}

export function OnboardingStep5({ step5, setStep5, step5Errors, createdUserCount, domainName }: Props) {
  const { t } = useI18n();

  return (
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
}
