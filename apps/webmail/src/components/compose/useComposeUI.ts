'use client';

import { useState } from 'react';

interface UseComposeUIReturn {
  confirmClose: boolean;
  setConfirmClose: React.Dispatch<React.SetStateAction<boolean>>;
  closeSaveInProgress: boolean;
  setCloseSaveInProgress: React.Dispatch<React.SetStateAction<boolean>>;
  showSigEditor: boolean;
  setShowSigEditor: React.Dispatch<React.SetStateAction<boolean>>;
  signature: string;
  setSignature: React.Dispatch<React.SetStateAction<string>>;
  showEmojiPicker: boolean;
  setShowEmojiPicker: React.Dispatch<React.SetStateAction<boolean>>;
  showOrgPicker: boolean;
  setShowOrgPicker: React.Dispatch<React.SetStateAction<boolean>>;
  showSendDropdown: boolean;
  setShowSendDropdown: React.Dispatch<React.SetStateAction<boolean>>;
  imageResizeToolbar: { top: number; left: number } | null;
  setImageResizeToolbar: React.Dispatch<React.SetStateAction<{ top: number; left: number } | null>>;
  trackOpens: boolean;
  setTrackOpens: React.Dispatch<React.SetStateAction<boolean>>;
}

export function useComposeUI(): UseComposeUIReturn {
  const [confirmClose, setConfirmClose] = useState(false);
  const [closeSaveInProgress, setCloseSaveInProgress] = useState(false);
  const [showSigEditor, setShowSigEditor] = useState(false);
  const [signature, setSignature] = useState(() => {
    try { return localStorage.getItem('webmail_signature') ?? ''; } catch { return ''; }
  });
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [showOrgPicker, setShowOrgPicker] = useState(false);
  const [showSendDropdown, setShowSendDropdown] = useState(false);
  const [imageResizeToolbar, setImageResizeToolbar] = useState<{ top: number; left: number } | null>(null);
  const [trackOpens, setTrackOpens] = useState(false);

  return {
    confirmClose,
    setConfirmClose,
    closeSaveInProgress,
    setCloseSaveInProgress,
    showSigEditor,
    setShowSigEditor,
    signature,
    setSignature,
    showEmojiPicker,
    setShowEmojiPicker,
    showOrgPicker,
    setShowOrgPicker,
    showSendDropdown,
    setShowSendDropdown,
    imageResizeToolbar,
    setImageResizeToolbar,
    trackOpens,
    setTrackOpens,
  };
}
