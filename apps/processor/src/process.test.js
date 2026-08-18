import { test } from "node:test";
import assert from "node:assert/strict";
import sharp from "sharp";
import { processImage, assertWithinPixelLimit, MAX_PIXELS } from "./process.js";

async function tinyJpeg() {
  return sharp({
    create: { width: 64, height: 48, channels: 3, background: { r: 200, g: 40, b: 40 } },
  })
    .jpeg()
    .toBuffer();
}

test("emits webp detail and thumb without copying exif", async () => {
  const out = await processImage(await tinyJpeg());
  assert.equal(out.detail.contentType, "image/webp");
  assert.equal(out.thumb.contentType, "image/webp");

  const origMeta = await sharp(out.orig.body).metadata();
  assert.equal(origMeta.exif, undefined);
  const detailMeta = await sharp(out.detail.body).metadata();
  assert.equal(detailMeta.format, "webp");
  assert.ok((detailMeta.width || 0) <= 1920);
  const thumbMeta = await sharp(out.thumb.body).metadata();
  assert.ok((thumbMeta.width || 0) <= 320);
});

test("rejects pixel counts above the decompression-bomb limit", () => {
  assert.doesNotThrow(() => assertWithinPixelLimit(4000, 4000));
  assert.throws(() => assertWithinPixelLimit(4001, 4000), /image too large/);
  const side = Math.floor(Math.sqrt(MAX_PIXELS)) + 1;
  assert.throws(() => assertWithinPixelLimit(side, side), /image too large/);
});

test("rejects non-image magic bytes", async () => {
  const fake = Buffer.concat([Buffer.from("not-an-image"), Buffer.alloc(16)]);
  await assert.rejects(() => processImage(fake), /unsupported image type/);
});
