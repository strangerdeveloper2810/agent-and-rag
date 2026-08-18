# syntax=docker/dockerfile:1
# ── J.A.R.V.I.S. API Gateway (Fastify/TS) ──
FROM node:22-alpine AS build
WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy workspace config & packages
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json tsconfig.base.json ./
COPY packages ./packages
COPY apps/api ./apps/api

# Install deps
RUN pnpm install --frozen-lockfile --filter @app/api...

# Build
RUN pnpm --filter @app/api build
RUN pnpm --filter @app/api --prod deploy --legacy /app/deploy

# ── Runtime ──
FROM node:22-alpine
WORKDIR /app

COPY --from=build /app/deploy/package.json ./
COPY --from=build /app/deploy/node_modules ./node_modules
COPY --from=build /app/apps/api/src ./src
COPY --from=build /app/tsconfig.base.json ./
COPY --from=build /app/packages ./packages

EXPOSE 3001
ENV NODE_ENV=production
CMD ["./node_modules/.bin/tsx", "src/server.ts"]
