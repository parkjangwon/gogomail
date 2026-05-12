import { composeCloseSaveButtonLabel } from './composeCloseSaveButtonLabel';

const idleLabel: string = composeCloseSaveButtonLabel(false);
const savingLabel: string = composeCloseSaveButtonLabel(true);

export const composeCloseSaveButtonLabelContract = {
  idleLabel,
  savingLabel,
} as const;
