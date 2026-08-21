import type { FastifyReply, FastifyRequest } from "fastify";
import * as chatService from "../services";
import { parseOrBadRequest } from "../../../lib/validate";
import {
  chatBodySchema,
  createConversationBodySchema,
} from "../../../schemas/chat-request";
import { getPgPool } from "../../../database/postgres/postgres.module";
import { getSuggestions as fetchSuggestions } from "../../../agent/client/go-agent.client";

// Cùng pattern với modules/documents/controllers, modules/upload/upload.routes.ts
// — authGuard đã set req.tenantId từ JWT trước khi route chạm tới controller;
// fallback "default" chỉ để phòng hờ, không nên xảy ra trong thực tế vì mọi
// route chat đều có authGuard (xem chat.routes.ts).
const getTenantId = (req: FastifyRequest): string =>
  ((req as unknown as Record<string, unknown>).tenantId as string) ?? "default";

export const postConversation = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const body = parseOrBadRequest(createConversationBodySchema, req.body);
  return chatService.createConversation(tenantId, body.firstMessage ?? "");
};

export const getConversations = async (req: FastifyRequest) =>
  chatService.listConversations(getTenantId(req));

export const getConversationMessages = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { id } = req.params as { id: string };
  return chatService.getConversationMessages(tenantId, id);
};

export const deleteConversation = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { id } = req.params as { id: string };
  return chatService.deleteConversation(tenantId, id);
};

/**
 * Gợi ý mở đầu hội thoại (GET /api/suggestions) — proxy sang Go agent kèm
 * đúng X-Tenant-ID để agent-go cá nhân hoá theo tenant thật, thay vì FE gọi
 * thẳng agent-go không qua authGuard (mọi user rơi về tenant "default").
 */
export const getSuggestions = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  return fetchSuggestions(tenantId);
};

/**
 * Chat qua SSE: stream agent events về client.
 *
 * Flow:
 * 1. Validate + lưu user message vào DB (lỗi sớm, vẫn trả HTTP JSON).
 * 2. Hijack reply, mở SSE stream.
 * 3. Chạy agent, forward từng event -> SSE `data: {...}\n\n`.
 * 4. Khi stream xong -> gửi event `done` kèm metadata (agent name, tokens).
 * 5. Nếu lỗi -> gửi event `error`.
 */
export const postChat = async (req: FastifyRequest, reply: FastifyReply) => {
  const { id } = req.params as { id: string };
  const tenantId = getTenantId(req);
  const { content, attachments, lang } = parseOrBadRequest(
    chatBodySchema,
    req.body,
  );

  // Lưu user msg TRƯỚC khi mở SSE -> lỗi sớm (validate/DB) vẫn trả HTTP JSON
  // qua error handler (chưa "chiếm" reply nên còn gửi JSON được).
  await chatService.appendUserMessage(tenantId, id, content, attachments);

  // hijack(): ta tự ghi thẳng vào socket (SSE); Fastify không serialize/gửi nữa.
  reply.hijack();

  // Client đóng kết nối (đóng tab, đổi trang) -> abort agent để không đốt token.
  const ac = new AbortController();
  req.raw.on("close", () => ac.abort());

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  const write = (payload: unknown) => {
    if (!reply.raw.writableEnded) {
      reply.raw.write(`data: ${JSON.stringify(payload)}\n\n`);
    }
  };

  try {
    let personaSettings:
      | {
          personaPreset?: string;
          formality?: string;
          verbosity?: string;
          customInstructions?: string;
        }
      | undefined;

    const userId = (req.user as { sub?: string } | undefined)?.sub;

    let mcpServers:
      | Array<{
          name: string;
          url: string;
          apiKey?: string;
          transport?: "http" | "sse";
        }>
      | undefined;
    let disabledSkills: string[] | undefined;
    let customSkills:
      | Array<{
          name: string;
          description?: string;
          whenToUse?: string;
          content?: string;
          triggers?: string[];
        }>
      | undefined;

    if (userId) {
      try {
        const pg = getPgPool();
        const [settingsRes, mcpRes, disabledRes, skillsRes] = await Promise.all(
          [
            pg.query(
              "SELECT persona_preset, formality, verbosity, custom_instructions FROM user_settings WHERE user_id = $1",
              [userId],
            ),
            pg.query(
              "SELECT name, url, transport, auth_token FROM user_mcp_servers WHERE user_id = $1 AND enabled = TRUE ORDER BY created_at ASC",
              [userId],
            ),
            pg.query(
              "SELECT skill_name FROM user_disabled_skills WHERE user_id = $1",
              [userId],
            ),
            pg.query(
              "SELECT name, description, when_to_use, content, triggers FROM user_skills WHERE user_id = $1 AND enabled = TRUE ORDER BY created_at ASC",
              [userId],
            ),
          ],
        );

        const settingsRow = settingsRes.rows[0];
        if (settingsRow) {
          personaSettings = {
            personaPreset: settingsRow.persona_preset,
            formality: settingsRow.formality,
            verbosity: settingsRow.verbosity,
            customInstructions: settingsRow.custom_instructions,
          };
        }

        // apiKey: field JSON nội bộ gửi xuống agent-go (giữ tên cũ để khớp
        // Go McpServerInput.APIKey/json:"apiKey" -- xem
        // services/agent-go/internal/transport/http/chat.go), nguồn dữ liệu
        // giờ là cột auth_token (thay api_key cũ -- xem migration 004). Đây
        // KHÔNG phải response công khai cho FE nên forward giá trị thật của
        // token là đúng ý (agent-go cần nó để build header Authorization).
        mcpServers = mcpRes.rows.map((r) => ({
          name: r.name,
          url: r.url,
          apiKey: r.auth_token ?? undefined,
          transport: r.transport as "http" | "sse" | undefined,
        }));
        disabledSkills = disabledRes.rows.map((r) => r.skill_name);
        customSkills = skillsRes.rows.map((r) => ({
          name: r.name,
          description: r.description ?? "",
          whenToUse: r.when_to_use ?? "",
          content: r.content ?? "",
          triggers: r.triggers ?? [],
        }));
      } catch {
        // Nếu query lỗi hoặc user chưa có settings, bỏ qua
      }
    }

    const { events, metadata } = await chatService.streamReply(
      id,
      ac.signal,
      undefined, // use default agent
      attachments,
      tenantId,
      lang,
      personaSettings,
      mcpServers,
      disabledSkills,
      customSkills,
    );

    for await (const ev of events) {
      if (ev.type === "text") {
        write({ token: ev.text });
      } else if (ev.type === "done") {
        // Go agent gửi event done kèm usage, nhưng ta chỉ dùng để cập nhật
        // metadata bên trong streamReply. Controller tự gửi event done cuối
        // cùng với agent+token info — không forward event done của agent ra SSE.
      } else {
        write(ev);
      }
    }

    // Sau khi stream hoàn tất -> gửi metadata (agent name, tokens used).
    const meta = await metadata;
    write({
      done: true,
      agent: meta.backend,
      tokens: meta.tokensUsed,
      truncated: meta.truncated,
      // contextTokens/contextBudget: FE dùng để gợi ý bắt đầu chat mới khi
      // context lớn (Tier 4) — trước fix, controller tự dựng payload done
      // riêng (không forward event done gốc từ agent) nên 2 field này bị rơi
      // mất dù go-agent.client/chat.service đã forward đúng.
      contextTokens: meta.contextTokens,
      contextBudget: meta.contextBudget,
    });
  } catch (err) {
    // Client chủ động ngắt (AbortError) là bình thường — không phải lỗi.
    if (!ac.signal.aborted) {
      req.log.error(err);
      write({ type: "error", message: "Đã xảy ra lỗi khi tạo câu trả lời." });
    }
  }

  if (!reply.raw.writableEnded) reply.raw.end();
};

/**
 * Tiếp tục 1 câu trả lời assistant bị cắt vì chạm giới hạn output token, qua
 * SSE giống postChat — nhưng KHÔNG nhận content/attachments (không phải 1
 * user turn mới) và response được NỐI vào cuối message cũ (xem
 * chatService.continueReply) thay vì tạo message mới, để lịch sử liền mạch
 * cả sau khi F5 lại trang.
 */
export const postContinue = async (
  req: FastifyRequest,
  reply: FastifyReply,
) => {
  const { id } = req.params as { id: string };
  const tenantId = getTenantId(req);

  const ac = new AbortController();

  // continueReply validate sớm (throw BadRequestError nếu không có assistant
  // message để tiếp tục) TRƯỚC khi hijack -> lỗi này vẫn trả HTTP JSON bình
  // thường qua error handler chung, giống appendUserMessage ở postChat.
  const { events, metadata } = await chatService.continueReply(
    id,
    ac.signal,
    undefined,
    tenantId,
  );

  reply.hijack();
  req.raw.on("close", () => ac.abort());

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  const write = (payload: unknown) => {
    if (!reply.raw.writableEnded) {
      reply.raw.write(`data: ${JSON.stringify(payload)}\n\n`);
    }
  };

  try {
    for await (const ev of events) {
      if (ev.type === "text") {
        write({ token: ev.text });
      } else if (ev.type === "done") {
        // Controller tự gửi event done cuối cùng, giống postChat.
      } else {
        write(ev);
      }
    }

    const meta = await metadata;
    write({
      done: true,
      agent: meta.backend,
      tokens: meta.tokensUsed,
      truncated: meta.truncated,
      contextTokens: meta.contextTokens,
      contextBudget: meta.contextBudget,
    });
  } catch (err) {
    if (!ac.signal.aborted) {
      req.log.error(err);
      write({
        type: "error",
        message: "Đã xảy ra lỗi khi tiếp tục câu trả lời.",
      });
    }
  }

  if (!reply.raw.writableEnded) reply.raw.end();
};
