import type { FastifyInstance } from "fastify";
import type { Pool } from "pg";
import { UsersRepository } from "./users.repository";
import { UsersService } from "./users.service";
import { UsersController } from "./users.controller";
import { authGuard } from "../../common/guards/auth.guard";
import { adminGuard } from "../../common/guards/admin.guard";

// ── Module Options ──

export interface UsersModuleOptions {
  pgPool: Pool;
}

/**
 * Users admin module — Fastify plugin đóng gói toàn bộ route quản lý user.
 *
 * Tất cả route đều yêu cầu auth + quyền admin (authGuard → adminGuard).
 *
 * Register trong app.ts:
 *   app.register(usersModule, { pgPool: getPgPool() });
 */
export const usersModule = async (
  app: FastifyInstance,
  opts: UsersModuleOptions,
): Promise<void> => {
  // ── Wire dependencies ──
  const repo = new UsersRepository(opts.pgPool);
  const service = new UsersService(repo);
  const controller = new UsersController(service);

  const guard = [authGuard, adminGuard];

  // ── User Self-Service routes (Yêu cầu đăng nhập) ──
  app.get(
    "/api/user/profile",
    { preHandler: [authGuard] },
    controller.getProfile,
  );
  app.patch(
    "/api/user/profile",
    { preHandler: [authGuard] },
    controller.updateProfile,
  );
  app.post(
    "/api/user/avatar",
    { preHandler: [authGuard] },
    controller.uploadAvatar,
  );
  app.post(
    "/api/user/change-password",
    { preHandler: [authGuard] },
    controller.changePassword,
  );
  app.get(
    "/api/user/settings",
    { preHandler: [authGuard] },
    controller.getSettings,
  );
  app.patch(
    "/api/user/settings",
    { preHandler: [authGuard] },
    controller.updateSettings,
  );

  // ── MCP Servers routes ──
  app.get(
    "/api/user/mcp-servers",
    { preHandler: [authGuard] },
    controller.listMcpServers,
  );
  app.post(
    "/api/user/mcp-servers",
    { preHandler: [authGuard] },
    controller.createMcpServer,
  );
  app.patch(
    "/api/user/mcp-servers/:id",
    { preHandler: [authGuard] },
    controller.updateMcpServer,
  );
  app.delete(
    "/api/user/mcp-servers/:id",
    { preHandler: [authGuard] },
    controller.deleteMcpServer,
  );
  app.post(
    "/api/user/mcp-servers/:id/test-connection",
    { preHandler: [authGuard] },
    controller.testMcpServer,
  );

  // ── Skills routes ──
  app.get(
    "/api/user/skills",
    { preHandler: [authGuard] },
    controller.listSkills,
  );
  app.post(
    "/api/user/skills",
    { preHandler: [authGuard] },
    controller.createSkill,
  );
  app.patch(
    "/api/user/skills/:id",
    { preHandler: [authGuard] },
    controller.updateSkill,
  );
  app.delete(
    "/api/user/skills/:id",
    { preHandler: [authGuard] },
    controller.deleteSkill,
  );
  app.post(
    "/api/user/skills/:name/toggle",
    { preHandler: [authGuard] },
    controller.toggleBuiltinSkill,
  );

  // ⚠️ Route /search phải được register TRƯỚC route /:id,
  // nếu không Fastify sẽ match "search" như 1 param :id.

  // ── Admin routes ──
  app.get("/api/admin/users/search", { preHandler: guard }, controller.search);
  app.get("/api/admin/users", { preHandler: guard }, controller.list);
  app.get("/api/admin/users/:id", { preHandler: guard }, controller.getOne);
  app.patch(
    "/api/admin/users/:id/disable",
    { preHandler: guard },
    controller.disable,
  );
  app.patch(
    "/api/admin/users/:id/enable",
    { preHandler: guard },
    controller.enable,
  );
};
