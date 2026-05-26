import { useState, useRef, useEffect, useCallback } from 'react';
import type { Editor } from '@tiptap/react';
import { SLASH_COMMANDS, type SlashCommand } from '@/lib/compose/slashCommands';

interface SlashMenuState {
  query: string;
  top: number;
  cursorTop: number;
  left: number;
}

interface UseComposeSlashReturn {
  slashMenu: SlashMenuState | null;
  setSlashMenu: React.Dispatch<React.SetStateAction<SlashMenuState | null>>;
  slashIndex: number;
  setSlashIndex: React.Dispatch<React.SetStateAction<number>>;
  slashStartPosRef: React.MutableRefObject<number | null>;
  slashMenuRef: React.MutableRefObject<SlashMenuState | null>;
  slashIndexRef: React.MutableRefObject<number>;
  runSlashCommandRef: React.MutableRefObject<((cmd: SlashCommand) => void) | null>;
  runSlashCommand: (cmd: SlashCommand, editor: Editor | null) => void;
  filteredCommands: (query: string) => SlashCommand[];
}

export function useComposeSlash(): UseComposeSlashReturn {
  const [slashMenu, setSlashMenu] = useState<SlashMenuState | null>(null);
  const [slashIndex, setSlashIndex] = useState(0);
  const slashStartPosRef = useRef<number | null>(null);
  const slashMenuRef = useRef<SlashMenuState | null>(null);
  const slashIndexRef = useRef(0);
  const runSlashCommandRef = useRef<((cmd: SlashCommand) => void) | null>(null);

  // Keep refs in sync with state
  useEffect(() => {
    slashMenuRef.current = slashMenu;
  }, [slashMenu]);

  useEffect(() => {
    slashIndexRef.current = slashIndex;
  }, [slashIndex]);

  const filteredCommands = useCallback((query: string): SlashCommand[] => {
    return SLASH_COMMANDS.filter(
      (c) => !query || c.id.startsWith(query.toLowerCase()) || c.label.includes(query)
    );
  }, []);

  const runSlashCommand = useCallback((cmd: SlashCommand, editor: Editor | null) => {
    if (!editor || slashStartPosRef.current === null) return;
    const { from } = editor.state.selection;
    editor.chain().focus()
      .deleteRange({ from: slashStartPosRef.current, to: from })
      .run();
    switch (cmd.id) {
      case 'h1': editor.chain().focus().toggleHeading({ level: 1 }).run(); break;
      case 'h2': editor.chain().focus().toggleHeading({ level: 2 }).run(); break;
      case 'h3': editor.chain().focus().toggleHeading({ level: 3 }).run(); break;
      case 'bullet': editor.chain().focus().toggleBulletList().run(); break;
      case 'numbered': editor.chain().focus().toggleOrderedList().run(); break;
      case 'quote': editor.chain().focus().toggleBlockquote().run(); break;
      case 'code': editor.chain().focus().toggleCodeBlock().run(); break;
      case 'hr': editor.chain().focus().setHorizontalRule().run(); break;
      case 'bold': editor.chain().focus().toggleBold().run(); break;
      case 'italic': editor.chain().focus().toggleItalic().run(); break;
    }
    setSlashMenu(null);
    slashStartPosRef.current = null;
  }, []);

  return {
    slashMenu,
    setSlashMenu,
    slashIndex,
    setSlashIndex,
    slashStartPosRef,
    slashMenuRef,
    slashIndexRef,
    runSlashCommandRef,
    runSlashCommand,
    filteredCommands,
  };
}
