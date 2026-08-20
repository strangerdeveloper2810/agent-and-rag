import { connectMongo, closeMongo, COLLECTIONS } from "../lib/mongo";

/**
 * Migration một lần: gán tenantId cho các conversation/message CŨ (tạo ra
 * trước khi cách ly tenant được thêm vào chat module — xem chat.repository.ts).
 * Các document này KHÔNG thể xác định đúng chủ sở hữu thật (bug lịch sử không
 * hề ghi tenantId lúc tạo), nên gán về 1 giá trị cố định thay vì xoá — giữ lại
 * dữ liệu phòng khi cần tra cứu/điều tra sau, nhưng KHÔNG dùng UUID thật của
 * bất kỳ user nào (tenantId thật là UUID từ bảng `users`, xem
 * auth-context.ts/JwtPayload.sub) nên không tenant thật nào có thể query trúng
 * (listConversations/getMessages luôn lọc theo tenantId của chính request).
 *
 * Chạy: pnpm migrate:legacy-tenant
 */
const LEGACY_TENANT_ID = "legacy-unassigned";

async function main() {
  const db = await connectMongo();
  const conversations = db.collection(COLLECTIONS.conversations);
  const messages = db.collection(COLLECTIONS.messages);

  const beforeConvos = await conversations.countDocuments({
    tenantId: { $exists: false },
  });
  const beforeMsgs = await messages.countDocuments({
    tenantId: { $exists: false },
  });

  if (beforeConvos === 0 && beforeMsgs === 0) {
    console.log("Không có conversation/message nào thiếu tenantId. ✓");
    return;
  }

  const convoRes = await conversations.updateMany(
    { tenantId: { $exists: false } },
    { $set: { tenantId: LEGACY_TENANT_ID } },
  );
  const msgRes = await messages.updateMany(
    { tenantId: { $exists: false } },
    { $set: { tenantId: LEGACY_TENANT_ID } },
  );

  console.log(
    `conversations: ${beforeConvos} thiếu tenantId → đã set ${convoRes.modifiedCount} document (tenantId="${LEGACY_TENANT_ID}")`,
  );
  console.log(
    `messages: ${beforeMsgs} thiếu tenantId → đã set ${msgRes.modifiedCount} document (tenantId="${LEGACY_TENANT_ID}")`,
  );

  const afterConvos = await conversations.countDocuments({
    tenantId: { $exists: false },
  });
  const afterMsgs = await messages.countDocuments({
    tenantId: { $exists: false },
  });
  console.log(
    `Còn lại thiếu tenantId — conversations: ${afterConvos}, messages: ${afterMsgs} (kỳ vọng 0).`,
  );
}

main()
  .catch((err) => {
    console.error("Migration lỗi:", err);
    process.exitCode = 1;
  })
  .finally(() => closeMongo());
