import { config } from "../config";

const VOYAGE_URL = "https://api.voyageai.com/v1/embeddings";
const MODEL = "voyage-3"; // 1024 chiều — phải khớp numDimensions của Atlas Vector Index

// Tách phần dựng request body ra để test thuần (không gọi mạng)
export function buildEmbeddingRequest(
  texts: string[],
  inputType: "document" | "query",
) {
  return { input: texts, model: MODEL, input_type: inputType };
}

// Lỗi mang theo HTTP status để route biết cách phản hồi (vd 429 → báo rate limit)
export class VoyageError extends Error {
  status: number;
  constructor(status: number, detail: string) {
    super(`Voyage error: ${status} ${detail}`);
    this.name = "VoyageError";
    this.status = status;
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/**
 * Embed danh sách text thành vector qua Voyage AI.
 * - inputType "document": khi nạp tài liệu (lúc upload)
 * - inputType "query": khi embed câu hỏi để search
 * Voyage tối ưu vector khác nhau cho 2 loại này.
 *
 * Tự retry với backoff khi gặp 429 (rate limit) — free tier rất thấp
 * (3 RPM / 10K TPM) nên dễ dính khi test. Sau vài lần vẫn lỗi thì ném VoyageError.
 */
export async function embed(
  texts: string[],
  inputType: "document" | "query",
): Promise<number[][]> {
  const MAX_RETRIES = 2;
  for (let attempt = 0; ; attempt++) {
    const res = await fetch(VOYAGE_URL, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${config.VOYAGE_API_KEY}`,
      },
      body: JSON.stringify(buildEmbeddingRequest(texts, inputType)),
    });

    if (res.ok) {
      const data = (await res.json()) as { data: { embedding: number[] }[] };
      return data.data.map((d) => d.embedding);
    }

    const detail = await res.text();
    // 429 = vượt rate limit → đợi cho cửa sổ /phút reset rồi thử lại
    if (res.status === 429 && attempt < MAX_RETRIES) {
      await sleep((attempt + 1) * 10_000); // 10s, 20s
      continue;
    }
    throw new VoyageError(res.status, detail);
  }
}
