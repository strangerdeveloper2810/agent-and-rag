import type { FastifyRequest, FastifyReply } from "fastify";
import type { UsersService } from "./users.service";
import { validate } from "../../common/pipes/validation.pipe";
import { updateProfileSchema } from "./dto/update-profile.dto";
import { changePasswordSchema } from "./dto/change-password.dto";
import { updateSettingsSchema } from "./dto/update-settings.dto";

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
