import { useState, useEffect, useCallback } from 'react';

export interface UseMailSessionParams {
  router: { push: (href: string) => void };
  t: (key: string, values?: Record<string, unknown>) => string;
}

export function useMailSession({ router, t }: UseMailSessionParams) {
  const [userEmail, setUserEmail] = useState('');
  const [mustChangePassword, setMustChangePassword] = useState(false);
  const [sessionWarning, setSessionWarning] = useState<string | null>(null);

  const DEV_USER_ID = process.env.NODE_ENV !== 'production' ? (process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID || '') : '';
  const DEV_SKIP_LOGIN = process.env.NODE_ENV !== 'production' && process.env.NEXT_PUBLIC_GOGOMAIL_DEV_SKIP_LOGIN === 'true';

  // Check auth on mount, load email
  useEffect(() => {
    const authenticated = localStorage.getItem('webmail_authenticated');
    if (!authenticated && !(DEV_SKIP_LOGIN && DEV_USER_ID)) { router.push('/login'); return; }
    let email = localStorage.getItem('webmail_email') ?? '';
    if (!email && DEV_SKIP_LOGIN && DEV_USER_ID.includes('@')) email = DEV_USER_ID;
    setUserEmail(email);
    if (localStorage.getItem('webmail_must_change_password') === '1') {
      setMustChangePassword(true);
    }
  }, [router, DEV_SKIP_LOGIN, DEV_USER_ID]);

  // Session expiry warning: check every 60s, warn when < 10 min left
  useEffect(() => {
    function check() {
      const expiresAt = localStorage.getItem('webmail_token_expires_at');
      if (!expiresAt) { setSessionWarning(null); return; }
      const msLeft = new Date(expiresAt).getTime() - Date.now();
      if (msLeft <= 0) { setSessionWarning(t('misc.mailPage.sessionExpired')); return; }
      const minsLeft = Math.floor(msLeft / 60000);
      if (minsLeft < 10) setSessionWarning(t('misc.mailPage.sessionExpiresIn', { mins: minsLeft }));
      else setSessionWarning(null);
    }
    check();
    const id = setInterval(check, 60000);
    return () => clearInterval(id);
  }, [t]);

  const handleLogout = useCallback(() => {
    fetch('/api/auth/logout', { method: 'POST' }).catch(() => {}); // fire-and-forget: logout navigates away regardless
    localStorage.removeItem('webmail_authenticated');
    localStorage.removeItem('webmail_email');
    localStorage.removeItem('webmail_must_change_password');
    localStorage.removeItem('webmail_token_expires_at');
    router.push('/login');
  }, [router]);

  return {
    userEmail,
    setUserEmail,
    mustChangePassword,
    setMustChangePassword,
    sessionWarning,
    setSessionWarning,
    handleLogout,
  };
}
