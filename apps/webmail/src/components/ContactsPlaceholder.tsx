'use client';
import { UserGroupIcon } from '@heroicons/react/24/outline';
export function ContactsPlaceholder() {
  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '12px', color: 'var(--color-text-tertiary)' }}>
      <UserGroupIcon style={{ width: '48px', height: '48px', opacity: 0.4 }} />
      <span style={{ fontSize: '16px', fontWeight: 500 }}>연락처</span>
      <span style={{ fontSize: '13px' }}>준비 중입니다</span>
    </div>
  );
}
