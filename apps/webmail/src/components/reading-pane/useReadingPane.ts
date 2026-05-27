import { useCallback, useEffect, useRef, useState } from 'react';
import { MessageDetail } from '@/lib/api';

interface UseReadingPaneParams {
  message: MessageDetail | null;
}

interface UseReadingPaneReturn {
  fontSize: number;
  savedContact: boolean;
  setSavedContact: (v: boolean) => void;
  scrollProgress: number;
  emailDarkMode: boolean;
  setEmailDarkMode: React.Dispatch<React.SetStateAction<boolean>>;
  copiedEmail: string;
  setCopiedEmail: (v: string) => void;
  inlineCompose: {
    intent: 'reply' | 'reply_all' | 'forward';
    to: string;
    subject: string;
  } | null;
  setInlineCompose: React.Dispatch<React.SetStateAction<{
    intent: 'reply' | 'reply_all' | 'forward';
    to: string;
    subject: string;
  } | null>>;
  scrollContainerRef: React.RefObject<HTMLElement | null>;
  copyTimerRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>;
  increaseFontSize: () => void;
  decreaseFontSize: () => void;
  handleReadingScroll: () => void;
  handleSaveContact: () => void;
}

export function useReadingPane({ message }: UseReadingPaneParams): UseReadingPaneReturn {
  const [fontSize, setFontSize] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_font_size') ?? '14', 10) || 14; } catch { return 14; }
  });
  const [savedContact, setSavedContact] = useState(false);
  const [scrollProgress, setScrollProgress] = useState(0);
  const [emailDarkMode, setEmailDarkMode] = useState(false);
  const [copiedEmail, setCopiedEmail] = useState('');
  const [inlineCompose, setInlineCompose] = useState<{
    intent: 'reply' | 'reply_all' | 'forward';
    to: string;
    subject: string;
  } | null>(null);

  const scrollContainerRef = useRef<HTMLElement>(null);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    localStorage.setItem('webmail_font_size', String(fontSize));
  }, [fontSize]);

  useEffect(() => {
    setSavedContact(false);
    setScrollProgress(0);
  }, [message?.id]);

  useEffect(() => {
    setInlineCompose(null);
  }, [message?.id]);

  const increaseFontSize = useCallback(() => {
    setFontSize((current) => Math.min(24, current + 1));
  }, []);

  const decreaseFontSize = useCallback(() => {
    setFontSize((current) => Math.max(11, current - 1));
  }, []);

  const handleReadingScroll = () => {
    const container = scrollContainerRef.current;
    if (!container) return;
    const max = container.scrollHeight - container.clientHeight;
    setScrollProgress(max > 0 ? Math.round((container.scrollTop / max) * 100) : 0);
  };

  const handleSaveContact = () => {
    if (!message) return;
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      contacts[message.from_addr.toLowerCase()] = message.from_name || message.from_addr;
      localStorage.setItem('webmail_contacts', JSON.stringify(contacts));
    } catch {
      // ignore
    }
    setSavedContact(true);
    setTimeout(() => setSavedContact(false), 2000);
  };

  return {
    fontSize,
    savedContact,
    setSavedContact,
    scrollProgress,
    emailDarkMode,
    setEmailDarkMode,
    copiedEmail,
    setCopiedEmail,
    inlineCompose,
    setInlineCompose,
    scrollContainerRef,
    copyTimerRef,
    increaseFontSize,
    decreaseFontSize,
    handleReadingScroll,
    handleSaveContact,
  };
}
