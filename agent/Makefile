SHELL := /bin/bash

BPF_DIR    := bpf
BPF_BUILD  := $(BPF_DIR)/build
BPF_SRCS   := $(wildcard $(BPF_DIR)/*.bpf.c)
BPF_OBJS   := $(patsubst $(BPF_DIR)/%.bpf.c,$(BPF_BUILD)/%.bpf.o,$(BPF_SRCS))

CLANG      ?= clang

# Map `uname -m` to the __TARGET_ARCH_* define used by libbpf / vmlinux.h.
# macOS arm64 reports `arm64`; Linux ARM reports `aarch64`; Linux x86 reports
# `x86_64`. We feed all of these through to the right libbpf arch name.
UNAME_M    := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
    BPF_ARCH := x86
else ifeq ($(UNAME_M),aarch64)
    BPF_ARCH := arm64
else ifeq ($(UNAME_M),arm64)
    BPF_ARCH := arm64
else
    BPF_ARCH := $(UNAME_M)
endif

CFLAGS_BPF := -O2 -g -Wall -target bpf -D__TARGET_ARCH_$(BPF_ARCH) -I$(BPF_DIR)

BIN        := bin/citadel-agent

.PHONY: all bpf generate build clean net-smoke-test

all: build

# Compile every .bpf.c → bpf/build/%.bpf.o
bpf: $(BPF_OBJS)

$(BPF_BUILD)/%.bpf.o: $(BPF_DIR)/%.bpf.c | $(BPF_BUILD)
	$(CLANG) $(CFLAGS_BPF) -c $< -o $@

$(BPF_BUILD):
	mkdir -p $(BPF_BUILD)

# bpf2go runs at `go generate` time and produces Go bindings from each .bpf.c.
generate:
	go generate ./...

# Full build: compile BPF objects, run bpf2go, then build the Go binary.
build: bpf generate
	mkdir -p bin
	go build -o $(BIN) ./cmd/agent

clean:
	rm -rf $(BPF_BUILD) bin
	find ./internal/probes -type f \( -name '*_bpfel.go' -o -name '*_bpfeb.go' -o -name '*_bpfel.o' -o -name '*_bpfeb.o' \) -delete 2>/dev/null || true

# Phase 1 smoke test: load the network probe, fire two curl(1) calls, then
# assert at least one outbound JSON event was emitted with comm=curl.
# Requires: Linux, sudo (for BPF), clang (for `make build`), curl, internet.
net-smoke-test: build
	@[ "$$(uname -s)" = "Linux" ] || { echo "ERROR: net-smoke-test requires Linux (eBPF)"; exit 1; }
	@OUT=$$(mktemp -t citadel-smoke.XXXXXX); \
	echo "==> Starting agent (sudo). Output → $$OUT"; \
	sudo ./$(BIN) run > "$$OUT" 2>&1 & \
	sleep 1; \
	echo "==> Triggering connections"; \
	curl -s -o /dev/null https://github.com           || true; \
	curl -s -o /dev/null https://registry.npmjs.org   || true; \
	sleep 3; \
	echo "==> Stopping agent"; \
	sudo pkill -f citadel-agent 2>/dev/null || true; \
	sleep 1; \
	echo "==> Captured output:"; \
	cat "$$OUT"; \
	echo; \
	if grep -q '"comm":"curl"' "$$OUT" && grep -qE '(github\.com|registry\.npmjs\.org|"dst_port":443)' "$$OUT"; then \
		echo "PASS"; \
		rm -f "$$OUT"; \
	else \
		echo "FAIL — expected at least one curl event with github/npm hostname or dst_port=443"; \
		rm -f "$$OUT"; \
		exit 1; \
	fi
