# syntax=docker/dockerfile:1

FROM golang:1.27.4 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api/ api/
COPY controllers/ controllers/
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
    -o /out/rhaf-portal-operator ./cmd/rhaf-portal-operator

FROM registry.fedoraproject.org/fedora-minimal:44
LABEL org.opencontainers.image.source="https://github.com/rhaf-ai/rhaf-portal-operator" \
    org.opencontainers.image.description="RHAF Portal Operator" \
    org.opencontainers.image.licenses="Apache-2.0"

RUN microdnf install -y ca-certificates && microdnf clean all
COPY --from=builder --chown=0:0 --chmod=0555 /out/rhaf-portal-operator /usr/local/bin/rhaf-portal-operator
# OpenShift can replace this UID with its namespace-allocated UID.
USER 1001:0
ENTRYPOINT ["/usr/local/bin/rhaf-portal-operator"]
