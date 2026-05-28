// Korean QWERTY → Latin normalization so shortcuts work in Korean IME mode
export const KO_KEYS: Record<string, string> = {
  'ㄷ':'e','ㄱ':'r','ㅅ':'t','ㅛ':'y','ㅕ':'u','ㅑ':'i','ㅐ':'o','ㅔ':'p',
  'ㅁ':'a','ㄴ':'s','ㅇ':'d','ㄹ':'f','ㅎ':'g','ㅗ':'h','ㅓ':'j','ㅏ':'k','ㅣ':'l',
  'ㅋ':'z','ㅌ':'x','ㅊ':'c','ㅍ':'v','ㅠ':'b','ㅜ':'n','ㅡ':'m',
  'ㅂ':'q','ㅈ':'w',
};

export type DateGroupKey = 'today' | 'yesterday' | 'lastWeek' | 'thisMonth' | 'older';

const isoDate = (d: Date) => d.toLocaleDateString('en-CA'); // YYYY-MM-DD

export function getDateGroup(receivedAt: string): DateGroupKey {
  const date = new Date(receivedAt);
  const now = new Date();
  const today = isoDate(now);
  const yesterday = isoDate(new Date(now.getTime() - 86_400_000));

  const dateStr = isoDate(date);
  if (dateStr === today) return 'today';
  if (dateStr === yesterday) return 'yesterday';

  const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));
  if (diffDays < 7) return 'lastWeek';
  if (date.getMonth() === now.getMonth() && date.getFullYear() === now.getFullYear()) return 'thisMonth';
  return 'older';
}
