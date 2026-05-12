export function composeCloseSaveButtonLabel(inProgress: boolean): string {
  return inProgress ? '저장 중...' : '임시저장';
}
