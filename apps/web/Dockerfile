# syntax=docker/dockerfile:1
# ── J.A.R.V.I.S. Frontend (React/Vite) ──
FROM node:22-alpine AS build
WORKDIR /app

RUN corepack enable && corepack prepare pnpm@latest --activate

COPY pnpm-lock.yaml pnpm-workspace.yaml package.json tsconfig.base.json ./
COPY apps/web/package.json apps/web/tsconfig.json apps/web/tsconfig.node.json apps/web/vite.config.ts apps/web/tailwind.config.js ./apps/web/
COPY apps/web/index.html ./apps/web/

RUN pnpm install --frozen-lockfile --filter @app/web

COPY apps/web/src ./apps/web/src

RUN pnpm --filter @app/web build

# ── Runtime (nginx) ──
FROM nginx:1.27-alpine
COPY --from=build /app/apps/web/dist /usr/share/nginx/html
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
