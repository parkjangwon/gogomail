import type { SendMessageResult } from './api';

export function sendStatusLabel(status?: string): string {
  switch (status) {
    case 'sent':
      return '발송 요청 완료';
    case 'scheduled':
      return '예약 등록';
    case 'failed':
      return '발송 실패';
    case 'queued':
    case undefined:
      return '대기열 등록';
    default:
      return status;
  }
}

export function deliveryStatusLabel(status?: string): string {
  switch (status) {
    case 'delivered':
      return '배송 완료';
    case 'deferred':
      return '재시도 중';
    case 'failed':
      return '배송 실패';
    case 'pending':
    case undefined:
      return '배송 대기';
    default:
      return status;
  }
}

export function bounceStatusLabel(status?: string): string {
  switch (status) {
    case 'bounced':
      return '반송됨';
    case 'complained':
      return '스팸 신고';
    case 'none':
    case '':
    case undefined:
      return '';
    default:
      return status;
  }
}

export function formatSendResultLabel(result: SendMessageResult | null): string {
  if (!result) return '';
  const bounce = bounceStatusLabel(result.bounce_status);
  return [
    `전송: ${sendStatusLabel(result.send_status)}`,
    `배송: ${deliveryStatusLabel(result.delivery_status)}`,
    bounce && `반송: ${bounce}`,
  ].filter(Boolean).join(' · ');
}
