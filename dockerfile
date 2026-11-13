# Build stage
# Go con soporte vigente (nov-2026): la 1.22 dejó de recibir parches de seguridad en
# feb-2025. Las pruebas pasan igual con la 1.26 (probado con la 1.26.11).
# Ojo antes de subir a la 1.27: ahí encoding/json ya no dice en qué campo falló un
# tipo propio (models.Text), y el motivo de un registro rechazado en /UploadData
# pierde el nombre del campo (fallan TestTextAcceptsNumbers y
# TestUploadDataRejectsOnlyBadRecords). Hay que adaptar models/text.go primero. La
# 1.26 deja de tener soporte cuando salga la 1.28 (hacia feb-2027).
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
