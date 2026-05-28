'use client';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Textarea,
  Toggle,
  Alert,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';
import type { Step4Data } from './types';

interface Props {
  step4: Step4Data;
  setStep4: (data: Step4Data) => void;
  step4Errors: Partial<Step4Data>;
  step4Loading: boolean;
  dkimCreated: boolean;
}

export function OnboardingStep4({ step4, setStep4, step4Errors, step4Loading, dkimCreated }: Props) {
  const { t } = useI18n();

  return (
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
                placeholder={'-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----'}
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
}
