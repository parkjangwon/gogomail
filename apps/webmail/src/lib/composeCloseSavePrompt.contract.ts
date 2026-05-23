import { composeCloseSavePrompt } from './composeCloseSavePrompt';

const t = (k: string) => k;
const scheduledPrompt: string = composeCloseSavePrompt(true, t);
const unscheduledPrompt: string = composeCloseSavePrompt(false, t);

export const composeCloseSavePromptContract = {
  scheduledPrompt,
  unscheduledPrompt,
} as const;
