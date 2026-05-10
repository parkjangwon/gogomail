'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Spinner } from '@cloudscape-design/components';

export default function RootPage() {
  const router = useRouter();

  useEffect(() => {
    // Check if user is authenticated by checking for auth cookie
    // If authenticated, redirect to dashboard; otherwise to login
    const checkAuth = async () => {
      try {
        const response = await fetch('/api/auth/verify', {
          credentials: 'include',
        });

        if (response.ok) {
          router.replace('/dashboard');
        } else {
          router.replace('/login');
        }
      } catch (error) {
        // If API call fails, redirect to login
        router.replace('/login');
      }
    };

    checkAuth();
  }, [router]);

  return (
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      height: '100vh',
      width: '100vw',
    }}>
      <Spinner />
    </div>
  );
}
