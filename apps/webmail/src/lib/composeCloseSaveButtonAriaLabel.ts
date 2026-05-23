export function composeCloseSaveButtonAriaLabel(
  inProgress: boolean,
  t: (key: string) => string,
): string {
  return inProgress ? t('misc.compose.closeSaveAriaInProgress') : t('misc.compose.closeSaveAria');
}
