# syntax=docker/dockerfile:1

FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE=cmd/api-gateway
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/service ./${SERVICE}

FROM alpine:3.20
RUN adduser -D -g '' stu
USER stu
WORKDIR /home/stu
COPY --from=builder /out/service /usr/local/bin/service
COPY --from=builder /app/client /home/stu/client

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/service"]
