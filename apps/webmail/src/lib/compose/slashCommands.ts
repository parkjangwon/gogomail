export const SLASH_COMMANDS = [
  { id: 'h1', label: 'Heading 1', labelKey: 'misc.slashCommands.h1', desc: 'Heading 1', icon: 'H1' },
  { id: 'h2', label: 'Heading 2', labelKey: 'misc.slashCommands.h2', desc: 'Heading 2', icon: 'H2' },
  { id: 'h3', label: 'Heading 3', labelKey: 'misc.slashCommands.h3', desc: 'Heading 3', icon: 'H3' },
  { id: 'bullet', label: 'Bullet list', labelKey: 'misc.slashCommands.bullet', desc: 'Bullet list', icon: '•' },
  { id: 'numbered', label: 'Numbered list', labelKey: 'misc.slashCommands.numbered', desc: 'Numbered list', icon: '1.' },
  { id: 'quote', label: 'Quote', labelKey: 'misc.slashCommands.quote', desc: 'Blockquote / callout', icon: '"' },
  { id: 'code', label: 'Code block', labelKey: 'misc.slashCommands.code', desc: 'Code block', icon: '</>' },
  { id: 'hr', label: 'Divider', labelKey: 'misc.slashCommands.hr', desc: 'Horizontal divider', icon: '—' },
  { id: 'bold', label: 'Bold', labelKey: 'misc.slashCommands.bold', desc: 'Bold text', icon: 'B' },
  { id: 'italic', label: 'Italic', labelKey: 'misc.slashCommands.italic', desc: 'Italic text', icon: 'I' },
] as const;

export type SlashCommand = typeof SLASH_COMMANDS[number];
