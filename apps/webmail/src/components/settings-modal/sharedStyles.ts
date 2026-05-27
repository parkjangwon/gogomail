import type React from 'react';

export const labelStyle: React.CSSProperties = {
  fontSize: '13px',
  fontWeight: 500,
  color: 'var(--color-text-primary)',
  marginBottom: '8px',
  display: 'block',
};

export const sectionStyle: React.CSSProperties = { marginBottom: '24px' };

export const radioGroupStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: '6px',
};

export const radioLabelStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: '8px',
  fontSize: '13px',
  color: 'var(--color-text-secondary)',
  cursor: 'pointer',
};
