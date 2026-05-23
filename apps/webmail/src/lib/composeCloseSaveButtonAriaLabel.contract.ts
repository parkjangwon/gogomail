import { composeCloseSaveButtonAriaLabel } from './composeCloseSaveButtonAriaLabel';

const t = (k: string) => k;
const idleAriaLabel: string = composeCloseSaveButtonAriaLabel(false, t);
const savingAriaLabel: string = composeCloseSaveButtonAriaLabel(true, t);

export const composeCloseSaveButtonAriaLabelContract = {
  idleAriaLabel,
  savingAriaLabel,
} as const;
