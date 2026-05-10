'use client';

import { AppLayout, Flashbar } from '@cloudscape-design/components';
import { Sidebar } from './Sidebar';
import { useState } from 'react';

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const [notifications, setNotifications] = useState<any[]>([]);

  return (
    <AppLayout
      navigation={<Sidebar />}
      content={children}
      toolsHide
      notifications={
        notifications.length > 0 ? (
          <Flashbar items={notifications} />
        ) : undefined
      }
    />
  );
}
