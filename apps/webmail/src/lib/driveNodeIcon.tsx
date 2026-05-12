import { DriveNode } from './api';
import { FileIcon } from '@untitledui/file-icons';
import { FolderIcon as FolderSolid } from '@heroicons/react/24/outline';

const MIME_FALLBACK_TYPES = new Map<string, string>([
  ['application/pdf', 'pdf'],
  ['application/x-pdf', 'pdf'],
  ['application/msword', 'doc'],
  ['application/vnd.ms-word.document.macroEnabled.12', 'doc'],
  ['application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'docx'],
  ['application/vnd.openxmlformats-officedocument.wordprocessingml.template', 'docx'],
  ['application/vnd.ms-excel', 'xls'],
  ['application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'xlsx'],
  ['application/vnd.openxmlformats-officedocument.spreadsheetml.template', 'xlsx'],
  ['application/vnd.ms-powerpoint', 'ppt'],
  ['application/vnd.openxmlformats-officedocument.presentationml.presentation', 'pptx'],
  ['text/csv', 'csv'],
  ['text/plain', 'txt'],
  ['text/xml', 'xml'],
  ['application/xml', 'xml'],
  ['application/json', 'json'],
  ['image/svg+xml', 'svg'],
  ['application/zip', 'zip'],
  ['application/x-rar-compressed', 'rar'],
  ['application/gzip', 'zip'],
  ['application/x-gzip', 'zip'],
  ['application/x-tar', 'zip'],
  ['application/x-7z-compressed', 'zip'],
  ['audio/mpeg', 'mp3'],
  ['audio/mp3', 'mp3'],
  ['video/mp4', 'mp4'],
  ['video/quicktime', 'mp4'],
]);

const EXTENSION_ALIASES = new Map<string, string>([
  ['jpeg', 'jpg'],
  ['htm', 'html'],
  ['ts', 'js'],
  ['tsx', 'js'],
  ['jsx', 'js'],
  ['m4a', 'mp3'],
  ['wav', 'audio'],
  ['m4v', 'mp4'],
  ['webm', 'mp4'],
  ['mov', 'video'],
  ['avi', 'video'],
  ['mkv', 'video'],
  ['svg', 'svg'],
  ['xlsm', 'xls'],
  ['xlsx', 'xlsx'],
  ['docm', 'docx'],
  ['md', 'txt'],
  ['markdown', 'txt'],
]);

const FILE_ICON_TYPES = new Set([
  'aep',
  'ai',
  'audio',
  'avi',
  'code',
  'css',
  'csv',
  'doc',
  'docx',
  'dmg',
  'document',
  'eps',
  'exe',
  'fig',
  'folder',
  'gif',
  'html',
  'image',
  'img',
  'indd',
  'java',
  'jpeg',
  'jpg',
  'js',
  'json',
  'mkv',
  'mp3',
  'mp4',
  'mpeg',
  'pdf',
  'pdf-simple',
  'png',
  'ppt',
  'pptx',
  'psd',
  'rar',
  'rss',
  'sql',
  'spreadsheets',
  'svg',
  'tiff',
  'txt',
  'video',
  'video-01',
  'video-02',
  'wav',
  'webp',
  'xls',
  'xlsx',
  'xml',
  'zip',
]);

function getNameExtension(name: string): string {
  const idx = name.lastIndexOf('.');
  if (idx <= 0 || idx === name.length - 1) return '';
  return name.slice(idx + 1).toLowerCase();
}

function resolveDriveFileType(node: DriveNode): string {
  if (node.node_type === 'folder') return 'folder';
  const mime = (node.mime_type ?? '').toLowerCase();
  const ext = getNameExtension(node.name);

  if (ext) {
    const alias = EXTENSION_ALIASES.get(ext) ?? ext;
    if (FILE_ICON_TYPES.has(alias)) return alias;
  }

  if (mime && MIME_FALLBACK_TYPES.has(mime)) return MIME_FALLBACK_TYPES.get(mime)!;

  if (mime?.startsWith('image/')) return 'image';
  if (mime?.startsWith('video/')) return 'video';
  if (mime?.startsWith('audio/')) return 'audio';
  if (mime.includes('pdf')) return 'pdf';
  if (mime.includes('word') || mime.includes('officedocument.wordprocessingml')) return 'doc';
  if (mime.includes('excel') || mime.includes('spreadsheet') || mime.includes('csv')) return 'xls';
  if (mime.includes('powerpoint') || mime.includes('presentation')) return 'ppt';
  if (mime.includes('json') || mime.includes('javascript') || mime.includes('text/')) return 'js';
  if (mime.includes('xml') || mime.includes('yaml') || mime.includes('toml')) return 'xml';
  if (mime.includes('zip') || mime.includes('archive') || mime.includes('compressed')) return 'zip';
  if (mime.includes('text')) return 'txt';
  if (mime.includes('binary') || mime.includes('octet-stream')) return 'exe';

  return 'empty';
}

function UnknownFileIcon({ size }: { size: number }) {
  return (
    <svg
      width={size}
      height={size}
      fill="none"
      viewBox="0 0 40 40"
      aria-hidden="true"
      focusable="false"
    >
      <path
        stroke="#D5D7DA"
        strokeWidth={1.5}
        d="M4.75 4A3.25 3.25 0 0 1 8 .75h16c.121 0 .238.048.323.134l10.793 10.793a.46.46 0 0 1 .134.323v24A3.25 3.25 0 0 1 32 39.25H8A3.25 3.25 0 0 1 4.75 36z"
      />
      <path stroke="#D5D7DA" strokeWidth={1.5} d="M24 .5V8a4 4 0 0 0 4 4h7.5" />
      <path
        stroke="#155EEF"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.5}
        d="M11.9 19.5h16.2m-16.2 3.6h16.2m-16.2 3.6h16.2m-16.2 3.6h12.6"
      />
    </svg>
  );
}

export function DriveNodeIcon({ node, size = 36 }: { node: DriveNode; size?: number }) {
  const type = resolveDriveFileType(node);
  return (
    <span
      style={{
        width: `${size}px`,
        height: `${size}px`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
      }}
    >
      {type === 'folder' ? (
        <FolderSolid style={{ width: `${size}px`, height: `${size}px`, color: '#f59e0b' }} />
      ) : type === 'empty' ? (
        <UnknownFileIcon size={size} />
      ) : (
        <FileIcon type={type} size={size} variant="default" theme="light" />
      )}
    </span>
  );
}
