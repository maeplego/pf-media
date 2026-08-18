// 拡張子や Content-Type はクライアントが嘘を付けるので、先頭バイトで実形式を見る。

const PNG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

export function sniffImageMime(buf) {
  if (!buf || buf.length < 12) {
    throw new Error("file too small");
  }
  if (buf[0] === 0xff && buf[1] === 0xd8 && buf[2] === 0xff) {
    return "image/jpeg";
  }
  if (buf.subarray(0, 8).equals(PNG)) {
    return "image/png";
  }
  const gif = buf.subarray(0, 6).toString("ascii");
  if (gif === "GIF87a" || gif === "GIF89a") {
    return "image/gif";
  }
  if (buf.subarray(0, 4).toString("ascii") === "RIFF" && buf.subarray(8, 12).toString("ascii") === "WEBP") {
    return "image/webp";
  }
  throw new Error("unsupported image type");
}
