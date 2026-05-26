export function createCalendarModalStyles() {
  return {
    overlay: { position: 'fixed' as const, inset: 0, zIndex: 400, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.4)' },
    card: (w: string) => ({ background: 'var(--color-bg-primary)', borderRadius: '14px', width: w, maxWidth: 'calc(100vw - 32px)', boxShadow: '0 24px 64px rgba(0,0,0,0.22)', display: 'flex', flexDirection: 'column' as const, overflow: 'hidden' }),
    header: { padding: '20px 24px 16px', borderBottom: '1px solid var(--color-border-subtle)' },
    title: { fontSize: '16px', fontWeight: 600, color: 'var(--color-text-primary)' },
    body: { padding: '20px 24px', display: 'flex', flexDirection: 'column' as const, gap: '14px' },
    footer: { padding: '16px 24px 20px', borderTop: '1px solid var(--color-border-subtle)', display: 'flex', justifyContent: 'flex-end', gap: '8px' },
    footerSplit: { padding: '16px 24px 20px', borderTop: '1px solid var(--color-border-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' },
    label: { fontSize: '12px', color: 'var(--color-text-secondary)', display: 'block' as const, marginBottom: '4px' },
    input: { width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' as const },
    select: { width: '100%', padding: '8px 10px', fontSize: '13px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', cursor: 'pointer' },
    error: { fontSize: '12px', color: '#e53e3e' },
    cancelBtn: { padding: '8px 16px', borderRadius: '7px', border: '1px solid var(--color-border-default)', background: 'none', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 },
    primaryBtn: (disabled: boolean) => ({ padding: '8px 20px', borderRadius: '7px', border: 'none', background: disabled ? 'var(--color-bg-tertiary)' : 'var(--color-accent)', color: disabled ? 'var(--color-text-tertiary)' : '#fff', fontSize: '13px', fontWeight: 600 as const, cursor: disabled ? 'default' as const : 'pointer' as const }),
    dangerBtn: { padding: '8px 14px', borderRadius: '7px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 },
  };
}
