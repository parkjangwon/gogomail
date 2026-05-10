"use client";

import { useState } from "react";
import { useAuditPolicy, useUpdateAuditPolicy, type AuditLevel } from "@/hooks";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Box from "@cloudscape-design/components/box";
import FormField from "@cloudscape-design/components/form-field";
import RadioGroup from "@cloudscape-design/components/radio-group";
import Button from "@cloudscape-design/components/button";
import SpaceBetween from "@cloudscape-design/components/space-between";
import Checkbox from "@cloudscape-design/components/checkbox";
import Spinner from "@cloudscape-design/components/spinner";
import Alert from "@cloudscape-design/components/alert";

// TODO: Get company ID from context/auth
const DEMO_COMPANY_ID = "demo-company";

export default function AuditPolicyPage() {
  const { isLoading } = useAuditPolicy(DEMO_COMPANY_ID);
  const updateMutation = useUpdateAuditPolicy();
  const [level, setLevel] = useState<AuditLevel>("level_1");
  const [auditAdmin, setAuditAdmin] = useState(false);
  const [auditSecurity, setAuditSecurity] = useState(false);

  const handleSave = () => {
    updateMutation.mutate({
      company_id: DEMO_COMPANY_ID,
      audit_level: level,
      audit_admin_actions: auditAdmin,
      audit_security_events: auditSecurity,
    });
  };

  return (
    <Box padding="l">
      <Container
        header={<Header variant="h1">Audit Policy Configuration</Header>}
      >
        {isLoading ? (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        ) : (
          <SpaceBetween direction="vertical" size="l">
            {updateMutation.isSuccess && (
              <Alert type="success">Policy updated successfully</Alert>
            )}
            {updateMutation.isError && (
              <Alert type="error">Failed to update policy</Alert>
            )}

            <FormField label="Audit Level">
              <RadioGroup
                value={level}
                onChange={(e) => setLevel(e.detail.value as AuditLevel)}
                items={[
                  {
                    value: "level_1",
                    label: "Level 1 - Basic (Compliance minimum)",
                  },
                  {
                    value: "level_2",
                    label: "Level 2 - Standard (Admin actions)",
                  },
                  {
                    value: "level_3",
                    label: "Level 3 - Enhanced (User mail operations)",
                  },
                ]}
              />
            </FormField>

            <FormField label="Audit Scope">
              <SpaceBetween direction="vertical" size="s">
                <Checkbox
                  checked={auditAdmin}
                  onChange={(e) => setAuditAdmin(e.detail.checked)}
                >
                  Audit Admin Actions
                </Checkbox>
                <Checkbox
                  checked={auditSecurity}
                  onChange={(e) => setAuditSecurity(e.detail.checked)}
                >
                  Audit Security Events
                </Checkbox>
              </SpaceBetween>
            </FormField>

            <Box>
              <Button
                variant="primary"
                loading={updateMutation.isPending}
                onClick={handleSave}
              >
                Save Policy
              </Button>
            </Box>
          </SpaceBetween>
        )}
      </Container>
    </Box>
  );
}
