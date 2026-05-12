export function composeCloseSavePrompt(scheduled: boolean): string {
  return scheduled
    ? '예약 설정을 포함해 임시저장 후 닫으시겠습니까?'
    : '임시저장 후 닫으시겠습니까?';
}
