export function composeCloseSaveButtonAriaLabel(inProgress: boolean): string {
  return inProgress ? '임시저장 중입니다' : '임시저장 후 작성창 닫기';
}
