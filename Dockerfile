FROM golang:1.20 as builder
WORKDIR /src
COPY . .
RUN GO111MODULE=on go build -o /bin/kube-scheduler main.go

FROM k8s.gcr.io/kube-scheduler:v1.28.8
COPY --from=builder /bin/kube-scheduler /usr/local/bin/kube-scheduler
