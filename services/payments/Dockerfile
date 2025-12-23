# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o payments .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/payments .
EXPOSE 8082
ENTRYPOINT ["./payments"]

