FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

FROM builder AS dev
RUN apk add --no-cache git
RUN go install github.com/air-verse/air@latest
COPY .air.toml ./
CMD ["air"]

FROM builder AS prod
COPY . .
RUN go build -o main .
CMD ["./main"]
