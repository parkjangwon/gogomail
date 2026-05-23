import type { SendMessageResult } from './api';

type TFn = (key: string, values?: Record<string, unknown>) => string;

export function sendStatusLabel(status?: string, t?: TFn): string {
  switch (status) {
    case 'sent':
      return t ? t('misc.sendResult.sent') : 'Send requested';
    case 'scheduled':
      return t ? t('misc.sendResult.scheduled') : 'Scheduled';
    case 'failed':
      return t ? t('misc.sendResult.failed') : 'Send failed';
    case 'queued':
    case undefined:
      return t ? t('misc.sendResult.queued') : 'Queued';
    default:
      return status;
  }
}

export function deliveryStatusLabel(status?: string, t?: TFn): string {
  switch (status) {
    case 'delivered':
      return t ? t('misc.sendResult.deliverDelivered') : 'Delivered';
    case 'deferred':
      return t ? t('misc.sendResult.deliverDeferred') : 'Retrying';
    case 'failed':
      return t ? t('misc.sendResult.deliverFailed') : 'Delivery failed';
    case 'pending':
    case undefined:
      return t ? t('misc.sendResult.deliverPending') : 'Delivery pending';
    default:
      return status;
  }
}

export function bounceStatusLabel(status?: string, t?: TFn): string {
  switch (status) {
    case 'bounced':
      return t ? t('misc.sendResult.bounced') : 'Bounced';
    case 'complained':
      return t ? t('misc.sendResult.complained') : 'Spam reported';
    case 'none':
    case '':
    case undefined:
      return '';
    default:
      return status;
  }
}

export function formatSendResultLabel(result: SendMessageResult | null, t?: TFn): string {
  if (!result) return '';
  const bounce = bounceStatusLabel(result.bounce_status, t);
  const sendLabel = sendStatusLabel(result.send_status, t);
  const deliverLabel = deliveryStatusLabel(result.delivery_status, t);
  if (t) {
    return [
      t('misc.sendResult.sendPrefix', { label: sendLabel }),
      t('misc.sendResult.deliverPrefix', { label: deliverLabel }),
      bounce && t('misc.sendResult.bouncePrefix', { label: bounce }),
    ].filter(Boolean).join(' · ');
  }
  return [
    `Send: ${sendLabel}`,
    `Delivery: ${deliverLabel}`,
    bounce && `Bounce: ${bounce}`,
  ].filter(Boolean).join(' · ');
}
