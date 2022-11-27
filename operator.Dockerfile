# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/operator cmd/operator
COPY api/ api/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 go build -a -o /operator cmd/operator/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /operator /usr/local/bin/operator
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/operator"]
