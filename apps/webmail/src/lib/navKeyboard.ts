'use client';

type NavDirection = 'prev' | 'next' | 'first' | 'last';

function getNavItems(group: string): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(`[data-nav-group="${group}"]`)).filter((el) => {
    if (el.hasAttribute('disabled')) return false;
    if (el.getAttribute('aria-disabled') === 'true') return false;
    return true;
  });
}

function choosePreferredItem(items: HTMLElement[], preferred?: HTMLElement | null): HTMLElement | null {
  if (preferred && items.includes(preferred)) return preferred;

  return items.find((el) =>
    el.dataset.navCurrent === 'true' ||
    el.getAttribute('aria-current') === 'page' ||
    el.getAttribute('aria-selected') === 'true'
  ) ?? items[0] ?? null;
}

export function focusNavGroup(group: string, preferred?: HTMLElement | null): HTMLElement | null {
  if (!group) return null;
  const items = getNavItems(group);
  const next = choosePreferredItem(items, preferred);
  next?.focus();
  return next;
}

export function moveNavFocus(current: HTMLElement, direction: NavDirection, group = current.dataset.navGroup ?? ''): HTMLElement | null {
  if (!group) return null;
  const items = getNavItems(group);
  if (items.length === 0) return null;

  const currentIndex = items.indexOf(current);
  if (currentIndex === -1) {
    const first = direction === 'last' ? items[items.length - 1] : items[0];
    first.focus();
    return first;
  }

  const lastIndex = items.length - 1;
  let nextIndex = currentIndex;
  if (direction === 'first') nextIndex = 0;
  else if (direction === 'last') nextIndex = lastIndex;
  else if (direction === 'next') nextIndex = currentIndex >= lastIndex ? 0 : currentIndex + 1;
  else nextIndex = currentIndex <= 0 ? lastIndex : currentIndex - 1;

  const next = items[nextIndex] ?? null;
  next?.focus();
  return next;
}

export function handleVerticalNavKeyDown(event: { key: string; preventDefault: () => void; currentTarget: HTMLElement }, group?: string) {
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault();
      moveNavFocus(event.currentTarget, 'next', group);
      return true;
    case 'ArrowUp':
      event.preventDefault();
      moveNavFocus(event.currentTarget, 'prev', group);
      return true;
    case 'Home':
      event.preventDefault();
      moveNavFocus(event.currentTarget, 'first', group);
      return true;
    case 'End':
      event.preventDefault();
      moveNavFocus(event.currentTarget, 'last', group);
      return true;
    default:
      return false;
  }
}
