import { test } from "node:test";
import assert from "node:assert/strict";
import { sniffImageMime } from "./magic.js";

test("sniffs jpeg png gif webp", () => {
  const jpeg = Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0]);
  assert.equal(sniffImageMime(jpeg), "image/jpeg");

  const png = Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    Buffer.alloc(4),
  ]);
  assert.equal(sniffImageMime(png), "image/png");

  const gif = Buffer.concat([Buffer.from("GIF89a"), Buffer.alloc(6)]);
  assert.equal(sniffImageMime(gif), "image/gif");

  const webp = Buffer.concat([Buffer.from("RIFF"), Buffer.alloc(4), Buffer.from("WEBP")]);
  assert.equal(sniffImageMime(webp), "image/webp");
});

test("rejects html pretending to be an image", () => {
  const html = Buffer.from("<!doctype html>....");
  assert.throws(() => sniffImageMime(html), /unsupported image type/);
});

test("rejects truncated buffers", () => {
  assert.throws(() => sniffImageMime(Buffer.from([0xff, 0xd8])), /file too small/);
});
