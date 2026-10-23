# gitfusion

`github.com/KubeRocketCI/gitfusion` — Go HTTP server (chi router, not Echo) that normalises GitHub, GitLab, and Bitbucket APIs behind a single REST interface. Used by the KRCI portal to discover repositories, branches, pull requests, and pipelines. Reads `GitServer` CRDs from Kubernetes to resolve provider credentials at runtime.

## Build & Test

```
make build       # → dist/api-<arch>  (CGO_ENABLED=0)
make test        # runs lint first, then go test ./... -coverprofile=coverage.out, then the CRAP gate
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

`cmd/gitfusion-api/main.go` initialises the chi router with `httplog.RequestLogger` (chains chi `RequestID`, structured JSON logging and `Recoverer`), a 60 s timeout and a heartbeat, then mounts `api.BuildHandler` through `api.NewHTTPHandler`. `BuildHandler` (`internal/api/server.go`) creates a controller-runtime Kubernetes client; `buildServer` creates a `GitServerService` and one multi-provider service per resource type. `BuildHandler` wraps them in oapi-codegen's `StrictHandler` and starts `OrganizationsService.WarmUp` in a goroutine; the warm-up skips unsupported GitServers and continues past failing ones.

Required env vars: `NAMESPACE` (Kubernetes namespace to watch for `GitServer` CRs); `PORT` defaults to `8080`.

### Errors

Handlers return Go errors. `internal/api/problem.go` writes every API error as RFC 9457 `application/problem+json`: fixed `detail`, `requestId` equal to the `X-Request-Id` header. The cause (`err.Error()`) goes only to the request log line, field `error`; never put it in a response.

| Error (`internal/errors`) | Status |
|---|---|
| missing or invalid parameter (generated binding errors, `InvalidParamError`) | 400 |
| `ErrUnsupportedOperation`: provider not supported, or operation not implemented by the provider | 400 |
| `ErrNotFound` | 404 |
| `ErrUnauthorized`: provider rejects the stored token | 502 |
| anything else | 500 |

Unknown routes (404), wrong methods (405), panics (500) and timeouts (504) keep chi's defaults.

### Provider dispatch

Every resource type (repositories, branches, organisations, pull requests, pipelines) has the same three-layer structure:

```
internal/services/<resource>/
  <resource>_provider.go  — MultiProvider* struct: map[string]<Resource>Provider
                            dispatches by settings.GitProvider ("github" | "gitlab" | "bitbucket")
  <resource>_service.go   — thin wrapper that resolves GitServerSettings then calls the provider

internal/services/github/github.go    — GitHub (no PipelineTrigger or PipelineJobsProvider)
internal/services/gitlab/gitlab.go    — implements all provider interfaces for GitLab
internal/services/bitbucket/bitbucket.go — Bitbucket (no PipelineTrigger or PipelineJobsProvider)
```

`internal/services/krci/gitserver.go` `Resolve` order: `GitServer` CR (from `edp-codebase-operator`) → support check → Secret `token`. An unsupported operation never reads the Secret. The token is required for every provider. Cache: CR per name, token per Secret name, 60 s each (token rotation).

New operation: add one lookup in the multi-provider service that returns `gferrors.UnsupportedOperation(gitProvider, operationId)`; dispatch and `Check<Operation>` share it; the service passes `Check<Operation>` to `Resolve`. Optional capabilities (`PipelineTrigger`, `PipelineJobsProvider`) are separate interfaces found by type assertion.

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
