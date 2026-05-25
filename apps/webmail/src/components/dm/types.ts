/** Shared type aliases for the DM panel module. */
import { type useTranslations } from 'next-intl';

/** next-intl translator function — matches ReturnType<typeof useTranslations>. */
export type DMTFunction = ReturnType<typeof useTranslations>;
