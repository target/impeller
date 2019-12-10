FROM golang:1.12-alpine as builder
WORKDIR /go/src/github.com/target/impeller
COPY . .
ENV GO111MODULE=on
RUN apk add --no-cache git && \
    go get -d ./... && \
    go build
RUN apk add --update openssl && \
    rm -rf /var/cache/apk/*
RUN apk update && apk add bash
RUN apk add ca-certificates
RUN cd /tmp && \
    wget -O get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 \
    && chmod +x get_helm.sh \
    && ./get_helm.sh

FROM alpine:3.8
ENV DESIRED_VERSION=v3.0.1
ENV KUBECTL_VERSION=v1.15.6

RUN apk add ca-certificates
RUN wget -O /usr/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x /usr/bin/kubectl
RUN mkdir /root/.kube
ENTRYPOINT ["/usr/bin/impeller"]
COPY --from=builder /go/src/github.com/target/impeller/impeller /usr/bin/impeller
