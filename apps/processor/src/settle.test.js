import { test } from "node:test";
import assert from "node:assert/strict";
import { settleAction, dlqPayload } from "./settle.js";

test("successful job is acked", () => {
  assert.equal(settleAction({ processed: true, finishReported: true }), "ack");
});

test("failed job with finish reported goes to DLQ then ack", () => {
  assert.equal(settleAction({ processed: false, finishReported: true }), "dlq-then-ack");
});

test("failed finish leaves the message pending", () => {
  assert.equal(settleAction({ processed: false, finishReported: false }), "leave-pending");
});

test("dlq payload keeps job id and error", () => {
  const raw = dlqPayload({ jobId: "j1", fileId: "f1" }, new Error("image too large"));
  const parsed = JSON.parse(raw);
  assert.equal(parsed.jobId, "j1");
  assert.equal(parsed.error, "image too large");
});
