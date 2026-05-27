'use client';
import { useState } from 'react';

export function useSidebarFolders() {
  const [dragOverFolderId, setDragOverFolderId] = useState<string | null>(null);
  const [newFolderInput, setNewFolderInput] = useState('');
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [renamingFolderId, setRenamingFolderId] = useState<string | null>(null);
  const [renamingValue, setRenamingValue] = useState('');
  const [hoveredFolderId, setHoveredFolderId] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);

  return {
    dragOverFolderId,
    setDragOverFolderId,
    newFolderInput,
    setNewFolderInput,
    showNewFolder,
    setShowNewFolder,
    renamingFolderId,
    setRenamingFolderId,
    renamingValue,
    setRenamingValue,
    hoveredFolderId,
    setHoveredFolderId,
    showSettings,
    setShowSettings,
  };
}
