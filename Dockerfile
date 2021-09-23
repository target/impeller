FROM golang:1.15-alpine as builder
ENV DESIRED_VERSION=v3.7.0
ENV HELM_DIFF_VERSION=v3.1.3
WORKDIR /go/src/github.com/target/impeller
COPY . .
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
RUN apk add --no-cache git && \
    go mod vendor && \
    go build -mod vendor -a -installsuffix cgo -ldflags '-extldflags "-static"' -o impeller . && \
    go test -v -coverprofile cp.out
RUN apk add --update openssl && \
    rm -rf /var/cache/apk/*
RUN apk update && apk add bash git openssh
RUN apk add ca-certificates
RUN cd /tmp && \
    wget -O get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 \
    && chmod +x get_helm.sh \
    && ./get_helm.sh
RUN /usr/local/bin/helm plugin install https://github.com/databus23/helm-diff --version ${HELM_DIFF_VERSION}

FROM alpine:latest
ENV KUBECTL_VERSION=v1.18.19
RUN apk add ca-certificates
RUN wget -O /usr/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x /usr/bin/kubectl
RUN mkdir /root/.kube
ENTRYPOINT ["/usr/bin/impeller"]
COPY --from=builder /go/src/github.com/target/impeller/impeller /usr/bin/impeller
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /root/.local /root/.local
COPY --from=builder /root/.cache /root/.cache
