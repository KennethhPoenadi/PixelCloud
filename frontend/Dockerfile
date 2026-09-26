# Build context is the repository root (needs deploy/nginx as well).
FROM node:22-alpine AS build
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM nginx:1.29-alpine
COPY deploy/nginx/templates /etc/nginx/templates
COPY deploy/nginx/snippets /etc/nginx/snippets
COPY --from=build /app/dist /usr/share/nginx/html
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1/nginx-health || exit 1
