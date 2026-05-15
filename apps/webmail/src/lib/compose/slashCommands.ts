export const SLASH_COMMANDS = [
  { id: 'h1', label: '제목 1', desc: 'Heading 1', icon: 'H1' },
  { id: 'h2', label: '제목 2', desc: 'Heading 2', icon: 'H2' },
  { id: 'h3', label: '제목 3', desc: 'Heading 3', icon: 'H3' },
  { id: 'bullet', label: '글머리 목록', desc: 'Bullet list', icon: '•' },
  { id: 'numbered', label: '번호 목록', desc: 'Numbered list', icon: '1.' },
  { id: 'quote', label: '인용문', desc: 'Blockquote / callout', icon: '"' },
  { id: 'code', label: '코드 블록', desc: 'Code block', icon: '</>' },
  { id: 'hr', label: '구분선', desc: 'Horizontal divider', icon: '—' },
  { id: 'bold', label: '굵게', desc: 'Bold text', icon: 'B' },
  { id: 'italic', label: '기울임', desc: 'Italic text', icon: 'I' },
] as const;

export type SlashCommand = typeof SLASH_COMMANDS[number];
