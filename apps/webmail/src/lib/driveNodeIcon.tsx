import { DriveNode } from './api';
import { categorizeMimeType, FileIcon, SUPPORTED_FILE_TYPES } from '@untitledui/file-icons';

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
    if (SUPPORTED_FILE_TYPES.includes(alias)) return alias;
  }

  if (mime && MIME_FALLBACK_TYPES.has(mime)) return MIME_FALLBACK_TYPES.get(mime)!;

  const mimeCategory = categorizeMimeType(mime);
  if (SUPPORTED_FILE_TYPES.includes(mimeCategory)) return mimeCategory;

  if (mime?.startsWith('image/') && SUPPORTED_FILE_TYPES.includes('image')) return 'image';
  if (mime?.startsWith('video/') && SUPPORTED_FILE_TYPES.includes('video')) return 'video';
  if (mime?.startsWith('audio/') && SUPPORTED_FILE_TYPES.includes('audio')) return 'audio';

  return 'empty';
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
      <FileIcon
        type={type}
        size={size}
        variant={size >= 24 ? 'solid' : 'default'}
        theme="light"
      />
    </span>
  );
}
