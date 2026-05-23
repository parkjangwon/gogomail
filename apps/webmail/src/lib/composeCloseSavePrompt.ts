export function composeCloseSavePrompt(
  scheduled: boolean,
  t: (key: string) => string,
): string {
  return scheduled ? t('misc.compose.savePromptScheduled') : t('misc.compose.savePrompt');
}
