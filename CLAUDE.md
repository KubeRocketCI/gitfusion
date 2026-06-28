# gitfusion

`github.com/KubeRocketCI/gitfusion` — Go HTTP server (chi router, not Echo) that normalises GitHub, GitLab, and Bitbucket APIs behind a single REST interface. Used by the KRCI portal to discover repositories, branches, pull requests, and pipelines. Reads `GitServer` CRDs from Kubernetes to resolve provider credentials at runtime.

## Build & Test

```
make build       # → dist/api-<arch>  (CGO_ENABLED=0)
make test        # runs lint first, then go test ./... -coverprofile=coverage.out
make lint        # golangci-lint v2 with .golangci.yaml
make lint-fix    # golangci-lint --fix
make generate    # regenerate server_gen.go + models_gen.go from oapi.yaml (see below)
make helm-docs   # regenerate deploy-templates/README.md from README.md.gotmpl
```

Run a single test package:

```
go test ./internal/services/pipelines/... -run TestListPipelines
```

## Architecture

### Entry point & wiring

`cmd/gitfusion-api/main.go` initialises the chi router with request-ID, structured JSON logging (`go-chi/httplog`), recovery, and a 60 s timeout, then calls `api.BuildHandler` to wire everything together. `BuildHandler` (`internal/api/server.go`) creates a controller-runtime Kubernetes client, a `GitServerService`, one multi-provider service per resource type, wraps them in oapi-codegen's `StrictHandler`, and mounts them on the router.

Required env vars: `NAMESPACE` (Kubernetes namespace to watch for `GitServer` CRs); `PORT` defaults to `8080`.

### Provider dispatch

Every resource type (repositories, branches, organisations, pull requests, pipelines) has the same three-layer structure:

```
internal/services/<resource>/
  <resource>_provider.go  — MultiProvider* struct: map[string]<Resource>Provider
                            dispatches by settings.GitProvider ("github" | "gitlab" | "bitbucket")
  <resource>_service.go   — thin wrapper that resolves GitServerSettings then calls the provider

internal/services/github/github.go    — implements all provider interfaces for GitHub
internal/services/gitlab/gitlab.go    — implements all provider interfaces for GitLab
internal/services/bitbucket/bitbucket.go — Bitbucket (pipeline jobs/traces not supported here)
```

`internal/services/krci/gitserver.go` reads the `GitServer` CR (from `edp-codebase-operator`) and the referenced K8s Secret to build `GitServerSettings`. Token resolution is cached for 60 s (bound by token-rotation concerns).

### Caching

All list responses go through `github.com/viccon/sturdyc` (sharded in-process cache with early background refreshes). Pipeline job traces use a custom `TerminalAwareCache` (`internal/cache/terminal_aware_cache.go`): finished jobs are stored in a long-TTL "done" tier so they are never evicted by running jobs. The `/api/v1/cache/invalidate` endpoint lets callers flush a named cache bucket via `DELETE`.

### OpenAPI / oapi-codegen

The single source of truth is `internal/api/oapi.yaml`. Two codegen passes are configured:

| Config | Output | Contents |
|---|---|---|
| `internal/api/oapi-config.yaml` | `internal/api/server_gen.go` | chi server stubs + `StrictServerInterface` |
| `internal/models/oapi-config.yaml` | `internal/models/models_gen.go` | Go model types |

**Never hand-edit `server_gen.go` or `models_gen.go`.** After changing `oapi.yaml`, run:

```
make generate
```

This installs `oapi-codegen` into `bin/` on first run (version-pinned via Makefile).

### Helm chart

`deploy-templates/` is a standard Helm chart. `deploy-templates/README.md` is auto-generated from `README.md.gotmpl` via `make helm-docs` — do not edit the README directly.
