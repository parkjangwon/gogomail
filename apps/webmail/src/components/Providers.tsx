'use client';

import { useState, useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ErrorBoundary } from './ErrorBoundary';
import { NotificationProvider } from '@/lib/notifications/store';

function ThemeInitializer() {
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const apply = () => {
      const stored = localStorage.getItem('webmail_theme');
      if (!stored) {
        document.documentElement.setAttribute('data-theme', mq.matches ? 'dark' : 'light');
      }
    };
    const stored = localStorage.getItem('webmail_theme');
    document.documentElement.setAttribute('data-theme', stored || (mq.matches ? 'dark' : 'light'));
    mq.addEventListener('change', apply);
    return () => mq.removeEventListener('change', apply);
  }, []);
  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { staleTime: 30_000, retry: 1 },
        },
      })
  );

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ThemeInitializer />
        <NotificationProvider>{children}</NotificationProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
