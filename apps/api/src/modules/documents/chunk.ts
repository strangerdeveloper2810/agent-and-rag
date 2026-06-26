import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

// chunkSize ~800: đủ giữ ngữ cảnh, không quá to gây nhiễu/tốn tiền
// chunkOverlap ~100: để câu vắt ngang ranh giới không bị cắt mất nghĩa
const splitter = new RecursiveCharacterTextSplitter({
  chunkSize: 800,
  chunkOverlap: 100,
});

export async function chunkText(text: string): Promise<string[]> {
  return splitter.splitText(text);
}
