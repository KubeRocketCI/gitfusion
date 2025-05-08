OAPICODEGEN_VERSION ?= v2.4.1
CURRENT_DIR=$(shell pwd)

## Location to install dependencies to
LOCALBIN ?= ${CURRENT_DIR}/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: generate
generate: oapi-codegen # Generate the server code from OpenAPI spec
	$(OAPICODEGEN) --config=${CURRENT_DIR}/internal/api/oapi-config.yaml ${CURRENT_DIR}/internal/api/oapi.yaml
	$(OAPICODEGEN) --config=${CURRENT_DIR}/internal/models/oapi-config.yaml ${CURRENT_DIR}/internal/api/oapi.yaml

OAPICODEGEN=$(LOCALBIN)/oapi-codegen
.PHONY: oapi-codegen
oapi-codegen: $(OAPICODEGEN) ## Download oapi-codegen locally if necessary.
$(OAPICODEGEN): $(LOCALBIN)
	$(call go-install-tool,$(OAPICODEGEN),github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen,$(OAPICODEGEN_VERSION))

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