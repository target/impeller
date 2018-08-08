FROM alpine:3.8
ENV HELM_VERSION=v2.9.1
RUN cd /tmp && \
    wget -O /tmp/helm.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-${HELM_VERSION}-linux-amd64.tar.gz && \
    tar -xvf /tmp/helm.tar.gz && \
    cp linux-amd64/helm /usr/bin/ && \
    chmod +x /usr/bin/helm && \
    rm -rf /tmp/helm.tar.gz /tmp/linux-amd64

ENTRYPOINT ["/usr/bin/propeller"]
COPY propeller /usr/bin/
