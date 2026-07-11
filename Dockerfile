# ---- 前端 build ----
FROM node:24-alpine AS web
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
# 產出 /app/web/dist

# ---- Go 後端 build ----
FROM golang:1.26-alpine AS server
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /guandan ./cmd/server

# ---- 運行環境 ----
FROM alpine:3.20
WORKDIR /app
COPY --from=server /guandan ./guandan
COPY --from=web /app/web/dist ./web/dist

# Render 會注入 PORT 環境變數，伺服器已支援；本地預設 3000
EXPOSE 3000
CMD ["./guandan"]
