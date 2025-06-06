FROM golang:1.24.4-alpine AS builder

WORKDIR /app
COPY "../go.mod" "../go.sum" ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o computenode computenode.go && chmod +x computenode

FROM gcr.io/distroless/static
COPY --from=builder /app/computenode /computenode
ENTRYPOINT ["/computenode"]
