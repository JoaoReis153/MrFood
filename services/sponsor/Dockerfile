FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /sponsor ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=builder /sponsor .
COPY --from=builder /app/sponsor_db.sql .
EXPOSE 5055
CMD ["./sponsor"]
