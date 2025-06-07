FROM golang:1.24-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /bin/kube-scheduler main.go
FROM k8s.gcr.io/kube-scheduler:v1.28.8
COPY --from=builder /bin/kube-scheduler /usr/local/bin/kube-scheduler
