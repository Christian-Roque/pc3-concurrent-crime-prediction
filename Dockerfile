FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /app/bin/pc3 ./cmd/pc3
RUN go build -o /app/bin/prepare ./cmd/prepare
RUN go build -o /app/bin/train ./cmd/train
RUN go build -o /app/bin/api ./cmd/api
RUN go build -o /app/bin/mlnode ./cmd/mlnode

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/bin/pc3 /app/pc3
COPY --from=builder /app/bin/prepare /app/prepare
COPY --from=builder /app/bin/train /app/train
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/mlnode /app/mlnode
COPY --from=builder /app/models /app/models
RUN mkdir -p /app/data /app/output /app/models
CMD ["/app/api"]
