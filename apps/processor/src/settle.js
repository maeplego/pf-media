export const JOBS_STREAM = "media:jobs";
export const DLQ_STREAM = "media:jobs:dlq";
export const CLAIM_IDLE_MS = 60_000;

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

/** XAUTOCLAIM の戻りを xReadGroup と同じ {id, message} 形に揃える。 */
export function entriesFromAutoClaim(result) {
  if (!result) {
    return [];
  }
  if (Array.isArray(result.messages)) {
    return result.messages;
  }
  if (Array.isArray(result) && Array.isArray(result[1])) {
    return result[1].map((row) => {
      if (row && row.id && row.message) {
        return row;
      }
      const id = row[0];
      const fields = row[1] || [];
      const message = {};
      for (let i = 0; i + 1 < fields.length; i += 2) {
        message[fields[i]] = fields[i + 1];
      }
      return { id, message };
    });
  }
  return [];
}
