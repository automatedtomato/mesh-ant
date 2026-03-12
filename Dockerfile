# Stage 1: Build
# Compile the demo binary inside the official Go image.
# The module root is meshant/ (where go.mod lives), so we work from there.
FROM golang:1.25-alpine AS builder

WORKDIR /src/meshant
COPY meshant/ .

RUN go build -o /demo ./cmd/demo/
RUN go build -o /meshant ./cmd/meshant

# Stage 2: Runtime
# Copy only the binaries and the default dataset into a minimal Alpine image.
# No source code, no Go toolchain, no build artefacts at runtime.
FROM alpine:latest

# Dataset lives at the path the binary expects when called with an argument.
# The ENTRYPOINT passes this path explicitly so the binary is path-agnostic.
COPY --from=builder /demo /demo
COPY --from=builder /meshant /usr/local/bin/meshant
COPY data/examples/evacuation_order.json /data/examples/evacuation_order.json

ENTRYPOINT ["/demo", "/data/examples/evacuation_order.json"]
