import type { ReactNode } from 'react';
import { DriveNode } from './api';
import {
  FolderIcon as FolderSolid,
  DocumentIcon,
  DocumentTextIcon,
  ArchiveBoxIcon,
  PhotoIcon,
} from '@heroicons/react/24/outline';

type DriveFileKind =
  | 'folder'
  | 'image'
  | 'video'
  | 'audio'
  | 'pdf'
  | 'document'
  | 'spreadsheet'
  | 'presentation'
  | 'code'
  | 'archive'
  | 'text'
  | 'binary'
  | 'unknown';

type DriveNodeIconTheme = {
  background: string;
  border: string;
  color: string;
  icon: (size: number, color: string) => ReactNode;
  badge?: string;
  badgeBg?: string;
  badgeColor?: string;
};

const DRIVE_FILE_ICON_THEMES: Record<DriveFileKind, DriveNodeIconTheme> = {
  folder: {
    background: 'var(--color-accent-subtle)',
    border: 'rgba(245, 158, 11, 0.25)',
    color: '#f59e0b',
    icon: (size, color) => <FolderSolid style={{ width: `${size}px`, height: `${size}px`, color }} />,
  },
  image: {
    background: '#dbeafe',
    border: '#bfdbfe',
    color: '#1d4ed8',
    icon: (size, color) => <PhotoIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
  },
  video: {
    background: '#ede9fe',
    border: '#ddd6fe',
    color: '#6d28d9',
    icon: (size, color) => <DocumentIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'VID',
    badgeBg: '#ede9fe',
    badgeColor: '#4c1d95',
  },
  audio: {
    background: '#dbeafe',
    border: '#bfdbfe',
    color: '#0ea5e9',
    icon: (size, color) => <DocumentIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'AUDIO',
    badgeBg: '#dbeafe',
    badgeColor: '#0369a1',
  },
  pdf: {
    background: '#fee2e2',
    border: '#fecaca',
    color: '#b91c1c',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'PDF',
    badgeBg: '#fee2e2',
    badgeColor: '#b91c1c',
  },
  document: {
    background: '#dbeafe',
    border: '#bfdbfe',
    color: '#1e40af',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'DOC',
    badgeBg: '#dbeafe',
    badgeColor: '#1d4ed8',
  },
  spreadsheet: {
    background: '#dcfce7',
    border: '#bbf7d0',
    color: '#15803d',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'XLS',
    badgeBg: '#dcfce7',
    badgeColor: '#166534',
  },
  presentation: {
    background: '#f5d0fe',
    border: '#f9a8d4',
    color: '#9d174d',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'PPT',
    badgeBg: '#f5d0fe',
    badgeColor: '#9d174d',
  },
  code: {
    background: '#ede9fe',
    border: '#ddd6fe',
    color: '#5b21b6',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'CODE',
    badgeBg: '#ede9fe',
    badgeColor: '#4c1d95',
  },
  archive: {
    background: '#fef3c7',
    border: '#fde68a',
    color: '#b45309',
    icon: (size, color) => <ArchiveBoxIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'ZIP',
    badgeBg: '#fef3c7',
    badgeColor: '#92400e',
  },
  text: {
    background: '#f3f4f6',
    border: '#e5e7eb',
    color: '#374151',
    icon: (size, color) => <DocumentTextIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'TXT',
    badgeBg: '#f3f4f6',
    badgeColor: '#1f2937',
  },
  binary: {
    background: '#f3e8ff',
    border: '#e9d5ff',
    color: '#7e22ce',
    icon: (size, color) => <DocumentIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'BIN',
    badgeBg: '#f3e8ff',
    badgeColor: '#6b21a8',
  },
  unknown: {
    background: 'var(--color-bg-secondary)',
    border: 'var(--color-border-subtle)',
    color: 'var(--color-text-secondary)',
    icon: (size, color) => <DocumentIcon style={{ width: `${size}px`, height: `${size}px`, color }} />,
    badge: 'FILE',
    badgeBg: 'var(--color-bg-secondary)',
    badgeColor: 'var(--color-text-secondary)',
  },
};

const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'tif', 'tiff', 'heic', 'avif']);
const VIDEO_EXTENSIONS = new Set(['mp4', 'mov', 'avi', 'mkv', 'webm', 'm4v', 'ogv', 'flv', 'wmv', 'm4p', 'mpeg', 'mpg']);
const AUDIO_EXTENSIONS = new Set(['mp3', 'wav', 'm4a', 'aac', 'ogg', 'oga', 'flac', 'opus', 'wma']);
const PDF_EXTENSIONS = new Set(['pdf']);
const SPREADSHEET_EXTENSIONS = new Set(['xls', 'xlsx', 'csv', 'tsv', 'ods', 'numbers']);
const PRESENTATION_EXTENSIONS = new Set(['ppt', 'pptx', 'pps', 'ppsx', 'odp', 'key']);
const TEXT_EXTENSIONS = new Set(['txt', 'md', 'markdown', 'log', 'ini', 'toml', 'yaml', 'yml', 'json', 'xml', 'html', 'css', 'scss', 'sass', 'less']);
const CODE_EXTENSIONS = new Set(['js', 'jsx', 'ts', 'tsx', 'py', 'go', 'java', 'rb', 'php', 'swift', 'kt', 'kts', 'cs', 'cpp', 'c', 'h', 'hpp', 'rs', 'lua', 'sql', 'sh', 'bash', 'r', 'scala', 'clj', 'pl']);
const DOC_EXTENSIONS = new Set(['doc', 'docx', 'odt', 'rtf', 'pages', 'txt', 'md', 'markdown']);
const ARCHIVE_EXTENSIONS = new Set(['zip', '7z', 'rar', 'tar', 'gz', 'bz2', 'xz', 'jar', 'war', 'tgz', 'tbz2']);
const BINARY_EXTENSIONS = new Set(['exe', 'msi', 'dll', 'bin', 'dmg', 'iso', 'img', 'apk']);

function getNameExtension(name: string): string {
  const idx = name.lastIndexOf('.');
  if (idx <= 0 || idx === name.length - 1) return '';
  return name.slice(idx + 1).toLowerCase();
}

function resolveDriveFileKind(node: DriveNode): DriveFileKind {
  if (node.node_type === 'folder') return 'folder';

  const mime = (node.mime_type ?? '').toLowerCase();
  const ext = getNameExtension(node.name);

  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('video/')) return 'video';

  if (mime === 'application/pdf' || PDF_EXTENSIONS.has(ext)) return 'pdf';

  if (ext && ARCHIVE_EXTENSIONS.has(ext)) return 'archive';
  if (ext && IMAGE_EXTENSIONS.has(ext)) return 'image';
  if (ext && VIDEO_EXTENSIONS.has(ext)) return 'video';
  if (ext && AUDIO_EXTENSIONS.has(ext)) return 'audio';
  if (ext && SPREADSHEET_EXTENSIONS.has(ext)) return 'spreadsheet';
  if (ext && PRESENTATION_EXTENSIONS.has(ext)) return 'presentation';
  if (ext && CODE_EXTENSIONS.has(ext)) return 'code';
  if (ext && (DOC_EXTENSIONS.has(ext) || mime.includes('word') || mime.includes('officedocument.wordprocessingml'))) return 'document';
  if (ext && TEXT_EXTENSIONS.has(ext)) return 'text';

  if (mime.includes('spreadsheet') || mime.includes('excel') || mime.includes('csv')) return 'spreadsheet';
  if (mime.includes('presentation') || mime.includes('powerpoint')) return 'presentation';
  if (mime.includes('json') || mime.includes('javascript') || mime.includes('text/')) return 'code';
  if (mime.includes('zip') || mime.includes('archive') || mime.includes('compressed')) return 'archive';
  if (ext && BINARY_EXTENSIONS.has(ext)) return 'binary';
  if (mime.includes('binary') || mime.includes('octet-stream')) return 'binary';

  return 'unknown';
}

export function DriveNodeIcon({ node, size = 36 }: { node: DriveNode; size?: number }) {
  const kind = resolveDriveFileKind(node);
  const theme = DRIVE_FILE_ICON_THEMES[kind] ?? DRIVE_FILE_ICON_THEMES.unknown;
  const iconSize = Math.max(12, Math.round(size * 0.62));
  const badgeSize = Math.max(6, Math.round(size * 0.25));
  const showBadge = size >= 24 && Boolean(theme.badge);

  return (
    <div
      style={{
        width: `${size}px`,
        height: `${size}px`,
        borderRadius: size >= 34 ? '10px' : size >= 28 ? '8px' : '6px',
        background: theme.background,
        border: `1px solid ${theme.border}`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        overflow: 'hidden',
        flexShrink: 0,
      }}
    >
      {theme.icon(iconSize, theme.color)}
      {showBadge && theme.badge && (
        <span
          style={{
            position: 'absolute',
            right: '4px',
            bottom: '3px',
            padding: '1px 4px',
            borderRadius: '999px',
            fontSize: `${badgeSize}px`,
            lineHeight: 1.1,
            fontWeight: 700,
            letterSpacing: '0.02em',
            color: theme.badgeColor || theme.color,
            background: `${theme.badgeBg || theme.background}CC`,
            border: `1px solid ${theme.border}`,
            textTransform: 'uppercase',
          }}
        >
          {theme.badge}
        </span>
      )}
    </div>
  );
}
