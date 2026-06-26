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

/**
 * Embed danh sách text thành vector qua Voyage AI.
 * - inputType "document": khi nạp tài liệu (lúc upload)
 * - inputType "query": khi embed câu hỏi để search
 * Voyage tối ưu vector khác nhau cho 2 loại này.
 */
export async function embed(
  texts: string[],
  inputType: "document" | "query",
): Promise<number[][]> {
  const res = await fetch(VOYAGE_URL, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      authorization: `Bearer ${config.VOYAGE_API_KEY}`,
    },
    body: JSON.stringify(buildEmbeddingRequest(texts, inputType)),
  });
  if (!res.ok) {
    throw new Error(`Voyage error: ${res.status} ${await res.text()}`);
  }
  const data = (await res.json()) as { data: { embedding: number[] }[] };
  return data.data.map((d) => d.embedding);
}
