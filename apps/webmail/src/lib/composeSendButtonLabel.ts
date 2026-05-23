export interface ComposeSendButtonLabelState {
  sending: boolean;
  sent: boolean;
  scheduled: boolean;
  uploading: boolean;
}

export function composeSendButtonLabel(
  state: ComposeSendButtonLabelState,
  t: (key: string) => string,
): string {
  if (state.sending) return t('misc.compose.sendSending');
  if (state.sent) return state.scheduled ? t('misc.compose.sendScheduled') : t('misc.compose.sendDone');
  if (state.uploading) return t('misc.compose.sendUploading');
  if (state.scheduled) return t('misc.compose.sendScheduledLabel');
  return t('misc.compose.send');
}
