FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o automata ./cmd/automata && \
    CGO_ENABLED=0 go build -o bff ./cmd/bff

FROM node:20-alpine AS webui
WORKDIR /app/webui
COPY webui/package.json webui/package-lock.json ./
RUN npm ci
COPY webui/ .
RUN npm run build

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/automata .
COPY --from=builder /app/bff .
COPY config.example.yaml config.yaml
COPY --from=webui /app/web /root/web
EXPOSE 8080
EXPOSE 8081
EXPOSE 8082
CMD ["./automata"]
