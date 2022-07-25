# This Makefile generates a bin/ directory in here to store some binaries. Then make generate and make test
# use those binaries.
BIN_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/bin

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/controller-gen: $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2

$(BIN_DIR)/setup-envtest: $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: generate
generate: $(BIN_DIR)/controller-gen
	PATH="$(PATH):$(BIN_DIR)" go generate ./api/...

ENVTEST_K8S_VERSION ?= 1.24

.PHONY: test
test: $(BIN_DIR)/setup-envtest
	KUBEBUILDER_ASSETS="$(shell $(BIN_DIR)/setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" go test ./...

.PHONY: clean
clean:
	rm -r $(BIN_DIR)
