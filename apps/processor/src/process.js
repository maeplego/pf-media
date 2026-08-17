const MAX_PIXELS = 4000 * 4000;
const MAX_BYTES = 20 * 1024 * 1024;

export async function processImage(buffer) {
  const sharp = (await import("sharp")).default;
  const meta = await sharp(buffer).metadata();
  const pixels = (meta.width || 0) * (meta.height || 0);
  if (pixels > MAX_PIXELS) {
    throw new Error("image too large");
  }
  if (buffer.length > MAX_BYTES) {
    throw new Error("file too large");
  }

  const orig = await sharp(buffer).rotate().withMetadata({ exif: {} }).toBuffer();
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
