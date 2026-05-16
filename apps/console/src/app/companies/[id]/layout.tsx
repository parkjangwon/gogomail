'use client';

import { useEffect, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
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
  const pathname = usePathname();
  const loginPath = `/login?next=${encodeURIComponent(pathname)}`;

  useEffect(() => {
    (async () => {
      const { id } = await params;
      setCompanyId(id);
      try {
        const res = await fetch('/api/admin/auth/verify', { credentials: 'include' });
        if (!res.ok) {
          router.replace(loginPath);
          return;
        }
        if (id === 'default') {
          const companiesRes = await fetch('/api/admin/companies?limit=1', { credentials: 'include' });
          if (companiesRes.status === 401) {
            router.replace(loginPath);
            return;
          }
          if (companiesRes.ok) {
            const data = await companiesRes.json() as { companies?: Array<{ id?: string }> };
            const resolvedCompanyId = data.companies?.[0]?.id;
            if (resolvedCompanyId) {
              const nextPath = pathname.replace('/companies/default', `/companies/${resolvedCompanyId}`);
              router.replace(nextPath);
              return;
            }
          }
        }
        setAuthorized(true);
        setResolved(true);
      } catch {
        router.replace(loginPath);
      }
    })();
  }, [loginPath, params, pathname, router]);

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
