# ── Vite dev server (hot reload) ──────────────────────────────────────────
FROM node:20-alpine AS dev

WORKDIR /app

# Install bun for faster installs
RUN npm install -g bun

COPY package.json bun.lockb* ./
RUN bun install --frozen-lockfile 2>/dev/null || bun install

COPY . .

EXPOSE 5173

CMD ["bun", "run", "dev", "--host"]

# ── Production build ────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

RUN npm install -g bun

COPY package.json bun.lockb* ./
RUN bun install --frozen-lockfile 2>/dev/null || bun install

COPY . .

RUN bun run build

# ── Production serve ────────────────────────────────────────────────────────
FROM nginx:alpine AS production

COPY --from=builder /app/dist /usr/share/nginx/html
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
