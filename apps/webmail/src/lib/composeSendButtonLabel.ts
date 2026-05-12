export interface ComposeSendButtonLabelState {
  sending: boolean;
  sent: boolean;
  scheduled: boolean;
  uploading: boolean;
}

export function composeSendButtonLabel(state: ComposeSendButtonLabelState): string {
  if (state.sending) return '전송 중...';
  if (state.sent) return state.scheduled ? '예약됨 ✓' : '전송됨 ✓';
  if (state.uploading) return '업로드 중...';
  if (state.scheduled) return '예약 전송';
  return '전송';
}
