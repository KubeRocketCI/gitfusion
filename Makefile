OAPICODEGEN_VERSION ?= v2.4.1
GOLANGCI_LINT_VERSION ?= v2.1.6
HELMDOCS_VERSION ?= v1.14.2
GITCHGLOG_VERSION ?= v0.15.4
GO_CRAP_VERSION ?= v0.5.1
GOJQ_VERSION ?= v0.12.19

PACKAGE=github.com/epam/edp-common/pkg/config
CURRENT_DIR=$(shell pwd)
HOST_OS?=$(shell go env GOOS)
HOST_ARCH?=$(shell go env GOARCH)
DIST_DIR=${CURRENT_DIR}/dist

VERSION?=$(shell git describe --tags)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_TAG=$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT}

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

# use https://github.com/git-chglog/git-chglog/
.PHONY: changelog
changelog: git-chglog	## generate changelog
ifneq (${NEXT_RELEASE_TAG},)
	$(GITCHGLOG) --next-tag v${NEXT_RELEASE_TAG} -o CHANGELOG.md
else
	$(GITCHGLOG) -o CHANGELOG.md
endif

# Run tests, then the CRAP gate on the produced coverage profile; CRAP_CHECK=false skips the gate
test: lint
	go test ./... -coverprofile=coverage.out
ifneq ($(CRAP_CHECK),false)
	@$(MAKE) --no-print-directory crap-check
endif

# CRAP = CC^2 * (1 - coverage)^3 + CC per function, scored from the coverage profile of make test.
# Gate logic and its settings live in hack/crap-gate.sh.
CRAP_CHECK ?= true
CRAP_GATE = GO_CRAP=$(GO_CRAP) GOJQ=$(GOJQ) ./hack/crap-gate.sh

.PHONY: crap
crap: go-crap gojq ## Report the worst CRAP scores
	@$(CRAP_GATE) report

.PHONY: crap-check
crap-check: go-crap gojq ## Fail on CRAP regressions vs baseline and new functions above threshold
	@$(CRAP_GATE) check

.PHONY: crap-baseline
crap-baseline: go-crap gojq ## Rewrite the CRAP baseline from the current coverage profile
	@$(CRAP_GATE) baseline

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

GO_CRAP = $(LOCALBIN)/go-crap
.PHONY: go-crap
go-crap: $(LOCALBIN) ## Download go-crap locally if necessary.
	$(call go-install-tool,$(GO_CRAP),github.com/padiazg/go-crap,$(GO_CRAP_VERSION))

GOJQ = $(LOCALBIN)/gojq
.PHONY: gojq
gojq: $(LOCALBIN) ## Download gojq locally if necessary.
	$(call go-install-tool,$(GOJQ),github.com/itchyny/gojq/cmd/gojq,$(GOJQ_VERSION))

HELMDOCS = $(LOCALBIN)/helm-docs
.PHONY: helmdocs
helmdocs: ## Download helm-docs locally if necessary.
	$(call go-install-tool,$(HELMDOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELMDOCS_VERSION))

GITCHGLOG = $(LOCALBIN)/git-chglog
.PHONY: git-chglog
git-chglog: ## Download git-chglog locally if necessary.
	$(call go-install-tool,$(GITCHGLOG),github.com/git-chglog/git-chglog/cmd/git-chglog,$(GITCHGLOG_VERSION))

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
