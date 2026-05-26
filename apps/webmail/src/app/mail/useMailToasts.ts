'use client';

import { useState, useCallback } from 'react';
import { type ToastItem } from '@/components/Toast';
import { stableId } from '@/lib/stableId';

export function useMailToasts() {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const addToast = useCallback((
    message: string,
    type: ToastItem['type'] = 'success',
    options?: { duration?: number; action?: ToastItem['action'] },
  ) => {
    const id = stableId('toast');
    setToasts((prev) => [...prev, { id, message, type, ...options }]);
  }, []);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return { toasts, setToasts, addToast, dismissToast };
}
