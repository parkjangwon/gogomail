// Re-export everything from all domain modules.
// Consumers importing from '@/lib/api' continue to work via the api.ts shim.

export * from './types';
export * from './http';
export * from './auth';
export * from './mail';
export * from './dm';
export * from './drive';
export * from './calendar';
export * from './contacts';

// Backward-compat alias: SendMessageRequest was renamed to MailSendRequest.
export type { MailSendRequest as SendMessageRequest } from './mail';
