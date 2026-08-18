import { sniffImageMime } from "./magic.js";

export const MAX_PIXELS = 4000 * 4000;
export const MAX_BYTES = 20 * 1024 * 1024;

export function assertWithinPixelLimit(width, height) {
  const pixels = (width || 0) * (height || 0);
  if (pixels > MAX_PIXELS) {
    throw new Error("image too large");
  }
}

export async function processImage(buffer) {
  if (!buffer || buffer.length > MAX_BYTES) {
    throw new Error("file too large");
  }
  sniffImageMime(buffer);

  const sharp = (await import("sharp")).default;
  const meta = await sharp(buffer).metadata();
  assertWithinPixelLimit(meta.width, meta.height);

  // rotate() で Orientation を画素に焼き、メタデータはコピーしない（GPS 等を残さない）。
  const orig = await sharp(buffer).rotate().toBuffer();
  const detail = await sharp(orig)
    .resize({ width: 1920, height: 1920, fit: "inside", withoutEnlargement: true })
    .webp({ quality: 82 })
    .toBuffer();
  const thumb = await sharp(orig)
    .resize({ width: 320, height: 320, fit: "inside", withoutEnlargement: true })
    .webp({ quality: 80 })
    .toBuffer();

  return {
    orig: { body: orig, contentType: meta.format === "jpeg" ? "image/jpeg" : `image/${meta.format}` },
    detail: { body: detail, contentType: "image/webp" },
    thumb: { body: thumb, contentType: "image/webp" },
  };
}
