// Shared types and helpers for the onboarding wizard

export interface CreatedCompany {
  id: string;
  name: string;
  quota_limit: number;
  status: string;
}

export interface CreatedDomain {
  id: string;
  name: string;
  display_name?: string;
}

export interface DnsCheckResult {
  mx: boolean;
  spf: boolean;
  dkim: boolean;
  checked: boolean;
}

export interface Step1Data {
  name: string;
  quota_gb: string;
  status: string;
}

export interface Step2Data {
  domain_name: string;
  display_name: string;
}

export interface Step4Data {
  selector: string;
  private_key_pem: string;
  public_key_dns: string;
  skip: boolean;
}

export interface Step5Data {
  email: string;
  display_name: string;
  password: string;
  skip: boolean;
}

export function formatBytes(bytes: number, unlimitedLabel = 'Unlimited'): string {
  if (bytes === 0) return unlimitedLabel;
  const gb = bytes / 1073741824;
  return `${gb.toFixed(0)} GB`;
}
