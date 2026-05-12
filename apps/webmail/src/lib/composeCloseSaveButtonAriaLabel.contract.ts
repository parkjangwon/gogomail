import { composeCloseSaveButtonAriaLabel } from './composeCloseSaveButtonAriaLabel';

const idleAriaLabel: string = composeCloseSaveButtonAriaLabel(false);
const savingAriaLabel: string = composeCloseSaveButtonAriaLabel(true);

export const composeCloseSaveButtonAriaLabelContract = {
  idleAriaLabel,
  savingAriaLabel,
} as const;
