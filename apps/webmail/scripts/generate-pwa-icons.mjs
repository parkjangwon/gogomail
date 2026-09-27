// Generates PWA icons as PNGs with no external image dependencies.
//
// We rasterize the GoGoMail brand mark (an envelope glyph on the accent color)
// into an RGBA pixel buffer and encode it as a valid PNG using Node's built-in
// zlib. This keeps the repo dependency surface minimal (no sharp/canvas) while
// producing real raster icons that satisfy PWA installability requirements
// (>=192px and >=512px, plus a maskable variant with safe-zone padding).
//
// Run: node scripts/generate-pwa-icons.mjs

import { deflateSync } from 'node:zlib';
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const appRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const publicDir = resolve(appRoot, 'public');
const iconsDir = resolve(publicDir, 'icons');

const ACCENT = { r: 0x2f, g: 0x6e, b: 0xe0 };
const ACCENT_DARK = { r: 0x25, g: 0x60, b: 0xc8 };
const WHITE = { r: 0xff, g: 0xff, b: 0xff };

// ─── PNG encoding ─────────────────────────────────────────────────────────────

function crc32(buf) {
  let c = ~0;
  for (let i = 0; i < buf.length; i += 1) {
    c ^= buf[i];
    for (let k = 0; k < 8; k += 1) {
      c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
    }
  }
  return ~c >>> 0;
}

function chunk(type, data) {
  const typeBuf = Buffer.from(type, 'ascii');
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(data.length, 0);
  const crcBuf = Buffer.alloc(4);
  crcBuf.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])), 0);
  return Buffer.concat([lenBuf, typeBuf, data, crcBuf]);
}

function encodePNG(width, height, rgba) {
  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 6; // color type RGBA
  ihdr[10] = 0; // compression
  ihdr[11] = 0; // filter
  ihdr[12] = 0; // interlace

  // Prepend a zero filter byte to each scanline.
  const stride = width * 4;
  const raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y += 1) {
    raw[y * (stride + 1)] = 0;
    rgba.copy(raw, y * (stride + 1) + 1, y * stride, y * stride + stride);
  }
  const idat = deflateSync(raw, { level: 9 });

  return Buffer.concat([
    sig,
    chunk('IHDR', ihdr),
    chunk('IDAT', idat),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// ─── Drawing helpers ────────────────────────────────────────────────────────

function makeCanvas(size) {
  return { size, data: Buffer.alloc(size * size * 4) };
}

function setPixel(canvas, x, y, color, alpha = 255) {
  if (x < 0 || y < 0 || x >= canvas.size || y >= canvas.size) return;
  const i = (y * canvas.size + x) * 4;
  const a = alpha / 255;
  const dst = canvas.data;
  const inv = 1 - a;
  dst[i] = Math.round(color.r * a + dst[i] * inv);
  dst[i + 1] = Math.round(color.g * a + dst[i + 1] * inv);
  dst[i + 2] = Math.round(color.b * a + dst[i + 2] * inv);
  dst[i + 3] = Math.max(dst[i + 3], alpha);
}

function fillRoundedRect(canvas, x0, y0, x1, y1, radius, color) {
  for (let y = Math.floor(y0); y < Math.ceil(y1); y += 1) {
    for (let x = Math.floor(x0); x < Math.ceil(x1); x += 1) {
      // Rounded-corner test with a small anti-alias band.
      let inside = true;
      let alpha = 255;
      const corners = [
        [x0 + radius, y0 + radius],
        [x1 - radius, y0 + radius],
        [x0 + radius, y1 - radius],
        [x1 - radius, y1 - radius],
      ];
      const nearLeft = x < x0 + radius;
      const nearRight = x > x1 - radius;
      const nearTop = y < y0 + radius;
      const nearBottom = y > y1 - radius;
      if ((nearLeft || nearRight) && (nearTop || nearBottom)) {
        const cx = nearLeft ? corners[0][0] : corners[1][0];
        const cy = nearTop ? corners[0][1] : corners[2][1];
        const d = Math.hypot(x + 0.5 - cx, y + 0.5 - cy);
        if (d > radius + 0.5) inside = false;
        else if (d > radius - 0.5) alpha = Math.round((radius + 0.5 - d) * 255);
      }
      if (inside) setPixel(canvas, x, y, color, alpha);
    }
  }
}

// Draw a simple envelope glyph (rectangle body + flap lines) centered in a box.
function drawEnvelope(canvas, bx, by, bw, bh, color) {
  const stroke = Math.max(2, Math.round(bw * 0.06));
  const x1 = bx + bw;
  const y1 = by + bh;

  // Body outline.
  for (let t = 0; t < stroke; t += 1) {
    for (let x = bx; x <= x1; x += 1) {
      setPixel(canvas, x, by + t, color);
      setPixel(canvas, x, y1 - t, color);
    }
    for (let y = by; y <= y1; y += 1) {
      setPixel(canvas, bx + t, y, color);
      setPixel(canvas, x1 - t, y, color);
    }
  }

  // Flap: two diagonal lines from top corners to center-top.
  const cx = bx + bw / 2;
  const flapY = by + bh * 0.55;
  for (let x = bx; x <= cx; x += 1) {
    const y = by + ((flapY - by) * (x - bx)) / (cx - bx);
    for (let t = 0; t < stroke; t += 1) {
      setPixel(canvas, x, Math.round(y) + t, color);
      setPixel(canvas, Math.round(bx + x1 - x), Math.round(y) + t, color);
    }
  }
}

function renderIcon(size, { maskable }) {
  const canvas = makeCanvas(size);
  // Maskable icons need their content inside the ~80% safe zone; we keep the
  // background full-bleed and shrink the glyph so it survives circular masks.
  const bgInset = 0;
  const radius = maskable ? 0 : Math.round(size * 0.22);
  fillRoundedRect(canvas, bgInset, bgInset, size - bgInset, size - bgInset, radius, ACCENT);

  // Subtle darker band at the bottom for depth.
  fillRoundedRect(canvas, bgInset, size * 0.72, size - bgInset, size - bgInset, radius, ACCENT_DARK);

  const glyphScale = maskable ? 0.5 : 0.6;
  const bw = Math.round(size * glyphScale);
  const bh = Math.round(bw * 0.68);
  const bx = Math.round((size - bw) / 2);
  const by = Math.round((size - bh) / 2);
  drawEnvelope(canvas, bx, by, bw, bh, WHITE);

  return encodePNG(size, size, canvas.data);
}

// ─── Main ───────────────────────────────────────────────────────────────────

mkdirSync(iconsDir, { recursive: true });

const targets = [
  { name: 'icon-192.png', size: 192, maskable: false },
  { name: 'icon-512.png', size: 512, maskable: false },
  { name: 'icon-maskable-192.png', size: 192, maskable: true },
  { name: 'icon-maskable-512.png', size: 512, maskable: true },
  { name: 'apple-touch-icon.png', size: 180, maskable: false },
];

for (const t of targets) {
  const png = renderIcon(t.size, { maskable: t.maskable });
  writeFileSync(resolve(iconsDir, t.name), png);
  console.log(`wrote icons/${t.name} (${png.length} bytes)`);
}

// favicon.ico is referenced by the existing service worker (badge/icon) and by
// browsers by default; write a small 32px PNG copy as favicon.png and a 48px
// one. We keep favicon.ico out of scope (ICO container) and instead expose PNG
// favicons which modern browsers prefer.
for (const size of [32, 48]) {
  const png = renderIcon(size, { maskable: false });
  writeFileSync(resolve(publicDir, `favicon-${size}.png`), png);
  console.log(`wrote favicon-${size}.png (${png.length} bytes)`);
}

console.log('done');
