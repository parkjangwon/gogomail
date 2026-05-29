import { useState } from 'react';
import type { useTranslations } from 'next-intl';
import { updateUserProfile, uploadUserAvatar, deleteUserAvatar, changePassword, revokeAllSessions, setPreferences, type UserProfile } from '@/lib/api';
import { setWebmailAvatar } from '@/lib/webmailAvatar';

export interface UseSettingsAccountParams {
  t: ReturnType<typeof useTranslations>;
  router: { push: (href: string) => void };
}

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
export function useSettingsAccount({ t, router }: UseSettingsAccountParams) {
  // Account
  const [displayName, setDisplayName] = useState('');
  const [nameSaved, setNameSaved] = useState(false);
  const [recoveryEmail, setRecoveryEmail] = useState('');
  const [recoverySaved, setRecoverySaved] = useState(false);
  const [recoveryError, setRecoveryError] = useState('');
  const [signature, setSignature] = useState('');
  const [sigSaved, setSigSaved] = useState(false);
  const [avatarUrl, setAvatarUrl] = useState('');
  const [avatarSaving, setAvatarSaving] = useState(false);
  const [avatarError, setAvatarError] = useState('');

  // User profile
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [pwCurrent, setPwCurrent] = useState('');
  const [pwNew, setPwNew] = useState('');
  const [pwConfirm, setPwConfirm] = useState('');
  const [pwError, setPwError] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwSaved, setPwSaved] = useState(false);

  // Security
  const [revokingAll, setRevokingAll] = useState(false);
  const [revokeAllError, setRevokeAllError] = useState('');

  async function handleAvatarUpload(file: File | undefined) {
    if (!file) return;
    setAvatarError('');
    setAvatarSaving(true);
    try {
      const url = await uploadUserAvatar(file);
      setAvatarUrl(url);
      setWebmailAvatar(url);
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : t('avatarUploadError'));
    } finally {
      setAvatarSaving(false);
    }
  }

  async function handleAvatarRemove() {
    setAvatarError('');
    setAvatarSaving(true);
    try {
      await deleteUserAvatar();
      setAvatarUrl('');
      setWebmailAvatar('');
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : t('avatarRemoveError'));
    } finally {
      setAvatarSaving(false);
    }
  }

  function saveDisplayName() {
    try { localStorage.setItem('webmail_display_name', displayName); } catch { /* ignore */ }
    updateUserProfile({ display_name: displayName }).catch((err) => {
      console.error('Failed to save display name:', err instanceof Error ? err.message : err);
    });
    setNameSaved(true);
    setTimeout(() => setNameSaved(false), 2000);
  }

  async function saveRecoveryEmail() {
    setRecoveryError('');
    try {
      await updateUserProfile({ recovery_email: recoveryEmail.trim() });
      setRecoverySaved(true);
      setTimeout(() => setRecoverySaved(false), 2000);
    } catch (err) {
      setRecoveryError(err instanceof Error ? err.message : t('recoverySaveError'));
    }
  }

  function saveSignature() {
    try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ }
    setPreferences({ signatures: { default: signature } }).catch((err) => {
      console.error('Failed to save signature:', err instanceof Error ? err.message : err);
    });
    setSigSaved(true);
    setTimeout(() => setSigSaved(false), 2000);
  }

  async function handleChangePassword() {
    setPwError('');
    if (!pwCurrent || !pwNew || !pwConfirm) { setPwError(t('pwAllRequired')); return; }
    if (pwNew.length < 8) { setPwError(t('pwMinLength')); return; }
    if (pwNew !== pwConfirm) { setPwError(t('pwMismatch')); return; }
    setPwSaving(true);
    try {
      await changePassword(pwCurrent, pwNew);
      setPwCurrent(''); setPwNew(''); setPwConfirm('');
      setPwSaved(true);
      setTimeout(() => setPwSaved(false), 3000);
    } catch (err) {
      setPwError(err instanceof Error ? err.message : t('pwChangeFailed'));
    } finally {
      setPwSaving(false);
    }
  }

  async function handleRevokeAll() {
    if (!window.confirm(t('revokeAllConfirm'))) return;
    setRevokeAllError('');
    setRevokingAll(true);
    try {
      const ok = await revokeAllSessions();
      if (ok) {
        try { localStorage.removeItem('webmail_token'); localStorage.removeItem('webmail_email'); } catch { /* ignore */ }
        router.push('/login');
      } else {
        setRevokeAllError(t('revokeFailed'));
      }
    } catch {
      setRevokeAllError(t('revokeFailed'));
    } finally {
      setRevokingAll(false);
    }
  }

  return {
    // Account
    displayName, setDisplayName,
    nameSaved, setNameSaved,
    recoveryEmail, setRecoveryEmail,
    recoverySaved, setRecoverySaved,
    recoveryError, setRecoveryError,
    signature, setSignature,
    sigSaved, setSigSaved,
    avatarUrl, setAvatarUrl,
    avatarSaving, setAvatarSaving,
    avatarError, setAvatarError,
    // User profile
    profile, setProfile,
    pwCurrent, setPwCurrent,
    pwNew, setPwNew,
    pwConfirm, setPwConfirm,
    pwError, setPwError,
    pwSaving, setPwSaving,
    pwSaved, setPwSaved,
    // Security
    revokingAll, setRevokingAll,
    revokeAllError, setRevokeAllError,
    // Handlers
    handleAvatarUpload,
    handleAvatarRemove,
    saveDisplayName,
    saveRecoveryEmail,
    saveSignature,
    handleChangePassword,
    handleRevokeAll,
  };
}
