import type { FastifyRequest, FastifyReply } from "fastify";
import type { UsersService } from "./users.service";

// ── Users Controller ──

export class UsersController {
  constructor(private service: UsersService) {}

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
