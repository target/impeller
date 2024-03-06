FROM golang:1.21-alpine as builder
ENV DESIRED_VERSION=v3.14.2
ENV HELM_DIFF_VERSION=v3.9.5
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

FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:alpine as gcloud
RUN gcloud components install gke-gcloud-auth-plugin


FROM alpine:latest
ENV KUBECTL_VERSION=v1.29.2
RUN apk add ca-certificates
RUN wget -O /usr/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x /usr/bin/kubectl
RUN mkdir /root/.kube
ENV USE_GKE_GCLOUD_AUTH_PLUGIN=True
ENTRYPOINT ["/usr/bin/impeller"]
COPY --from=gcloud /google-cloud-sdk/bin/gke-gcloud-auth-plugin /bin/gke-gcloud-auth-plugin
ENV USE_GKE_GCLOUD_AUTH_PLUGIN=True
COPY --from=builder /go/src/github.com/target/impeller/impeller /usr/bin/impeller
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /root/.local /root/.local
COPY --from=builder /root/.cache /root/.cache
