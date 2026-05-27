'use client';

interface MailWarningBannersProps {
  mustChangePassword: boolean;
  sessionWarning: string | null;
  isOnline: boolean;
  onDismissPasswordWarning: () => void;
  onLogout: () => void;
  onDismissSessionWarning: () => void;
  tClose: string;
  tMustChangePassword: string;
  tLoginAgain: string;
  tOffline: string;
}

export function MailWarningBanners({
  mustChangePassword,
  sessionWarning,
  isOnline,
  onDismissPasswordWarning,
  onLogout,
  onDismissSessionWarning,
  tClose,
  tMustChangePassword,
  tLoginAgain,
  tOffline,
}: MailWarningBannersProps) {
  return (
    <>
      {mustChangePassword && (
        <div
          role="status"
          aria-live="polite"
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            zIndex: 500,
            background: '#b45309',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px 40px',
            fontWeight: 500,
          }}
        >
          {tMustChangePassword}
          <button
            onClick={onDismissPasswordWarning}
            style={{ marginLeft: '12px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >{tClose}</button>
        </div>
      )}

      {sessionWarning && (
        <div
          role="alert"
          style={{
            position: 'fixed',
            top: mustChangePassword ? '33px' : 0,
            left: 0,
            right: 0,
            zIndex: 499,
            background: '#92400e',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px 40px',
            fontWeight: 500,
          }}
        >
          {sessionWarning}
          <button
            onClick={onLogout}
            style={{ marginLeft: '12px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >{tLoginAgain}</button>
          <button
            onClick={onDismissSessionWarning}
            style={{ marginLeft: '8px', background: 'none', border: '1px solid rgba(255,255,255,0.6)', color: '#fff', borderRadius: '4px', fontSize: '12px', padding: '2px 8px', cursor: 'pointer' }}
          >{tClose}</button>
        </div>
      )}

      {!isOnline && (
        <div
          role="status"
          aria-live="polite"
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            zIndex: 500,
            background: '#b45309',
            color: '#fff',
            textAlign: 'center',
            fontSize: '13px',
            padding: '6px',
            fontWeight: 500,
          }}
        >
          {tOffline}
        </div>
      )}
    </>
  );
}
