# syntax=docker/dockerfile:1.6
#
# Citadel agent — multi-stage build.
#
# IMPORTANT: this container MUST be run with `--privileged` and `--pid=host`
# (and typically `--network=host`) so it can attach eBPF programs to the host
# kernel and resolve PIDs across the runner. The /sys/fs/cgroup mount is also
# required for the cgroup_skb block-mode program.
#
#   docker run -d --name citadel \
#       --privileged --pid=host --network=host \
#       -v /sys/fs/cgroup:/sys/fs/cgroup \
#       -v /tmp:/tmp \
#       ghcr.io/<org>/citadel-agent:latest run \
#       --backend-url ... --mode audit

# ---------- Stage 1: builder ----------
FROM ubuntu:22.04 AS builder

ENV DEBIAN_FRONTEND=noninteractive \
    GO_VERSION=1.24.0

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        build-essential \
        clang-14 \
        llvm-14 \
        libelf-dev \
        libbpf-dev \
        linux-headers-generic \
        pkg-config \
        make \
        git \
        curl \
    && ln -sf /usr/bin/clang-14      /usr/bin/clang \
    && ln -sf /usr/bin/llc-14        /usr/bin/llc \
    && ln -sf /usr/bin/llvm-strip-14 /usr/bin/llvm-strip \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain (auto-detect arch: amd64 or arm64)
RUN ARCH=$(dpkg --print-architecture) \
    && curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz \
        | tar -xz -C /usr/local
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}" \
    GOPATH="/root/go" \
    CGO_ENABLED=0

WORKDIR /src

# Cache Go module deps if present.
COPY go.mod go.sum* ./
RUN if [ -f go.sum ]; then go mod download; fi

# Pull in the full agent source.
COPY . .

# 1) compile each .bpf.c → bpf/build/*.o
# 2) go generate ./...  (bpf2go embeds the .o files into Go source)
# 3) go build the agent binary
RUN make bpf && make build

# ---------- Stage 2: runtime ----------
FROM ubuntu:22.04 AS runtime

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        libelf1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/bin/citadel-agent /usr/local/bin/citadel-agent

# Default to `run` — overridable for `snapshot` / `diff` subcommands used by
# the GitHub Action's pre/post hooks.
ENTRYPOINT ["/usr/local/bin/citadel-agent"]
CMD ["run"]
