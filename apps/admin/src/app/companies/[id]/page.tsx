'use client';

import { useRouter, useParams } from 'next/navigation';
import { useEffect } from 'react';

export default function CompanyPage() {
  const router = useRouter();
  const params = useParams();

  useEffect(() => {
    const id = params.id as string;
    router.replace(`/companies/${id}/dashboard`);
  }, [params.id, router]);

  return null;
}
