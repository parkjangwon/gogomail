import { useCallback, useEffect, useRef, useState } from 'react';
import { Attachment } from '@/lib/api';

interface UseReadingPaneMediaParams {
  messageId: string | undefined;
  attachments: Attachment[];
}

interface UseReadingPaneMediaResult {
  imagePreviews: Record<string, string>;
  lightbox: { url: string; filename: string; attId: string } | null;
  setLightbox: React.Dispatch<React.SetStateAction<{ url: string; filename: string; attId: string } | null>>;
  pdfPreview: { url: string; filename: string } | null;
  setPdfPreview: React.Dispatch<React.SetStateAction<{ url: string; filename: string } | null>>;
  pdfPreviewLoadingId: string | null;
  onOpenImage: (url: string, filename: string, attId: string) => void;
  handlePdfPreview: (att: Attachment) => Promise<void>;
}

export function useReadingPaneMedia({
  messageId,
  attachments,
}: UseReadingPaneMediaParams): UseReadingPaneMediaResult {
  const [imagePreviews, setImagePreviews] = useState<Record<string, string>>({});
  const [lightbox, setLightbox] = useState<{ url: string; filename: string; attId: string } | null>(null);
  const [pdfPreview, setPdfPreview] = useState<{ url: string; filename: string } | null>(null);
  const [pdfPreviewLoadingId, setPdfPreviewLoadingId] = useState<string | null>(null);
  const imagePreviewsRef = useRef<Record<string, string>>({});

  useEffect(() => {
    if (!lightbox) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setLightbox(null);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [lightbox]);

  useEffect(() => {
    if (!pdfPreview) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setPdfPreview(null);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [pdfPreview]);

  useEffect(() => {
    const url = pdfPreview?.url;
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [pdfPreview]);

  useEffect(() => {
    // Revoke URLs from the previous message to prevent blob URL leaks
    const prevUrls = imagePreviewsRef.current;
    Object.values(prevUrls).forEach((url) => URL.revokeObjectURL(url));
    imagePreviewsRef.current = {};
    setImagePreviews({});

    if (!messageId || attachments.length === 0) return;
    const imageAttachments = attachments.filter((a) => a.mime_type.startsWith('image/') && a.status === 'stored');
    let cancelled = false;
    imageAttachments.forEach((att) => {
      fetch(`/api/mail/messages/${messageId}/attachments/${att.id}/download`)
        .then((response) => response.ok ? response.blob() : null)
        .then((blob) => {
          if (!blob || cancelled) return;
          const url = URL.createObjectURL(blob);
          imagePreviewsRef.current[att.id] = url;
          setImagePreviews((current) => ({ ...current, [att.id]: url }));
        })
        .catch(() => {});
    });
    return () => {
      cancelled = true;
    };
  }, [attachments, messageId]);

  useEffect(() => {
    const urls = imagePreviewsRef.current;
    return () => {
      Object.values(urls).forEach((url) => URL.revokeObjectURL(url));
    };
  }, []);

  const onOpenImage = (url: string, filename: string, attId: string) => {
    setLightbox({ url, filename, attId });
  };

  const handlePdfPreview = useCallback(async (att: Attachment) => {
    if (!messageId) return;
    setPdfPreviewLoadingId(att.id);
    try {
      const res = await fetch(`/api/mail/messages/${messageId}/attachments/${att.id}/download`);
      if (!res.ok) return;
      const blob = await res.blob();
      setPdfPreview({ url: URL.createObjectURL(blob), filename: att.filename });
    } catch {
      // ignore
    } finally {
      setPdfPreviewLoadingId(null);
    }
  }, [messageId]);

  return {
    imagePreviews,
    lightbox,
    setLightbox,
    pdfPreview,
    setPdfPreview,
    pdfPreviewLoadingId,
    onOpenImage,
    handlePdfPreview,
  };
}
