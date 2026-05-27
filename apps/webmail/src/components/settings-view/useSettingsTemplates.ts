import { useState } from 'react';
import { type StoredEmailTemplate } from '@/lib/emailTemplates';

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
export function useSettingsTemplates() {
  const [templates, setTemplates] = useState<StoredEmailTemplate[]>([]);
  const [newTplName, setNewTplName] = useState('');
  const [newTplSubject, setNewTplSubject] = useState('');
  const [newTplBody, setNewTplBody] = useState('');
  const [showNewTpl, setShowNewTpl] = useState(false);

  return {
    templates, setTemplates,
    newTplName, setNewTplName,
    newTplSubject, setNewTplSubject,
    newTplBody, setNewTplBody,
    showNewTpl, setShowNewTpl,
  };
}
