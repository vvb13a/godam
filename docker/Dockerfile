# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Binary
FROM golang:1.27-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy compiled SPA into internal static directory for go:embed
COPY --from=frontend-builder /app/web/dist internal/delivery/http/static/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o dam ./cmd/dam

# Stage 3: Final Runner
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /app/dam /usr/local/bin/dam
EXPOSE 8080
ENTRYPOINT ["dam"]
CMD ["serve"]