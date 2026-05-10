'use client';

import { useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import {
  Container,
  Header,
  SpaceBetween,
  FormField,
  Input,
  Select,
  Checkbox,
  Button,
  Box,
  Spinner,
  Alert,
} from '@cloudscape-design/components';
import { useSecurityPolicy, useUpdateSecurityPolicy } from '@/hooks/useSecurityPolicy';

export default function SecurityPolicyPage() {
  const params = useParams();
  const companyId = params.id as string;

  const policyQuery = useSecurityPolicy(companyId);
  const updateMutation = useUpdateSecurityPolicy();

  const [formData, setFormData] = useState({
    mfa_mode: 'optional',
    mfa_grace_period_days: 7,
    session_timeout_minutes: 60,
    password_min_length: 12,
    password_require_uppercase: true,
    password_require_numbers: true,
    password_require_special: true,
    password_expiration_days: 90,
    ip_restriction_enabled: false,
    login_failure_lockout_attempts: 5,
    login_failure_lockout_duration_minutes: 15,
  });

  useEffect(() => {
    if (policyQuery.data) {
      setFormData({
        mfa_mode: policyQuery.data.mfa_mode,
        mfa_grace_period_days: policyQuery.data.mfa_grace_period_days,
        session_timeout_minutes: policyQuery.data.session_timeout_minutes,
        password_min_length: policyQuery.data.password_min_length,
        password_require_uppercase: policyQuery.data.password_require_uppercase,
        password_require_numbers: policyQuery.data.password_require_numbers,
        password_require_special: policyQuery.data.password_require_special,
        password_expiration_days: policyQuery.data.password_expiration_days || 90,
        ip_restriction_enabled: policyQuery.data.ip_restriction_enabled,
        login_failure_lockout_attempts: policyQuery.data.login_failure_lockout_attempts,
        login_failure_lockout_duration_minutes: policyQuery.data.login_failure_lockout_duration_minutes,
      });
    }
  }, [policyQuery.data]);

  const handleSave = async () => {
    await updateMutation.mutateAsync({
      companyId,
      policy: formData as any,
    });
  };

  if (policyQuery.isPending) {
    return <Spinner />;
  }

  return (
    <Container header={<Header>Security Policy</Header>}>
      <SpaceBetween size="l">
        {/* MFA Settings */}
        <Box>
          <Header variant="h3">Multi-Factor Authentication (MFA)</Header>
          <SpaceBetween size="m">
            <FormField label="MFA Mode">
              <Select
                selectedOption={{
                  label: formData.mfa_mode.toUpperCase(),
                  value: formData.mfa_mode,
                }}
                options={[
                  { label: 'Disabled', value: 'disabled' },
                  { label: 'Optional', value: 'optional' },
                  { label: 'Required', value: 'required' },
                ]}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    mfa_mode: (e.detail.selectedOption.value || 'optional') as any,
                  })
                }
              />
            </FormField>

            <FormField label="MFA Grace Period (days)">
              <Input
                type="number"
                value={String(formData.mfa_grace_period_days)}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    mfa_grace_period_days: parseInt(e.detail.value) || 7,
                  })
                }
              />
            </FormField>
          </SpaceBetween>
        </Box>

        {/* Session Settings */}
        <Box>
          <Header variant="h3">Session Settings</Header>
          <FormField label="Session Timeout (minutes)">
            <Input
              type="number"
              value={String(formData.session_timeout_minutes)}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  session_timeout_minutes: parseInt(e.detail.value) || 60,
                })
              }
            />
          </FormField>
        </Box>

        {/* Password Policy */}
        <Box>
          <Header variant="h3">Password Policy</Header>
          <SpaceBetween size="m">
            <FormField label="Minimum Length">
              <Input
                type="number"
                value={String(formData.password_min_length)}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    password_min_length: parseInt(e.detail.value) || 12,
                  })
                }
              />
            </FormField>

            <Checkbox
              checked={formData.password_require_uppercase}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  password_require_uppercase: e.detail.checked,
                })
              }
            >
              Require Uppercase Letters
            </Checkbox>

            <Checkbox
              checked={formData.password_require_numbers}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  password_require_numbers: e.detail.checked,
                })
              }
            >
              Require Numbers
            </Checkbox>

            <Checkbox
              checked={formData.password_require_special}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  password_require_special: e.detail.checked,
                })
              }
            >
              Require Special Characters
            </Checkbox>

            <FormField label="Password Expiration (days)">
              <Input
                type="number"
                value={String(formData.password_expiration_days)}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    password_expiration_days: parseInt(e.detail.value) || 90,
                  })
                }
              />
            </FormField>
          </SpaceBetween>
        </Box>

        {/* Account Lockout */}
        <Box>
          <Header variant="h3">Account Lockout Policy</Header>
          <SpaceBetween size="m">
            <FormField label="Failed Login Attempts (before lockout)">
              <Input
                type="number"
                value={String(formData.login_failure_lockout_attempts)}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    login_failure_lockout_attempts: parseInt(e.detail.value) || 5,
                  })
                }
              />
            </FormField>

            <FormField label="Lockout Duration (minutes)">
              <Input
                type="number"
                value={String(formData.login_failure_lockout_duration_minutes)}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    login_failure_lockout_duration_minutes: parseInt(e.detail.value) || 15,
                  })
                }
              />
            </FormField>
          </SpaceBetween>
        </Box>

        {/* IP Restrictions */}
        <Box>
          <Header variant="h3">IP Restrictions</Header>
          <Checkbox
            checked={formData.ip_restriction_enabled}
            onChange={(e) =>
              setFormData({
                ...formData,
                ip_restriction_enabled: e.detail.checked,
              })
            }
          >
            Enable IP Restriction
          </Checkbox>
        </Box>

        {/* Save Button */}
        <Box float="right">
          <Button
            variant="primary"
            onClick={handleSave}
            loading={updateMutation.isPending}
          >
            Save Security Policy
          </Button>
        </Box>

        {updateMutation.isSuccess && (
          <Alert type="success">Security policy updated successfully</Alert>
        )}
      </SpaceBetween>
    </Container>
  );
}
