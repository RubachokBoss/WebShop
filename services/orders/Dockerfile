# syntax=docker/dockerfile:1
FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o orders .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/orders .
EXPOSE 8081
ENTRYPOINT ["./orders"]

