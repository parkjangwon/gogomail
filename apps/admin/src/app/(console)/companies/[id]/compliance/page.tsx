'use client';

import {
  Container,
  Header,
  SpaceBetween,
  Box,
  Alert,
  Checkbox,
  Button,
} from '@cloudscape-design/components';
import { useState } from 'react';

export default function CompliancePage() {
  const [compliance, setCompliance] = useState({
    gdpr: false,
    hipaa: false,
    pci_dss: false,
    sox: false,
    ccpa: false,
  });

  const handleSave = () => {
    console.log('Compliance settings saved:', compliance);
  };

  return (
    <Container header={<Header>Policy & Compliance</Header>}>
      <SpaceBetween size="l">
        <Alert type="info">
          Select applicable compliance frameworks for your organization
        </Alert>

        <Box>
          <Header variant="h3">Regulatory Compliance</Header>
          <SpaceBetween size="m">
            <Checkbox
              checked={compliance.gdpr}
              onChange={(e) =>
                setCompliance({ ...compliance, gdpr: e.detail.checked })
              }
            >
              GDPR (General Data Protection Regulation)
            </Checkbox>
            <Checkbox
              checked={compliance.hipaa}
              onChange={(e) =>
                setCompliance({ ...compliance, hipaa: e.detail.checked })
              }
            >
              HIPAA (Health Insurance Portability and Accountability Act)
            </Checkbox>
            <Checkbox
              checked={compliance.pci_dss}
              onChange={(e) =>
                setCompliance({ ...compliance, pci_dss: e.detail.checked })
              }
            >
              PCI DSS (Payment Card Industry Data Security Standard)
            </Checkbox>
            <Checkbox
              checked={compliance.sox}
              onChange={(e) =>
                setCompliance({ ...compliance, sox: e.detail.checked })
              }
            >
              SOX (Sarbanes-Oxley Act)
            </Checkbox>
            <Checkbox
              checked={compliance.ccpa}
              onChange={(e) =>
                setCompliance({ ...compliance, ccpa: e.detail.checked })
              }
            >
              CCPA (California Consumer Privacy Act)
            </Checkbox>
          </SpaceBetween>
        </Box>

        <Box>
          <Header variant="h3">Data Protection</Header>
          <SpaceBetween size="m">
            <Alert type="success">
              All data is encrypted at rest and in transit using industry-standard protocols
            </Alert>
            <Alert type="info">
              Regular security audits and penetration testing are performed
            </Alert>
          </SpaceBetween>
        </Box>

        <Box>
          <Header variant="h3">Audit & Logging</Header>
          <Alert type="success">
            Comprehensive audit logs are maintained for all administrative actions
          </Alert>
        </Box>

        <Box float="right">
          <Button variant="primary" onClick={handleSave}>
            Save Compliance Settings
          </Button>
        </Box>
      </SpaceBetween>
    </Container>
  );
}
