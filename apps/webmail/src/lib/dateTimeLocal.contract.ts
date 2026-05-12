import { toDateTimeLocalValue } from './dateTimeLocal';

const singleDigitDateTime: string = toDateTimeLocalValue(new Date(2026, 0, 2, 3, 4));
const doubleDigitDateTime: string = toDateTimeLocalValue(new Date(2026, 10, 12, 13, 45));

export const dateTimeLocalContract = {
  singleDigitDateTime,
  doubleDigitDateTime,
} as const;
