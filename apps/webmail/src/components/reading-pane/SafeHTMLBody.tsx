'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';

interface SafeHTMLBodyProps {
  html: string;
  onMailto?: (addr: string) => void;
  externalImages?: string;
}

export function SafeHTMLBody({ html, onMailto, externalImages = 'ask' }: SafeHTMLBodyProps) {
  const t = useTranslations('readingPaneSafe');
  const blockTrackingPixels = useMemo(() => {
    try {
      return JSON.parse(localStorage.getItem('webmail_settings') ?? '{}').blockTrackingPixels !== false;
    } catch {
      return true;
    }
  }, []);
  const ref = useRef<HTMLDivElement>(null);
  const [showImages, setShowImages] = useState(externalImages === 'always');
  const [showQuoted, setShowQuoted] = useState(false);
  const hasImages = useMemo(() => /<img\s/i.test(html), [html]);
  const hasQuoted = useMemo(() => /<blockquote/i.test(html), [html]);

  useEffect(() => {
    setShowQuoted(false);
    setShowImages(false);
  }, [html]);

  useEffect(() => {
    if (!ref.current) return;

    import('dompurify').then(({ default: DOMPurify }) => {
      if (!ref.current) return;

      const forbidTags: string[] = ['script', 'style', 'iframe', 'form', 'input', 'object', 'embed', 'svg', 'math'];
      if (!showImages) forbidTags.push('img');

      const clean = DOMPurify.sanitize(html, {
        USE_PROFILES: { html: true },
        FORBID_TAGS: forbidTags,
        FORBID_ATTR: ['style'],
        ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto):|[^a-z]|[a-z+.-]+(?:[^a-z+.-:]|$))/i,
      });

      ref.current.innerHTML = clean;

      if (showImages) {
        ref.current.querySelectorAll('img[src]').forEach((img) => {
          const src = img.getAttribute('src') ?? '';
          if (src.startsWith('http://') || src.startsWith('https://')) {
            img.setAttribute('src', `/api/image-proxy?url=${encodeURIComponent(src)}`);
          }
        });
      }

      if (blockTrackingPixels && showImages) {
        ref.current.querySelectorAll('img').forEach((img) => {
          const w = img.getAttribute('width');
          const h = img.getAttribute('height');
          const isPixel = (w === '1' || w === '0') && (h === '1' || h === '0');
          const src = img.getAttribute('src') ?? '';
          const isTracker = /track|pixel|beacon|open\.|email\.([a-z]+\.)+[a-z]+\/|\?t=|\.gif\?/i.test(src);

          if (isPixel || isTracker) img.remove();
        });
      }

      ref.current.querySelectorAll('a[href]').forEach((el) => {
        const a = el as HTMLAnchorElement;
        const href = a.getAttribute('href') ?? '';
        if (href.startsWith('mailto:')) {
          a.addEventListener('click', (e) => {
            e.preventDefault();
            const addr = href.replace(/^mailto:/i, '').split('?')[0];
            onMailto?.(addr);
          });
        } else if (href.startsWith('http://') || href.startsWith('https://')) {
          a.setAttribute('rel', 'noopener noreferrer nofollow');
          a.setAttribute('target', '_blank');
        } else {
          a.removeAttribute('href');
        }
      });

      if (hasQuoted && !showQuoted) {
        ref.current.querySelectorAll('blockquote').forEach((bq) => {
          (bq as HTMLElement).style.display = 'none';
        });
      }
    });
  }, [html, showImages, showQuoted, hasQuoted, onMailto, blockTrackingPixels]);

  return (
    <>
      {hasImages && !showImages && externalImages !== 'never' && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: '8px 16px',
            background: 'var(--color-bg-secondary)',
            borderBottom: '1px solid var(--color-border-subtle)',
            fontSize: '13px',
            color: 'var(--color-text-secondary)',
          }}
        >
          <span>{t('remoteImagesBlocked')}</span>
          <button
            type="button"
            onClick={() => setShowImages(true)}
            style={{
              fontSize: '13px',
              color: 'var(--color-accent)',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              padding: 0,
              fontWeight: 500,
            }}
          >
            {t('showImages')}
          </button>
        </div>
      )}
      <div ref={ref} style={{ wordBreak: 'break-word', lineHeight: 1.6 }} />
      {hasQuoted && (
        <button
          type="button"
          onClick={() => setShowQuoted((v) => !v)}
          style={{
            marginTop: '8px',
            fontSize: '12px',
            color: 'var(--color-text-tertiary)',
            background: 'var(--color-bg-secondary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '4px',
            cursor: 'pointer',
            padding: '3px 10px',
          }}
        >
          {showQuoted ? t('hideQuoted') : t('showQuoted')}
        </button>
      )}
    </>
  );
}
