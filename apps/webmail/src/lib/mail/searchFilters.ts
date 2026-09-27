// Pure, testable helpers for the advanced search builder: derive the list of
// active-filter "chips" from an AdvancedFilters value, and apply the
// client-side `label` operator to a result set. No React/DOM dependencies.

import type { AdvancedFilters } from '@/components/Sidebar';
import type { MessageSummary } from '@/lib/api';

export type FilterField = keyof AdvancedFilters;

export interface FilterChip {
  /** Which AdvancedFilters key this chip represents (used to clear it). */
  field: FilterField;
  /** i18n key suffix under misc.searchBar for the chip's field label. */
  labelKey: string;
  /** Display value for the chip (already resolved for folder/label). */
  value: string;
}

export interface ChipContext {
  /** Map folder id → display name, for resolving the folder chip value. */
  folderName?: (id: string) => string | undefined;
  /** Human label for a color value, for the label chip. */
  colorName?: (color: string) => string;
}

// Order chips are displayed in — mirrors the builder field order.
const CHIP_ORDER: { field: FilterField; labelKey: string }[] = [
  { field: 'from', labelKey: 'from' },
  { field: 'to', labelKey: 'to' },
  { field: 'subject', labelKey: 'subject' },
  { field: 'folder_id', labelKey: 'folder' },
  { field: 'label', labelKey: 'label' },
  { field: 'since', labelKey: 'since' },
  { field: 'until', labelKey: 'until' },
  { field: 'has_attachment', labelKey: 'hasAttachment' },
];

/**
 * Build the ordered list of active-filter chips from an AdvancedFilters object.
 * Empty/false fields produce no chip. folder_id/label resolve their display
 * value via ChipContext when provided.
 */
export function buildActiveFilterChips(
  filters: AdvancedFilters,
  ctx: ChipContext = {},
): FilterChip[] {
  const chips: FilterChip[] = [];
  for (const { field, labelKey } of CHIP_ORDER) {
    const raw = filters[field];
    if (raw === undefined || raw === '' || raw === false) continue;

    let value: string;
    if (field === 'folder_id') {
      value = ctx.folderName?.(String(raw)) ?? String(raw);
    } else if (field === 'label') {
      value = ctx.colorName?.(String(raw)) ?? String(raw);
    } else if (field === 'has_attachment') {
      value = '✓';
    } else {
      value = String(raw);
    }
    chips.push({ field, labelKey, value });
  }
  return chips;
}

/** Remove one field from an AdvancedFilters object (immutably). */
export function clearFilterField(filters: AdvancedFilters, field: FilterField): AdvancedFilters {
  const next = { ...filters };
  delete next[field];
  return next;
}

/** True when no advanced filter is active. */
export function hasActiveFilters(filters: AdvancedFilters): boolean {
  return buildActiveFilterChips(filters).length > 0;
}

/**
 * Apply the client-side `label` operator: keep only messages whose label color
 * (looked up in messageLabels) matches the requested one. Backend does not
 * understand labels, so this narrows the result set locally. A falsy `label`
 * leaves the list unchanged.
 */
export function applyLabelFilter(
  messages: MessageSummary[],
  label: string | undefined,
  messageLabels: Record<string, string>,
): MessageSummary[] {
  if (!label) return messages;
  return messages.filter((m) => messageLabels[m.id] === label);
}
