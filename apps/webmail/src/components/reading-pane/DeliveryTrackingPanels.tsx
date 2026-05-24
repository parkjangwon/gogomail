'use client';

import { useTranslations } from 'next-intl';
import type { MessageDeliveryStatus, TrackingEvent } from '@/lib/api';

interface DeliveryTrackingPanelsProps {
  isSent: boolean;
  deliveryStatus: MessageDeliveryStatus | null;
  deliveryOpen: boolean;
  setDeliveryOpen: (value: boolean) => void;
  trackingEvents: TrackingEvent[] | null;
  trackingOpen: boolean;
  setTrackingOpen: (value: boolean) => void;
}

export function DeliveryTrackingPanels({
  isSent,
  deliveryStatus,
  deliveryOpen,
  setDeliveryOpen,
  trackingEvents,
  trackingOpen,
  setTrackingOpen,
}: DeliveryTrackingPanelsProps) {
  const t = useTranslations();
  if (!isSent) return null;

  const deliveryAttempts = Array.isArray(deliveryStatus?.attempts) ? deliveryStatus.attempts : [];

  return (
    <>
      {deliveryStatus && (
        <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
          <button
            onClick={() => setDeliveryOpen(!deliveryOpen)}
            style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: deliveryOpen ? '8px' : 0 }}
          >
            <span style={{ fontSize: '11px', transform: deliveryOpen ? 'rotate(90deg)' : 'rotate(0deg)', display: 'inline-block', transition: 'transform 150ms' }}>▶</span>
            {t('misc.delivery.title')}
            <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, fontSize: '12px', color: deliveryStatus.delivery_status === 'delivered' ? 'var(--color-success, #22c55e)' : deliveryStatus.delivery_status === 'failed' ? 'var(--color-destructive)' : 'var(--color-text-tertiary)' }}>
              ({deliveryStatus.delivery_status === 'delivered' ? t('misc.delivery.statusDelivered') : deliveryStatus.delivery_status === 'failed' ? t('misc.delivery.statusFailed') : deliveryStatus.delivery_status === 'partial' ? t('misc.delivery.statusPartial') : t('misc.delivery.statusPending')})
            </span>
          </button>
          {deliveryOpen && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {deliveryAttempts.length === 0 ? (
                <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '6px 0' }}>{t('misc.delivery.noAttempts')}</div>
              ) : (
                deliveryAttempts.map((attempt, index) => {
                  const isOk = attempt.status === 'delivered' || attempt.status === 'success';
                  const isFail = attempt.status === 'failed' || attempt.status === 'bounced' || attempt.status === 'error';
                  const statusColor = isOk ? 'var(--color-success, #22c55e)' : isFail ? 'var(--color-destructive)' : 'var(--color-text-tertiary)';
                  const statusLabel = isOk ? t('misc.delivery.statusDelivered') : isFail ? t('misc.delivery.statusFailed') : attempt.status === 'pending' ? t('misc.delivery.statusPending') : attempt.status;
                  const dot = isOk ? '●' : isFail ? '●' : '○';

                  return (
                    <div key={index} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '6px 10px', borderRadius: '5px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)' }}>
                      <span style={{ color: statusColor, fontSize: '10px', marginTop: '2px' }}>{dot}</span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: '13px', color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{attempt.recipient}</div>
                        {attempt.error_message && <div style={{ fontSize: '11px', color: 'var(--color-destructive)', marginTop: '2px' }}>{attempt.error_message}</div>}
                      </div>
                      <div style={{ flexShrink: 0, textAlign: 'right' }}>
                        <div style={{ fontSize: '11px', fontWeight: 600, color: statusColor }}>{statusLabel}</div>
                        {attempt.attempted_at && (
                          <div style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginTop: '1px' }}>
                            {new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(attempt.attempted_at))}
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>
      )}

      {trackingEvents && trackingEvents.length > 0 && (
        <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
          <button
            onClick={() => setTrackingOpen(!trackingOpen)}
            style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: trackingOpen ? '8px' : 0 }}
          >
            <span style={{ fontSize: '11px', transform: trackingOpen ? 'rotate(90deg)' : 'rotate(0deg)', display: 'inline-block', transition: 'transform 150ms' }}>▶</span>
            {t('misc.delivery.trackingTitle')}
            <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
              ({t('misc.delivery.trackingOpenCount', { opened: trackingEvents.filter((event) => event.open_count > 0).length, total: trackingEvents.length })})
            </span>
          </button>
          {trackingOpen && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {trackingEvents.map((event) => {
                const opened = event.open_count > 0;
                return (
                  <div key={event.recipient_email} style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 10px', borderRadius: '6px', background: 'var(--color-bg-secondary)', fontSize: '13px' }}>
                    <span style={{ color: opened ? 'var(--color-success, #22c55e)' : 'var(--color-text-tertiary)', fontSize: '14px' }}>{opened ? '✓' : '○'}</span>
                    <span style={{ flex: 1, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{event.recipient_email}</span>
                    {opened && event.opened_at && (
                      <span style={{ color: 'var(--color-text-tertiary)', fontSize: '12px', whiteSpace: 'nowrap' }}>
                        {new Intl.DateTimeFormat('ko-KR', { dateStyle: 'short', timeStyle: 'short', hour12: false }).format(new Date(event.opened_at))}
                        {event.open_count > 1 && ` ${t('misc.delivery.openCount', { count: event.open_count })}`}
                      </span>
                    )}
                    {!opened && (
                      <span style={{ color: 'var(--color-text-tertiary)', fontSize: '12px' }}>{t('misc.delivery.notOpened')}</span>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </>
  );
}
