# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o catalog .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/catalog .
EXPOSE 8084
ENTRYPOINT ["./catalog"]

