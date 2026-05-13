import { formatSendResultLabel } from './sendResultLabel';
import type { SendMessageResult } from './api';

const baseResult = {
  id: 'msg-local-1',
  message_id: '<msg-local-1@gogomail.test>',
  farm: 'general',
  send_status: 'queued',
  delivery_status: 'pending',
  bounce_status: 'none',
} satisfies SendMessageResult;

const queuedPendingLabel: string = formatSendResultLabel(baseResult);
const bouncedLabel: string = formatSendResultLabel({
  ...baseResult,
  send_status: 'sent',
  delivery_status: 'failed',
  bounce_status: 'bounced',
});
const unknownStatusLabel: string = formatSendResultLabel({
  ...baseResult,
  send_status: 'provider_pending',
  delivery_status: 'provider_pending',
  bounce_status: 'provider_review',
});
const emptyLabel: string = formatSendResultLabel(null);

export const sendResultLabelContract = {
  queuedPendingLabel,
  bouncedLabel,
  unknownStatusLabel,
  emptyLabel,
} as const;
