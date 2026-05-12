import { composeSendButtonLabel, type ComposeSendButtonLabelState } from './composeSendButtonLabel';

const baseState = {
  sending: false,
  sent: false,
  scheduled: false,
  uploading: false,
} satisfies ComposeSendButtonLabelState;

const sendingLabel: string = composeSendButtonLabel({ ...baseState, sending: true });
const immediateSentLabel: string = composeSendButtonLabel({ ...baseState, sent: true });
const scheduledSentLabel: string = composeSendButtonLabel({ ...baseState, sent: true, scheduled: true });
const uploadingLabel: string = composeSendButtonLabel({ ...baseState, uploading: true });
const scheduledReadyLabel: string = composeSendButtonLabel({ ...baseState, scheduled: true });
const defaultLabel: string = composeSendButtonLabel(baseState);

export const composeSendButtonLabelContract = {
  sendingLabel,
  immediateSentLabel,
  scheduledSentLabel,
  uploadingLabel,
  scheduledReadyLabel,
  defaultLabel,
} as const;
