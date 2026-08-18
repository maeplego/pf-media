export const JOBS_STREAM = "media:jobs";
export const DLQ_STREAM = "media:jobs:dlq";

/**
 * 処理結果に対して Redis 上で何をするか。
 * - ack: 本線から消してよい（成功）
 * - dlq-then-ack: 失敗を隔離してから本線 ACK（毒メッセージで無限リトライしない）
 * - leave-pending: finish 自体が失敗。ACK するとジョブが消えるので残す
 */
export function settleAction({ processed, finishReported }) {
  if (processed) {
    return "ack";
  }
  if (finishReported) {
    return "dlq-then-ack";
  }
  return "leave-pending";
}

export function dlqPayload(msg, error) {
  return JSON.stringify({ ...msg, error: String(error?.message || error || "unknown") });
}
