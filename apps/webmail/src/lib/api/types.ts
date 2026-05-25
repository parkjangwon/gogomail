// Shared types and interfaces used across multiple domain modules.

export interface Folder {
  id: string;
  parent_id?: string;
  name: string;
  full_path: string;
  type: string;
  system_type?: string;
  order_index: number;
  total: number;
  unread: number;
  starred: number;
}

export interface MessageAddress {
  email: string;
  address?: string;
  name?: string;
}

export interface MessageSummary {
  id: string;
  folder_id: string;
  subject: string;
  preview: string;
  from_addr: string;
  from_name: string;
  sender_avatar_url?: string;
  received_at: string;
  size: number;
  has_attachment: boolean;
  read: boolean;
  starred: boolean;
  search_rank?: number;
  search_highlights?: {
    subject?: string[];
    from?: string[];
    body?: string[];
  };
  // Thread view optional fields
  thread_id?: string;
  message_count?: number;
  unread_count?: number;
}

export interface Attachment {
  id: string;
  message_id: string;
  upload_id: string;
  storage_path: string;
  filename: string;
  size: number;
  mime_type: string;
  status: 'uploading' | 'stored' | 'deleted';
  created_at: string;
}

export interface MessageDetail extends MessageSummary {
  message_id: string;
  to_addrs: MessageAddress[];
  cc_addrs: MessageAddress[];
  bcc_addrs: MessageAddress[];
  flags: Record<string, unknown>;
  storage_path: string;
  text_body: string;
  html_body?: string;
  attachments?: Attachment[];
}

export interface AuthTokenResponse {
  expires_at: string;
  must_change_password: boolean;
  client_ip?: string;
  mfa_required?: boolean;
  pending_token?: string;
  mfa_setup_required?: boolean;
}

export interface MFAVerifyResponse {
  expires_at: string;
}

export type ComposeIntent = 'new' | 'reply' | 'forward';
export type UIComposeIntent = ComposeIntent | 'reply_all';

export interface ThreadSummary {
  id: string;
  folder_id: string;
  subject: string;
  preview: string;
  message_count: number;
  unread_count: number;
  latest_message_id: string;
  latest_from_addr: string;
  latest_at: string;
  has_attachment: boolean;
  starred: boolean;
}
