FROM golang:1.21 AS builder

WORKDIR /app
COPY . .
RUN go build -o centralunit centralunit.go && chmod +X centralunit

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/centralunit /centralunit
ENTRYPOINT ["/centralunit"]

