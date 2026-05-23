import { composeSendButtonLabel, type ComposeSendButtonLabelState } from './composeSendButtonLabel';

const baseState = {
  sending: false,
  sent: false,
  scheduled: false,
  uploading: false,
} satisfies ComposeSendButtonLabelState;

const t = (k: string) => k;

const sendingLabel: string = composeSendButtonLabel({ ...baseState, sending: true }, t);
const immediateSentLabel: string = composeSendButtonLabel({ ...baseState, sent: true }, t);
const scheduledSentLabel: string = composeSendButtonLabel({ ...baseState, sent: true, scheduled: true }, t);
const uploadingLabel: string = composeSendButtonLabel({ ...baseState, uploading: true }, t);
const scheduledReadyLabel: string = composeSendButtonLabel({ ...baseState, scheduled: true }, t);
const defaultLabel: string = composeSendButtonLabel(baseState, t);

export const composeSendButtonLabelContract = {
  sendingLabel,
  immediateSentLabel,
  scheduledSentLabel,
  uploadingLabel,
  scheduledReadyLabel,
  defaultLabel,
} as const;
