# tf-provider-migrate

A CLI tool to help migrate an existing Terraform provider from SDKv2 to the provider Framework.
[There are a set of quite repeatable steps in this migration](https://danielmschmidt.de/posts/2025-09-17-terraform-provider-sdk-to-framework-migration/) that I think could be automated to some degree. This tool is a first attempt at that.

## Goals

- Auto-Detection if the provider at hand is a good candidate for migration (follows the conventions of SDKv2 providers)
- The migration step should mux the provider and add an empty framework structure alongside the existing SDKv2 code
- The framework structure needs to replicate the provider block from the SDKv2 code
- The user can then start to add new resources in the framework structure while still maintaining the existing SDKv2 resources
- Eventually, the user can then start migrating resources one by one from SDKv2 to the framework structure
- Once all resources are migrated, the user can then remove the SDKv2 code and the muxing.

## Usage

Build:

```bash
go build -o /tmp/tf-provider-migrate ./cmd/tf-provider-migrate
```

Check a provider:

```bash
/tmp/tf-provider-migrate check --path /path/to/provider
```

Migrate a provider (adds framework scaffold + muxed `main.go`):

```bash
/tmp/tf-provider-migrate migrate --path /path/to/provider
```

Optional flags:
- `--registry-address`: override the registry address used by `tf5server.Serve`
- `--provider-name`: override the provider type name in framework metadata
- `--dry-run`: show the plan without writing files (for `migrate`)
- `--vendor`: `auto` (default, run vendor only if `vendor/` exists), `on` (force `go mod vendor`), `off` (skip vendoring)

## Generated layout

After `migrate`, a new framework scaffold is created at:

```
framework/provider.go
```

`main.go` is rewritten to use `terraform-plugin-mux` so the SDKv2 provider and the framework provider can run side-by-side.

## Limitations

- The parser expects the SDKv2 provider schema to be in a `Provider() *schema.Provider` function.
- Provider schema can be a literal, a named map variable, or returned from a helper function.
- Only the provider block is replicated (resources/data sources are not migrated).
- Nested blocks are supported only for list/set blocks with `Elem: &schema.Resource{...}`.

If `check` fails, it will report the first unsupported pattern it encountered.

## Integration validation

There is an integration test that clones real providers, runs `check` + `migrate`, and then compiles them.
Run it with:

```bash
go test -tags=integration ./internal/migrate -run TestRealProviders
```

The provider list is fixed to specific git SHAs to keep integration validation deterministic.
