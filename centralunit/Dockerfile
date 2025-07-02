FROM golang:1.24.4-alpine AS builder

RUN apk add --no-cache bash curl

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o centralunit-bin ./centralunit/centralunit.go && chmod +x centralunit-bin

RUN ls -l /app/centralunit

# For debugging
FROM alpine:3.22
# For production
# FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/centralunit-bin /centralunit-bin
ENTRYPOINT ["/centralunit-bin"]

