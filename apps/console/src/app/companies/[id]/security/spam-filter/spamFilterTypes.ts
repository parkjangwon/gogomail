export interface SpamFilterPolicy {
  enabled: boolean;
  spam_threshold: number;
  virus_scan_enabled: boolean;
  strict_auth_enabled: boolean;
  rbl_check_enabled: boolean;
  rbl_reject_enabled: boolean;
  rbl_zones: string[];
  blocked_extensions: string[];
  blocked_senders: string[];
  allowed_senders: string[];
  quarantine_enabled: boolean;
  max_attachment_mb: number;
  bulk_recipient_limit: number;
  filter_packs: FilterPackBundle;
}

export interface FilterPackBundle {
  enabled_pack_ids: string[];
  custom_packs: FilterPack[];
}

export interface FilterPack {
  id: string;
  version: string;
  name: string;
  description: string;
  category: string;
  source: 'system' | 'custom' | string;
  enabled: boolean;
  rules: FilterRule[];
}

export interface FilterRule {
  id: string;
  type: 'phrase' | 'attachment_extension' | 'bulk_recipient' | 'auth_failure' | 'sender_domain' | 'url_host' | 'header_anomaly' | string;
  target?: 'subject' | 'body' | 'subject_body' | string;
  patterns: string[];
  score: number;
  enabled: boolean;
  action?: 'quarantine' | 'reject' | string;
}

export interface SpamFilterEvent {
  id: string;
  created_at: string;
  from_addr?: string;
  mail_from?: string;
  rcpt_to?: string;
  subject?: string;
  flow_status: string;
  enhanced_status?: string;
  error_message?: string;
  spam_score?: number;
  spf_result?: string;
  dkim_result?: string;
  dmarc_result?: string;
}

export interface SpamFilterStats {
  total_messages: number;
  filtered: number;
  rejected: number;
  delivered: number;
}

export type EventFilter = 'all' | 'filtered' | 'rejected' | 'delivered';

export interface DomainOption {
  value: string;
  label: string;
}

export const COMPANY_SCOPE_VALUE = '__company__';

export const defaultPolicy = (): SpamFilterPolicy => ({
  enabled: true,
  spam_threshold: 5,
  virus_scan_enabled: true,
  strict_auth_enabled: true,
  rbl_check_enabled: false,
  rbl_reject_enabled: true,
  rbl_zones: [],
  blocked_extensions: ['.exe', '.bat', '.cmd', '.scr', '.vbs', '.js', '.ps1', '.jar', '.docm', '.xlsm'],
  blocked_senders: [],
  allowed_senders: [],
  quarantine_enabled: true,
  max_attachment_mb: 25,
  bulk_recipient_limit: 50,
  filter_packs: {
    enabled_pack_ids: ['gogomail-core-auth', 'gogomail-core-malware', 'gogomail-core-phishing-ko', 'gogomail-core-bulk', 'gogomail-core-url', 'gogomail-core-sender'],
    custom_packs: [],
  },
});

export const builtinFilterPacks: Array<FilterPack & { nameKey: string; descriptionKey: string; categoryKey: string }> = [
  {
    id: 'gogomail-core-auth',
    nameKey: 'pack_auth_name',
    descriptionKey: 'pack_auth_desc',
    categoryKey: 'pack_category_authentication',
    version: '2026.05.17',
    name: 'Core authentication defense',
    description: 'Scores suspicious SPF, DKIM, and DMARC failure combinations.',
    category: 'authentication',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'no-auth-pass', type: 'auth_failure', patterns: ['no_auth_pass'], score: 1.5, enabled: true },
      { id: 'dmarc-fail', type: 'auth_failure', patterns: ['dmarc_fail'], score: 1.5, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-malware',
    nameKey: 'pack_malware_name',
    descriptionKey: 'pack_malware_desc',
    categoryKey: 'pack_category_malware',
    version: '2026.05.17',
    name: 'Core malware attachment defense',
    description: 'Scores high-risk executable and macro attachment extensions.',
    category: 'malware',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'dangerous-extension', type: 'attachment_extension', patterns: ['.exe', '.scr', '.js', '.vbs', '.ps1', '.jar', '.docm', '.xlsm'], score: 2, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-phishing-ko',
    nameKey: 'pack_phishing_name',
    descriptionKey: 'pack_phishing_desc',
    categoryKey: 'pack_category_phishing',
    version: '2026.05.17',
    name: 'Korean and global phishing phrases',
    description: 'Scores common credential theft, urgency, and payment-lure phrases.',
    category: 'phishing',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'credential-lures', type: 'phrase', target: 'subject_body', patterns: ['verify your account', 'password expired', 'login immediately', '계정 확인', '비밀번호 만료', '긴급 로그인'], score: 1.5, enabled: true },
      { id: 'payment-lures', type: 'phrase', target: 'subject_body', patterns: ['wire transfer', 'gift card', 'crypto giveaway', '송금', '상품권', '당첨'], score: 1, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-bulk',
    nameKey: 'pack_bulk_name',
    descriptionKey: 'pack_bulk_desc',
    categoryKey: 'pack_category_bulk',
    version: '2026.05.17',
    name: 'Bulk receive pressure defense',
    description: 'Scores messages above the tenant bulk recipient threshold.',
    category: 'bulk',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'recipient-fanout', type: 'bulk_recipient', patterns: [], score: 1.5, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-url',
    nameKey: 'pack_url_name',
    descriptionKey: 'pack_url_desc',
    categoryKey: 'pack_category_phishing',
    version: '2026.05.25',
    name: 'Core URL and credential phishing defense',
    description: 'Scores disguised links, credential forms, raw IP links, and IDN/punycode link lures.',
    category: 'phishing',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'link-text-mismatch', type: 'header_anomaly', patterns: ['url_mismatch'], score: 3, enabled: true },
      { id: 'credential-form', type: 'header_anomaly', patterns: ['html_form'], score: 3, enabled: true },
      { id: 'raw-ip-url', type: 'header_anomaly', patterns: ['raw_ip_url'], score: 2, enabled: true },
      { id: 'punycode-url', type: 'header_anomaly', patterns: ['punycode_url'], score: 2, enabled: true },
    ],
  },
  {
    id: 'gogomail-core-sender',
    nameKey: 'pack_sender_name',
    descriptionKey: 'pack_sender_desc',
    categoryKey: 'pack_category_impersonation',
    version: '2026.05.25',
    name: 'Core sender impersonation defense',
    description: 'Scores envelope/header sender mismatches and obfuscated credential-lure text.',
    category: 'impersonation',
    source: 'system',
    enabled: true,
    rules: [
      { id: 'from-envelope-mismatch', type: 'header_anomaly', patterns: ['from_envelope_mismatch'], score: 2, enabled: true },
      { id: 'text-obfuscation', type: 'header_anomaly', patterns: ['text_obfuscation'], score: 2, enabled: true },
    ],
  },
];
