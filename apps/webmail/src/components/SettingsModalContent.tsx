'use client';

import { type Dispatch, type SetStateAction } from 'react';
import {
  Category,
  FilterRule,
  WebmailSettings,
} from './settings/settingsConfig';
import { MailboxSection } from './settings-modal/MailboxSection';
import { ComposeSection } from './settings-modal/ComposeSection';
import { ThemeSection } from './settings-modal/ThemeSection';
import { NotificationsSection } from './settings-modal/NotificationsSection';
import { AccountSection } from './settings-modal/AccountSection';
import { SecuritySection } from './settings-modal/SecuritySection';
import { ShortcutsSection } from './settings-modal/ShortcutsSection';
import { AdvancedSection } from './settings-modal/AdvancedSection';
import { FiltersSection } from './settings-modal/FiltersSection';

interface SettingsModalContentProps {
  activeCategory: Category;
  settings: WebmailSettings;
  userEmail?: string;
  avatarUrl: string;
  setAvatarUrl: Dispatch<SetStateAction<string>>;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
  applyTheme: (theme: WebmailSettings['theme']) => void;
  handleNotificationToggle: (checked: boolean) => void;
  filterRules: FilterRule[];
  setFilterRules: Dispatch<SetStateAction<FilterRule[]>>;
  editingRule: FilterRule | null;
  setEditingRule: Dispatch<SetStateAction<FilterRule | null>>;
  newRule: Omit<FilterRule, 'id'>;
  setNewRule: Dispatch<SetStateAction<Omit<FilterRule, 'id'>>>;
}

export function SettingsModalContent({
  activeCategory,
  settings,
  userEmail,
  avatarUrl,
  setAvatarUrl,
  update,
  applyTheme,
  handleNotificationToggle,
  filterRules,
  setFilterRules,
  editingRule,
  setEditingRule,
  newRule,
  setNewRule,
}: SettingsModalContentProps) {
  switch (activeCategory) {
    case 'mailbox':
      return <MailboxSection settings={settings} update={update} />;
    case 'compose':
      return <ComposeSection settings={settings} update={update} />;
    case 'theme':
      return <ThemeSection settings={settings} update={update} applyTheme={applyTheme} />;
    case 'notifications':
      return <NotificationsSection settings={settings} handleNotificationToggle={handleNotificationToggle} />;
    case 'account':
      return <AccountSection userEmail={userEmail} avatarUrl={avatarUrl} setAvatarUrl={setAvatarUrl} />;
    case 'security':
      return <SecuritySection settings={settings} update={update} />;
    case 'shortcuts':
      return <ShortcutsSection />;
    case 'advanced':
      return <AdvancedSection settings={settings} update={update} />;
    case 'filters':
      return (
        <FiltersSection
          filterRules={filterRules}
          setFilterRules={setFilterRules}
          editingRule={editingRule}
          setEditingRule={setEditingRule}
          newRule={newRule}
          setNewRule={setNewRule}
        />
      );
  }
}
