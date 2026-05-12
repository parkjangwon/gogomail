import { composeCloseSavePrompt } from './composeCloseSavePrompt';

const scheduledPrompt: string = composeCloseSavePrompt(true);
const unscheduledPrompt: string = composeCloseSavePrompt(false);

export const composeCloseSavePromptContract = {
  scheduledPrompt,
  unscheduledPrompt,
} as const;
