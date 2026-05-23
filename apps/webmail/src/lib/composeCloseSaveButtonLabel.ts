export function composeCloseSaveButtonLabel(
  inProgress: boolean,
  t: (key: string) => string,
): string {
  return inProgress ? t('misc.compose.closeSaveSaving') : t('misc.compose.closeSaveLabel');
}
