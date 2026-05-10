'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Spinner } from '@cloudscape-design/components';
import { AdminLayout } from '@/components/AdminLayout';
import { CompanyProvider } from '@/contexts/CompanyContext';

export default function CompanyLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ id: string }>;
}) {
  const [resolved, setResolved] = useState(false);
  const [authorized, setAuthorized] = useState(false);
  const [companyId, setCompanyId] = useState<string>('default');
  const router = useRouter();

  useEffect(() => {
    (async () => {
      const { id } = await params;
      setCompanyId(id);
      try {
        const res = await fetch('/api/admin/auth/verify', { credentials: 'include' });
        if (!res.ok) {
          router.replace('/login');
          return;
        }
        setAuthorized(true);
      } catch {
        router.replace('/login');
      } finally {
        setResolved(true);
      }
    })();
  }, [params, router]);

  if (!resolved) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spinner size="large" />
      </div>
    );
  }

  if (!authorized) return null;

  return (
    <CompanyProvider initialCompanyId={companyId}>
      <AdminLayout>{children}</AdminLayout>
    </CompanyProvider>
  );
}
