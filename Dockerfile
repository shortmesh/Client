FROM golang:1.24-bookworm AS builder

RUN apt-get update && apt-get install -y \
    gcc \
    libolm-dev \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY . .

RUN swag init

RUN CGO_ENABLED=1 go build -o bin/client .

FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y \
    ca-certificates \
    libolm3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/client /app/client
COPY --from=builder /app/docs /app/docs

RUN mkdir -p /app/db /app/downloads

EXPOSE 8080

CMD ["./client"]
