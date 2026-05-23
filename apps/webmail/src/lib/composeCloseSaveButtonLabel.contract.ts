import { composeCloseSaveButtonLabel } from './composeCloseSaveButtonLabel';

const t = (k: string) => k;
const idleLabel: string = composeCloseSaveButtonLabel(false, t);
const savingLabel: string = composeCloseSaveButtonLabel(true, t);

export const composeCloseSaveButtonLabelContract = {
  idleLabel,
  savingLabel,
} as const;
