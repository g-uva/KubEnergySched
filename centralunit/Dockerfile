FROM golang:1.24.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./centralunit .
RUN go build -o centralunit centralunit.go && chmod +X centralunit

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/centralunit /centralunit
ENTRYPOINT ["/centralunit"]

