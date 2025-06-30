OAPICODEGEN_VERSION ?= v2.4.1
GOLANGCI_LINT_VERSION ?= v2.1.6
HELMDOCS_VERSION ?= v1.14.2

CURRENT_DIR=$(shell pwd)
HOST_OS?=$(shell go env GOOS)
HOST_ARCH?=$(shell go env GOARCH)
DIST_DIR=${CURRENT_DIR}/dist

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
  -X ${PACKAGE}.kubectlVersion=${KUBECTL_VERSION}

ifneq (${GIT_TAG},)
LDFLAGS += -X ${PACKAGE}.gitTag=${GIT_TAG}
endif

override GCFLAGS +=all=-trimpath=${CURRENT_DIR}

## Location to install dependencies to
LOCALBIN ?= ${CURRENT_DIR}/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: build
build: ## build binary
	CGO_ENABLED=0 GOOS=${HOST_OS} GOARCH=${HOST_ARCH} go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/api-${HOST_ARCH} -gcflags '${GCFLAGS}' ./cmd/gitfusion-api

.PHONY: generate
generate: oapi-codegen # Generate the server code from OpenAPI spec
	$(OAPICODEGEN) --config=${CURRENT_DIR}/internal/api/oapi-config.yaml ${CURRENT_DIR}/internal/api/oapi.yaml
	$(OAPICODEGEN) --config=${CURRENT_DIR}/internal/models/oapi-config.yaml ${CURRENT_DIR}/internal/api/oapi.yaml

.PHONY: lint
lint: golangci-lint ## Run go lint
	${GOLANGCI_LINT} run -v -c .golangci.yaml ./...

.PHONY: lint-fix
lint-fix: golangci-lint ## Run go lint fix
	${GOLANGCI_LINT} run -v -c .golangci.yaml ./... --fix

# Run tests
test:
	go test ./... -coverprofile=coverage.out

.PHONY: helm-docs
helm-docs: helmdocs	## Generate helm docs
	$(HELMDOCS)

GOLANGCI_LINT = ${CURRENT_DIR}/bin/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

OAPICODEGEN=$(LOCALBIN)/oapi-codegen
.PHONY: oapi-codegen
oapi-codegen: $(OAPICODEGEN) ## Download oapi-codegen locally if necessary.
$(OAPICODEGEN): $(LOCALBIN)
	$(call go-install-tool,$(OAPICODEGEN),github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen,$(OAPICODEGEN_VERSION))

HELMDOCS = $(LOCALBIN)/helm-docs
.PHONY: helmdocs
helmdocs: ## Download helm-docs locally if necessary.
	$(call go-install-tool,$(HELMDOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELMDOCS_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef