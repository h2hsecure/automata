FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o automata ./cmd/automata

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/automata .
COPY config.example.yaml config.yaml
COPY web/ web/
EXPOSE 8080
EXPOSE 8081
EXPOSE 8082
CMD ["./automata"]
