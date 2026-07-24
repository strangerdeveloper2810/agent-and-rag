# syntax=docker/dockerfile:1
# ── J.A.R.V.I.S. API Gateway (Fastify/TS) ──
FROM node:22-alpine AS build
WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy workspace config
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json tsconfig.base.json ./
COPY apps/api/package.json apps/api/tsconfig.json ./apps/api/

# Install deps
RUN pnpm install --frozen-lockfile --filter @app/api

# Copy source
COPY apps/api ./apps/api

# Build
RUN pnpm --filter @app/api build

# ── Runtime ──
FROM node:22-alpine
WORKDIR /app

RUN corepack enable && corepack prepare pnpm@latest --activate

COPY --from=build /app/apps/api/dist ./dist
COPY --from=build /app/apps/api/package.json ./
COPY --from=build /app/apps/api/node_modules ./node_modules

EXPOSE 3001
ENV NODE_ENV=production
CMD ["node", "dist/server.js"]
