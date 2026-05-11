'use client';

import { useEffect, useState } from 'react';
import { ComposeModal } from '@/components/ComposeModal';
import { Providers } from '@/components/Providers';

export default function ComposePage() {
  const [userEmail, setUserEmail] = useState('');
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setUserEmail(localStorage.getItem('webmail_email') ?? '');
    setMounted(true);
  }, []);

  if (!mounted) return null;

  return (
    <Providers>
      <ComposeModal
        intent="new"
        userEmail={userEmail}
        onClose={() => window.close()}
        isMobile={false}
      />
    </Providers>
  );
}
