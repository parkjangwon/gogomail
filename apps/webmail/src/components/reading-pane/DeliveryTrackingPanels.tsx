'use client';

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
  if (!isSent) return null;

  return (
    <>
      {deliveryStatus && (
        <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
          <button
            onClick={() => setDeliveryOpen(!deliveryOpen)}
            style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: deliveryOpen ? '8px' : 0 }}
          >
            <span style={{ fontSize: '11px', transform: deliveryOpen ? 'rotate(90deg)' : 'rotate(0deg)', display: 'inline-block', transition: 'transform 150ms' }}>▶</span>
            배달 현황
            <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, fontSize: '12px', color: deliveryStatus.delivery_status === 'delivered' ? 'var(--color-success, #22c55e)' : deliveryStatus.delivery_status === 'failed' ? 'var(--color-destructive)' : 'var(--color-text-tertiary)' }}>
              ({deliveryStatus.delivery_status === 'delivered' ? '전달됨' : deliveryStatus.delivery_status === 'failed' ? '실패' : deliveryStatus.delivery_status === 'partial' ? '일부 실패' : '대기 중'})
            </span>
          </button>
          {deliveryOpen && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {deliveryStatus.attempts.length === 0 ? (
                <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '6px 0' }}>배달 기록이 없습니다.</div>
              ) : (
                deliveryStatus.attempts.map((attempt, index) => {
                  const isOk = attempt.status === 'delivered' || attempt.status === 'success';
                  const isFail = attempt.status === 'failed' || attempt.status === 'bounced' || attempt.status === 'error';
                  const statusColor = isOk ? 'var(--color-success, #22c55e)' : isFail ? 'var(--color-destructive)' : 'var(--color-text-tertiary)';
                  const statusLabel = isOk ? '전달됨' : isFail ? '실패' : attempt.status === 'pending' ? '대기 중' : attempt.status;
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
            수신확인
            <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
              ({trackingEvents.filter((event) => event.open_count > 0).length}/{trackingEvents.length} 열람)
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
                        {event.open_count > 1 && ` (${event.open_count}회)`}
                      </span>
                    )}
                    {!opened && (
                      <span style={{ color: 'var(--color-text-tertiary)', fontSize: '12px' }}>미열람</span>
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
