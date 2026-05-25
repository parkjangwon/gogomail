'use client';

import { useState, useRef, useCallback, useEffect } from 'react';
import { getPreferences, setPreferences } from '@/lib/api';
import { EmailTemplate } from '@/lib/compose/composeUtils';
import { loadLocalEmailTemplates, normalizeEmailTemplates, saveLocalEmailTemplates } from '@/lib/emailTemplates';
import { stableId } from '@/lib/stableId';

interface UseComposeTemplatesOptions {
  t: (k: string) => string;
  getEditorHTML: () => string;
  subject: string;
}

interface ComposeTemplatesState {
  templates: EmailTemplate[];
  templateSaveName: string;
  setTemplateSaveName: React.Dispatch<React.SetStateAction<string>>;
  showTemplates: boolean;
  setShowTemplates: React.Dispatch<React.SetStateAction<boolean>>;
  showTemplateSave: boolean;
  setShowTemplateSave: React.Dispatch<React.SetStateAction<boolean>>;
  templateMenuRef: React.RefObject<HTMLDivElement | null>;
  persistTemplates: (next: EmailTemplate[]) => void;
  saveTemplate: () => void;
  deleteTemplate: (id: string) => void;
}

export function useComposeTemplates({ t: _t, getEditorHTML, subject }: UseComposeTemplatesOptions): ComposeTemplatesState {
  const [templates, setTemplates] = useState<EmailTemplate[]>(() => loadLocalEmailTemplates());
  const [templateSaveName, setTemplateSaveName] = useState('');
  const [showTemplates, setShowTemplates] = useState(false);
  const [showTemplateSave, setShowTemplateSave] = useState(false);
  const templateMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    getPreferences().then((prefs) => {
      if (cancelled || !prefs.templates) return;
      const serverTemplates = normalizeEmailTemplates(prefs.templates);
      setTemplates(serverTemplates);
      saveLocalEmailTemplates(serverTemplates);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (templateMenuRef.current && !templateMenuRef.current.contains(e.target as Node)) {
        setShowTemplates(false);
        setShowTemplateSave(false);
      }
    }
    if (showTemplates) document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showTemplates]);

  const persistTemplates = useCallback((next: EmailTemplate[]) => {
    const normalized = normalizeEmailTemplates(next);
    setTemplates(normalized);
    saveLocalEmailTemplates(normalized);
    setPreferences({ templates: normalized }).catch(() => {});
  }, []);

  const saveTemplate = useCallback(() => {
    const name = templateSaveName.trim();
    if (!name) return;
    const body = getEditorHTML();
    const newTemplate: EmailTemplate = { id: stableId('template'), name, subject, body };
    const updated = [...templates, newTemplate];
    persistTemplates(updated);
    setTemplateSaveName('');
    setShowTemplateSave(false);
  }, [templateSaveName, getEditorHTML, subject, templates, persistTemplates]);

  const deleteTemplate = useCallback((id: string) => {
    persistTemplates(templates.filter((tmpl) => tmpl.id !== id));
  }, [persistTemplates, templates]);

  return {
    templates,
    templateSaveName,
    setTemplateSaveName,
    showTemplates,
    setShowTemplates,
    showTemplateSave,
    setShowTemplateSave,
    templateMenuRef,
    persistTemplates,
    saveTemplate,
    deleteTemplate,
  };
}
