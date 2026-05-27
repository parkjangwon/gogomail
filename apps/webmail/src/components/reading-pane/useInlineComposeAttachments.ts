'use client';

import { useState } from 'react';
import type { Editor } from '@tiptap/react';
import { uploadAttachment } from '@/lib/api';
import { stableId } from '@/lib/stableId';

export interface Attachment {
  id: string;
  filename: string;
  size: number;
  uploading?: boolean;
}

export function useInlineComposeAttachments(editor: Editor | null) {
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  async function handleImageFile(file: File) {
    if (!editor) return;
    let src: string;

    if (file.size < 500 * 1024) {
      src = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = reject;
        reader.readAsDataURL(file);
      });
    } else {
      src = URL.createObjectURL(file);
      uploadAttachment(file).then((att) => {
        setAttachments((prev) => [...prev, { id: att.id, filename: att.filename, size: att.size }]);
      }).catch(() => {});
    }

    editor.chain().focus().setImage({ src, alt: file.name }).run();
  }

  async function handleFileAttach(files: FileList) {
    for (const file of Array.from(files)) {
      const tempId = stableId('tmp');
      setAttachments((prev) => [...prev, { id: tempId, filename: file.name, size: file.size, uploading: true }]);
      try {
        const att = await uploadAttachment(file);
        setAttachments((prev) => prev.map((a) => (a.id === tempId ? { id: att.id, filename: att.filename, size: att.size } : a)));
      } catch {
        setAttachments((prev) => prev.filter((a) => a.id !== tempId));
      }
    }
  }

  return { attachments, setAttachments, handleFileAttach, handleImageFile };
}
