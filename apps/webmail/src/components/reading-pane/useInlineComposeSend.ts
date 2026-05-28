'use client';

import { useState } from 'react';
import type { Editor } from '@tiptap/react';
import { parseAddrs } from '@/lib/message/messageUtils';
import { sendMessage } from '@/lib/api';
import { backendComposeIntent } from './readingPaneHelpers';

interface UseSendOptions {
  editor: Editor | null;
  to: string;
  cc: string;
  bcc: string;
  subject: string;
  messageId: string;
  intent: 'reply' | 'reply_all' | 'forward';
  attachments: Array<{ id: string; uploading?: boolean }>;
  onClose: () => void;
}

export function useInlineComposeSend({
  editor,
  to,
  cc,
  bcc,
  subject,
  messageId,
  intent,
  attachments,
  onClose,
}: UseSendOptions) {
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  function doSend() {
    if (sending || !editor) return;

    const toAddrs = parseAddrs(to);
    if (!toAddrs.length) return;
    const ccAddrs = parseAddrs(cc);
    const bccAddrs = parseAddrs(bcc);
    const normalizedIntent = backendComposeIntent(intent);
    const isReplyOrForward = normalizedIntent !== 'new';

    setSending(true);
    sendMessage({
      to: toAddrs,
      cc: ccAddrs.length ? ccAddrs : undefined,
      bcc: bccAddrs.length ? bccAddrs : undefined,
      subject,
      text_body: editor.getText(),
      html_body: editor.getHTML(),
      ...(isReplyOrForward && { intent: normalizedIntent, source_message_id: messageId }),
      attachment_ids: attachments.filter((a) => !a.uploading).map((a) => a.id),
    })
      .then(() => {
        setSent(true);
        setTimeout(() => onClose(), 1500);
      })
      .catch(() => {})
      .finally(() => setSending(false));
  }

  return { sending, sent, doSend };
}
