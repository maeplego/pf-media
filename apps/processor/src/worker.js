import { createClient } from "redis";
import * as Minio from "minio";
import { processImage } from "./process.js";
import { DLQ_STREAM, JOBS_STREAM, CLAIM_IDLE_MS, dlqPayload, entriesFromAutoClaim, settleAction } from "./settle.js";

const redisURL = process.env.MEDIA_REDIS_URL || "redis://redis:6379/0";
const apiBase = process.env.MEDIA_API_URL || "http://api:8090";
const token = process.env.MEDIA_PROCESSOR_TOKEN || "";
const group = "processor";
const consumer = process.env.HOSTNAME || "worker-1";

function s3Endpoint() {
  const raw =
    process.env.MEDIA_S3_ENDPOINT ||
    process.env.MEDIA_MINIO_ENDPOINT ||
    "garage:3900";
  const [host, port] = raw.split(":");
  return { host, port: Number(port || 3900) };
}

const { host, port } = s3Endpoint();
const s3 = new Minio.Client({
  endPoint: host,
  port,
  useSSL: process.env.MEDIA_S3_USE_SSL === "true" || process.env.MEDIA_MINIO_USE_SSL === "true",
  accessKey:
    process.env.MEDIA_S3_ACCESS_KEY ||
    process.env.MEDIA_MINIO_ACCESS_KEY ||
    "GKdevmedia00000001",
  secretKey:
    process.env.MEDIA_S3_SECRET_KEY ||
    process.env.MEDIA_MINIO_SECRET_KEY ||
    "dev-media-secret-key-for-local-compose-demo",
  region: process.env.MEDIA_S3_REGION || "garage",
  pathStyle: true,
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

async function deriveVariants(msg) {
  const { objectKey, bucket } = msg;
  const stream = await s3.getObject(bucket, objectKey);
  const chunks = [];
  for await (const c of stream) chunks.push(c);
  const out = await processImage(Buffer.concat(chunks));

  const variants = {};
  for (const [name, v] of Object.entries(out)) {
    const key = variantKey(objectKey, name);
    await s3.putObject(bucket, key, v.body, v.body.length, {
      "Content-Type": v.contentType,
    });
    variants[name] = { key, contentType: v.contentType };
  }
  return variants;
}

async function settleFailure(client, entryId, msg, error) {
  let finishReported = false;
  try {
    await finishJob(msg.jobId, {}, String(error?.message || error));
    finishReported = true;
  } catch (e) {
    console.error("finish failed", msg.jobId, e);
  }

  const action = settleAction({ processed: false, finishReported });
  if (action === "leave-pending") {
    return;
  }
  if (action === "dlq-then-ack") {
    await client.xAdd(DLQ_STREAM, "*", { payload: dlqPayload(msg, error) });
  }
  await client.xAck(JOBS_STREAM, group, entryId);
}

async function handle(client, entry) {
  const msg = JSON.parse(entry.message.payload);
  let variants;
  try {
    variants = await deriveVariants(msg);
  } catch (err) {
    console.error("job failed", msg.jobId, err);
    await settleFailure(client, entry.id, msg, err);
    return;
  }
  try {
    await finishJob(msg.jobId, variants, "");
    await client.xAck(JOBS_STREAM, group, entry.id);
    console.log("processed", msg.fileId, msg.jobId);
  } catch (err) {
    // 派生は書けたが API へ届いていない。ACK すると再実行できない。
    console.error("finish failed", msg.jobId, err);
  }
}

async function claimIdle(client) {
  try {
    if (typeof client.xAutoClaim !== "function") {
      return [];
    }
    const out = await client.xAutoClaim(JOBS_STREAM, group, consumer, CLAIM_IDLE_MS, "0-0", {
      COUNT: 5,
    });
    return entriesFromAutoClaim(out);
  } catch (e) {
    console.error("xautoclaim", e);
    return [];
  }
}

async function main() {
  const client = createClient({ url: redisURL });
  client.on("error", (e) => console.error("redis", e));
  await client.connect();

  try {
    await client.xGroupCreate(JOBS_STREAM, group, "0", { MKSTREAM: true });
  } catch (e) {
    if (!String(e.message).includes("BUSYGROUP")) throw e;
  }

  console.log("processor listening on", JOBS_STREAM);
  for (;;) {
    const stuck = await claimIdle(client);
    for (const entry of stuck) {
      await handle(client, entry);
    }
    const res = await client.xReadGroup(group, consumer, [{ key: JOBS_STREAM, id: ">" }], {
      COUNT: 1,
      BLOCK: 5000,
    });
    if (!res) continue;
    for (const stream of res) {
      for (const entry of stream.messages) {
        await handle(client, entry);
      }
    }
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
