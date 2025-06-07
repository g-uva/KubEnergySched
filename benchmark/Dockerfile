FROM golang:1.24-alpine AS builder

WORKDIR /scheduler
COPY go.mod go.sum ./
RUN go mod download

COPY ./benchmark .
RUN go build -o benchmark benchmark.go && chmod +x benchmark

# For debugging
FROM alpine:3.22
# For production
# FROM gcr.io/distroless/base-debian11
COPY --from=builder /scheduler/benchmark /benchmark
ENTRYPOINT ["/benchmark"]

