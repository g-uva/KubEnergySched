FROM golang:1.21 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Cross-compile statically for Linux AMD64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o computenode computenode.go

FROM gcr.io/distroless/static
COPY --from=builder /app/computenode /computenode
ENTRYPOINT ["/computenode"]
