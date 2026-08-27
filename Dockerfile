FROM node:22-alpine AS frontend-build
WORKDIR /frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

FROM golang:1.24-alpine AS backend-build
WORKDIR /src
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-build /frontend/dist ./static
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/news ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-build /out/news /app/news
RUN mkdir -p /data
ENV PORT=8080 DATABASE_PATH=/data/news.db
EXPOSE 8080
CMD ["/app/news"]
