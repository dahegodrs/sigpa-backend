# ── Etapa 1: Build ─────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copiar dependencias y descargar módulos (cacheado si go.mod no cambia)
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar el binario estático
RUN CGO_ENABLED=0 GOOS=linux go build -o sigpa-backend ./cmd/api/main.go

# ── Etapa 2: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.19

# Certificados TLS para llamadas HTTPS (Google Drive, Gmail, Supabase)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copiar binario compilado
COPY --from=builder /app/sigpa-backend .

# Carpeta de credenciales (se monta vía Railway Volume o variable de entorno)
RUN mkdir -p credentials

EXPOSE 8080

CMD ["./sigpa-backend"]
