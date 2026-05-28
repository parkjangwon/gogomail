'use client';
import { Wizard, SpaceBetween, Flashbar, FlashbarProps } from '@cloudscape-design/components';
import { useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import { useI18n } from '@/app/i18n-provider';
import { useOnboarding } from './components/useOnboarding';
import { OnboardingStep1 } from './components/OnboardingStep1';
import { OnboardingStep2 } from './components/OnboardingStep2';
import { OnboardingStep3 } from './components/OnboardingStep3';
import { OnboardingStep4 } from './components/OnboardingStep4';
import { OnboardingStep5 } from './components/OnboardingStep5';
import { OnboardingReview } from './components/OnboardingReview';

export default function OnboardingPage() {
  const { t } = useI18n();
  const router = useRouter();
  const params = useParams();
  const cid = params?.id as string;

  const [activeStep, setActiveStep] = useState(0);
  const [notifications, setNotifications] = useState<FlashbarProps.MessageDefinition[]>([]);

  const {
    step1, setStep1, step1Errors, createdCompany, step1Loading, submitStep1,
    step2, setStep2, step2Errors, createdDomain, step2Loading, submitStep2,
    dnsCheck, dnsChecking, dnsCheckError, handleCheckDns,
    step4, setStep4, step4Errors, step4Loading, dkimCreated, submitStep4,
    step5, setStep5, step5Errors, createdUserCount, submitStep5,
    isLoading,
  } = useOnboarding();

  const domainName = createdDomain?.name ?? (step2.domain_name.trim() || 'example.com');

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
        isLoadingNextStep={isLoading}
        steps={[
          {
            title: t('onboarding.step1_title'),
            isOptional: false,
            content: (
              <OnboardingStep1
                step1={step1}
                setStep1={setStep1}
                step1Errors={step1Errors}
                createdCompany={createdCompany}
                step1Loading={step1Loading}
              />
            ),
          },
          {
            title: t('onboarding.step2_title'),
            isOptional: false,
            content: (
              <OnboardingStep2
                step2={step2}
                setStep2={setStep2}
                step2Errors={step2Errors}
                createdDomain={createdDomain}
                step2Loading={step2Loading}
              />
            ),
          },
          {
            title: t('onboarding.step3_title'),
            isOptional: true,
            content: (
              <OnboardingStep3
                createdDomain={createdDomain}
                dnsCheck={dnsCheck}
                dnsChecking={dnsChecking}
                dnsCheckError={dnsCheckError}
                handleCheckDns={handleCheckDns}
                domainName={domainName}
                step4Selector={step4.selector}
                step4PublicKeyDns={step4.public_key_dns}
              />
            ),
          },
          {
            title: t('onboarding.step4_title'),
            isOptional: true,
            content: (
              <OnboardingStep4
                step4={step4}
                setStep4={setStep4}
                step4Errors={step4Errors}
                step4Loading={step4Loading}
                dkimCreated={dkimCreated}
              />
            ),
          },
          {
            title: t('onboarding.step5_title'),
            isOptional: true,
            content: (
              <OnboardingStep5
                step5={step5}
                setStep5={setStep5}
                step5Errors={step5Errors}
                createdUserCount={createdUserCount}
                domainName={domainName}
              />
            ),
          },
          {
            title: t('onboarding.step_review_title'),
            isOptional: false,
            content: (
              <OnboardingReview
                createdCompany={createdCompany}
                createdDomain={createdDomain}
                dnsCheck={dnsCheck}
                dkimCreated={dkimCreated}
                step4Selector={step4.selector}
                domainName={domainName}
                step5Skip={step5.skip}
                createdUserCount={createdUserCount}
              />
            ),
          },
        ]}
      />
    </SpaceBetween>
  );
}
