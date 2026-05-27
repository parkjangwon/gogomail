'use client';
import { useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';

interface UseCalendarSubscriptionFormParams {
  handleAddSubscription: (url: string, name: string, color: string) => Promise<unknown>;
  t: (key: string, values?: Record<string, any>) => string;
}

export interface UseCalendarSubscriptionFormReturn {
  subHoverId: string | null;
  setSubHoverId: Dispatch<SetStateAction<string | null>>;
  showSubModal: boolean;
  setShowSubModal: Dispatch<SetStateAction<boolean>>;
  subUrl: string;
  setSubUrl: Dispatch<SetStateAction<string>>;
  subName: string;
  setSubName: Dispatch<SetStateAction<string>>;
  subColor: string;
  setSubColor: Dispatch<SetStateAction<string>>;
  subSaving: boolean;
  subError: string;
  setSubError: Dispatch<SetStateAction<string>>;
  handleAddSubscriptionForm: () => Promise<void>;
}

export function useCalendarSubscriptionForm({
  handleAddSubscription,
  t,
}: UseCalendarSubscriptionFormParams): UseCalendarSubscriptionFormReturn {
  const [subHoverId, setSubHoverId] = useState<string | null>(null);
  const [showSubModal, setShowSubModal] = useState(false);
  const [subUrl, setSubUrl] = useState('');
  const [subName, setSubName] = useState('');
  const [subColor, setSubColor] = useState('#4285f4');
  const [subSaving, setSubSaving] = useState(false);
  const [subError, setSubError] = useState('');

  const handleAddSubscriptionForm = async () => {
    const trimmed = subUrl.trim();
    if (!trimmed) return;
    setSubSaving(true);
    setSubError('');
    try {
      await handleAddSubscription(trimmed, subName.trim() || trimmed, subColor);
      setShowSubModal(false);
      setSubUrl('');
      setSubName('');
      setSubColor('#4285f4');
    } catch {
      setSubError(t('subscription.failed'));
    } finally {
      setSubSaving(false);
    }
  };

  return {
    subHoverId, setSubHoverId,
    showSubModal, setShowSubModal,
    subUrl, setSubUrl,
    subName, setSubName,
    subColor, setSubColor,
    subSaving, subError, setSubError,
    handleAddSubscriptionForm,
  };
}
