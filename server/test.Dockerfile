# syntax=docker/dockerfile:1

# The Linux Server gate in one container: the same packages and Task targets
# as the server-linux and server-volumes-linux CI jobs, run on a Docker host
# chosen by DOCKER_CONTEXT and built there (task server:test:linux). The
# image carries the repository files the Server tests read (web routes and
# Problem catalog, site Problem pages, docs) and nothing it does not.
ARG GO_VERSION=1.25.12
FROM golang:${GO_VERSION}-trixie

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential pkg-config libsqlite3-dev libvips-dev libraw-dev \
        libheif-plugin-aomenc libheif-plugin-libde265 \
        dosfstools exfatprogs e2fsprogs \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --uid 1001 runner
USER runner
ENV GOPATH=/home/runner/go \
    GOCACHE=/home/runner/.cache/go-build \
    PATH=/home/runner/go/bin:/usr/local/go/bin:$PATH
RUN go install github.com/go-task/task/v3/cmd/task@v3.52.0

WORKDIR /src/server
COPY --chown=runner server/go.mod server/go.sum ./
RUN go mod download

COPY --chown=runner . /src
USER root
COPY server/test-linux-gate.sh /usr/local/bin/lumilio-linux-gate
ENTRYPOINT ["/usr/local/bin/lumilio-linux-gate"]
