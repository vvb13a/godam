# --- 1. Build the Vue frontend ---------------------------------------------
FROM node:22-alpine AS web
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- 2. Build the Go binary (embeds frontend/dist) --------------------------
FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /godam .

# --- 3. Runtime image with the external media tools -------------------------
# ffmpeg/ffprobe are used for video frame previews (and can be extended to
# audio/video metadata); poppler-utils provides pdftoppm for PDF previews.
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ffmpeg \
      poppler-utils \
      ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /godam /usr/local/bin/godam

# Keep the SQLite database and asset storage on a mounted volume.
ENV XDG_DATA_HOME=/data
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["godam"]
