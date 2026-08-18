import type { FastifyRequest, FastifyReply } from "fastify";
import type { UsersService } from "./users.service";
import { validate } from "../../common/pipes/validation.pipe";
import { updateProfileSchema, avatarUrlSchema } from "./dto/update-profile.dto";
import { changePasswordSchema } from "./dto/change-password.dto";
import { updateSettingsSchema } from "./dto/update-settings.dto";
import {
  createMcpServerSchema,
  updateMcpServerSchema,
} from "./dto/mcp-server.dto";
import {
  createUserSkillSchema,
  updateUserSkillSchema,
  toggleSkillSchema,
} from "./dto/user-skill.dto";

// ── Users Controller ──

export class UsersController {
  constructor(private service: UsersService) {}

  // ── User Self-Service Endpoints ──

  /** GET /api/user/profile */
  getProfile = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const user = await this.service.getUser(userId);
    return reply.status(200).send({ user });
  };

  /** PATCH /api/user/profile */
  updateProfile = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(updateProfileSchema, req.body);
    const user = await this.service.updateProfile(userId, input);
    return reply.status(200).send({ user });
  };

  /** POST /api/user/avatar — cập nhật ảnh đại diện sau khi upload lên MinIO xong. */
  uploadAvatar = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(avatarUrlSchema, req.body);
    const user = await this.service.updateProfile(userId, {
      avatar_url: input.avatar_url,
    });
    return reply.status(200).send({ user });
  };

  /** POST /api/user/change-password */
  changePassword = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(changePasswordSchema, req.body);
    const result = await this.service.changePassword(userId, input);
    return reply.status(200).send({ message: result.message });
  };

  /** GET /api/user/settings */
  getSettings = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const settings = await this.service.getSettings(userId);
    return reply.status(200).send({ settings });
  };

  /** PATCH /api/user/settings */
  updateSettings = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(updateSettingsSchema, req.body);
    const settings = await this.service.updateSettings(userId, input);
    return reply.status(200).send({ settings });
  };

  // ── MCP Servers ──

  /** GET /api/user/mcp-servers */
  listMcpServers = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const servers = await this.service.listMcpServers(userId);
    return reply.status(200).send({ servers });
  };

  /** POST /api/user/mcp-servers */
  createMcpServer = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(createMcpServerSchema, req.body);
    const server = await this.service.createMcpServer(userId, input);
    return reply.status(201).send({ server });
  };

  /** PATCH /api/user/mcp-servers/:id */
  updateMcpServer = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const { id } = req.params as { id: string };
    const input = validate(updateMcpServerSchema, req.body);
    const server = await this.service.updateMcpServer(userId, id, input);
    return reply.status(200).send({ server });
  };

  /** DELETE /api/user/mcp-servers/:id */
  deleteMcpServer = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const { id } = req.params as { id: string };
    await this.service.deleteMcpServer(userId, id);
    return reply.status(204).send();
  };

  // ── Skills ──

  /** GET /api/user/skills */
  listSkills = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const [customSkills, disabledSkills] = await Promise.all([
      this.service.listUserSkills(userId),
      this.service.listDisabledSkills(userId),
    ]);
    return reply
      .status(200)
      .send({ customSkills, disabledBuiltinSkills: disabledSkills });
  };

  /** POST /api/user/skills */
  createSkill = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const input = validate(createUserSkillSchema, req.body);
    const skill = await this.service.createUserSkill(userId, input);
    return reply.status(201).send({ skill });
  };

  /** PATCH /api/user/skills/:id */
  updateSkill = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const { id } = req.params as { id: string };
    const input = validate(updateUserSkillSchema, req.body);
    const skill = await this.service.updateUserSkill(userId, id, input);
    return reply.status(200).send({ skill });
  };

  /** DELETE /api/user/skills/:id */
  deleteSkill = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const { id } = req.params as { id: string };
    await this.service.deleteUserSkill(userId, id);
    return reply.status(204).send();
  };

  /** POST /api/user/skills/:name/toggle */
  toggleBuiltinSkill = async (req: FastifyRequest, reply: FastifyReply) => {
    const userId = req.user!.sub;
    const { name } = req.params as { name: string };
    const input = validate(toggleSkillSchema, req.body);
    await this.service.toggleBuiltinSkill(userId, name, input.enabled);
    return reply.status(200).send({ name, enabled: input.enabled });
  };

  // ── Admin Management Endpoints ──

  /** GET /api/admin/users */
  list = async (req: FastifyRequest, reply: FastifyReply) => {
    const { limit, offset } = req.query as {
      limit?: string;
      offset?: string;
    };
    const users = await this.service.listUsers(
      limit ? parseInt(limit, 10) : undefined,
      offset ? parseInt(offset, 10) : undefined,
    );
    return reply.status(200).send({ users });
  };

  /** GET /api/admin/users/:id */
  getOne = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const user = await this.service.getUser(id);
    return reply.status(200).send({ user });
  };

  /** PATCH /api/admin/users/:id/disable */
  disable = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const adminId = req.user!.sub;
    const user = await this.service.disableUser(adminId, id);
    return reply.status(200).send({ user });
  };

  /** PATCH /api/admin/users/:id/enable */
  enable = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const user = await this.service.enableUser(id);
    return reply.status(200).send({ user });
  };

  /** GET /api/admin/users/search */
  search = async (req: FastifyRequest, reply: FastifyReply) => {
    const { q } = req.query as { q?: string };
    if (!q) {
      return reply.status(200).send({ users: [] });
    }
    const users = await this.service.searchUsers(q);
    return reply.status(200).send({ users });
  };
}
