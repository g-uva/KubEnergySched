FROM golang:1.24.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./computenode .
RUN go build -o computenode computenode.go && chmod +X computenode

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/computenode /computenode
ENTRYPOINT ["/computenode"]
