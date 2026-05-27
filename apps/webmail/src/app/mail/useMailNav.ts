import { useState, useEffect, type Dispatch, type SetStateAction } from 'react';
import { type AppId } from '@/components/AppIconBar';
import { getInitialActiveApp, WEBMAIL_ACTIVE_APP_KEY } from './mailPageHelpers';

interface UseMailNavReturn {
  activeApp: AppId;
  setActiveApp: Dispatch<SetStateAction<AppId>>;
  activeFolderId: string;
  setActiveFolderId: Dispatch<SetStateAction<string>>;
  selectedMessageId: string | null;
  setSelectedMessageId: Dispatch<SetStateAction<string | null>>;
}

export function useMailNav(): UseMailNavReturn {
  const [activeApp, setActiveApp] = useState<AppId>(getInitialActiveApp);
  const [activeFolderId, setActiveFolderId] = useState('');
  const [selectedMessageId, setSelectedMessageId] = useState<string | null>(null);

  // Persist active app to localStorage and URL search params
  useEffect(() => {
    try {
      localStorage.setItem(WEBMAIL_ACTIVE_APP_KEY, activeApp);
    } catch {
      // ignore
    }

    try {
      const url = new URL(window.location.href);
      if (activeApp === 'mail') url.searchParams.delete('app');
      else url.searchParams.set('app', activeApp);
      const nextUrl = `${url.pathname}${url.search}${url.hash}`;
      const currentUrl = `${window.location.pathname}${window.location.search}${window.location.hash}`;
      if (nextUrl !== currentUrl) window.history.replaceState(window.history.state, '', nextUrl);
    } catch {
      // ignore
    }
  }, [activeApp]);

  return {
    activeApp,
    setActiveApp,
    activeFolderId,
    setActiveFolderId,
    selectedMessageId,
    setSelectedMessageId,
  };
}
