import { createClient } from "redis";
import * as Minio from "minio";
import { processImage } from "./process.js";

const redisURL = process.env.MEDIA_REDIS_URL || "redis://redis:6379/0";
const apiBase = process.env.MEDIA_API_URL || "http://api:8090";
const token = process.env.MEDIA_PROCESSOR_TOKEN || "";
const group = "processor";
const consumer = process.env.HOSTNAME || "worker-1";

const minio = new Minio.Client({
  endPoint: process.env.MEDIA_MINIO_ENDPOINT?.split(":")[0] || "minio",
  port: Number(process.env.MEDIA_MINIO_ENDPOINT?.split(":")[1] || 9000),
  useSSL: process.env.MEDIA_MINIO_USE_SSL === "true",
  accessKey: process.env.MEDIA_MINIO_ACCESS_KEY || "minio",
  secretKey: process.env.MEDIA_MINIO_SECRET_KEY || "minio12345",
});

async function finishJob(jobId, variants, error) {
  const res = await fetch(`${apiBase}/internal/v1/jobs/${jobId}/finish`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ variants, error }),
  });
  if (!res.ok) {
    throw new Error(`finish failed: ${res.status}`);
  }
}

function variantKey(objectKey, name) {
  const parts = objectKey.split("/");
  parts[parts.length - 1] = name;
  return parts.join("/");
}

async function handle(msg) {
  const { jobId, fileId, objectKey, bucket } = msg;
  try {
    const stream = await minio.getObject(bucket, objectKey);
    const chunks = [];
    for await (const c of stream) chunks.push(c);
    const buffer = Buffer.concat(chunks);
    const out = await processImage(buffer);

    const variants = {};
    for (const [name, v] of Object.entries(out)) {
      const key = variantKey(objectKey, name);
      await minio.putObject(bucket, key, v.body, v.body.length, {
        "Content-Type": v.contentType,
      });
      variants[name] = { key, contentType: v.contentType };
    }
    await finishJob(jobId, variants, "");
    console.log("processed", fileId, jobId);
  } catch (err) {
    console.error("job failed", jobId, err);
    await finishJob(jobId, {}, String(err.message || err));
  }
}

async function main() {
  const client = createClient({ url: redisURL });
  client.on("error", (e) => console.error("redis", e));
  await client.connect();

  try {
    await client.xGroupCreate("media:jobs", group, "0", { MKSTREAM: true });
  } catch (e) {
    if (!String(e.message).includes("BUSYGROUP")) throw e;
  }

  console.log("processor listening on media:jobs");
  for (;;) {
    const res = await client.xReadGroup(group, consumer, [{ key: "media:jobs", id: ">" }], {
      COUNT: 1,
      BLOCK: 5000,
    });
    if (!res) continue;
    for (const stream of res) {
      for (const entry of stream.messages) {
        const payload = entry.message.payload;
        const msg = JSON.parse(payload);
        await handle(msg);
        await client.xAck("media:jobs", group, entry.id);
      }
    }
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
