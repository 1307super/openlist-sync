FROM node:20-alpine AS frontend
WORKDIR /build/web-ui
COPY web-ui/package.json web-ui/package-lock.json ./
RUN npm ci
COPY web-ui/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/internal/static/dist /build/internal/static/dist
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /openlist-sync ./cmd

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone
ENV TZ=Asia/Shanghai
COPY --from=backend /openlist-sync /usr/local/bin/openlist-sync
ENV PORT=3000 DATA_DIR=/data
EXPOSE 3000
VOLUME /data
ENTRYPOINT ["openlist-sync"]
