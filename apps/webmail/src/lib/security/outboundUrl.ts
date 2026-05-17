import { lookup } from 'node:dns/promises';
import { isIP } from 'node:net';

const PRIVATE_IPV4_RANGES: Array<[number, number]> = [
  [ip4ToInt('0.0.0.0'), ip4ToInt('0.255.255.255')],
  [ip4ToInt('10.0.0.0'), ip4ToInt('10.255.255.255')],
  [ip4ToInt('127.0.0.0'), ip4ToInt('127.255.255.255')],
  [ip4ToInt('169.254.0.0'), ip4ToInt('169.254.255.255')],
  [ip4ToInt('172.16.0.0'), ip4ToInt('172.31.255.255')],
  [ip4ToInt('192.168.0.0'), ip4ToInt('192.168.255.255')],
];

function ip4ToInt(value: string): number {
  return value.split('.').reduce((acc, part) => ((acc << 8) + Number(part)) >>> 0, 0);
}

function isPrivateIPv4(value: string): boolean {
  const int = ip4ToInt(value);
  return PRIVATE_IPV4_RANGES.some(([start, end]) => int >= start && int <= end);
}

function isPrivateIPv6(value: string): boolean {
  const normalized = value.toLowerCase();
  return normalized === '::' ||
    normalized === '::1' ||
    normalized.startsWith('fc') ||
    normalized.startsWith('fd') ||
    normalized.startsWith('fe80:') ||
    normalized.startsWith('ff');
}

export function isPrivateAddress(value: string): boolean {
  if (value === '169.254.169.254') return true;
  const family = isIP(value);
  if (family === 4) return isPrivateIPv4(value);
  if (family === 6) return isPrivateIPv6(value);
  return false;
}

export async function validateOutboundHttpUrl(rawUrl: string): Promise<URL> {
  const parsed = new URL(rawUrl);
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('Only http/https allowed');
  }
  const hostname = parsed.hostname.replace(/\.$/, '');
  if (!hostname || hostname.toLowerCase() === 'localhost') {
    throw new Error('Private addresses not allowed');
  }
  if (isPrivateAddress(hostname)) {
    throw new Error('Private addresses not allowed');
  }
  const records = await lookup(hostname, { all: true, verbatim: true });
  if (records.length === 0 || records.some((record) => isPrivateAddress(record.address))) {
    throw new Error('Private addresses not allowed');
  }
  return parsed;
}

export function assertImageContentType(contentType: string): void {
  if (!/^image\/(jpeg|png|gif|webp|bmp|tiff|x-icon|avif)(?:\s*;|$)/i.test(contentType)) {
    throw new Error('Not an allowed image type');
  }
}
