FROM golang:1.24.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY ./computenode .
RUN go build -o computenode computenode.go && chmod +X computenode

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/computenode /computenode
ENTRYPOINT ["/computenode"]

# WARNING: This Dockerfile must be executed from the root of the repository, with the following command:
# docker build -f ./computenode/Dockerfile -t goncaloferreirauva/computenode .
# Perhaps it would be smarter to either use a Makefile or a script to build the image.
# Or simply apply this rule to all Dockerfiles in the repository.
