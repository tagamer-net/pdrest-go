# Tagamer PalDefender REST Client (Go)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![License: BSD 3-Clause](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)

Typed Go client for the
[PalDefender](https://ultimeit.github.io/PalDefender/) REST API
(`/v1/pdapi`): version, guilds, players, pals, items, techs, progression,
bans, kicks, broadcasts, alerts and item/pal grants.

PalDefender is a server-side validation plugin for Palworld dedicated
servers (currently Windows-based) that detects and blocks cheats,
exploits, and crashes.

The deprecated `POST /v1/pdapi/give` endpoint is intentionally not supported;
use the split reward endpoints (`GiveItems`, `GivePals`, `GivePalTemplates`,
`GivePalEggs`, `GiveProgression`) instead.

## Install

```bash
go get github.com/tagamer-net/pdrest-go
```

## Usage

```go
import "context"

client, err := pdrest.NewClient("http://127.0.0.1:17993", "bearer-token")
if err != nil {
    // handle error
}
defer client.Close()

players, err := client.GetPlayers(context.Background())
if err != nil {
    // handle error
}

_, err = client.Alert(context.Background(), "Server restart in 5 minutes")
```

All request methods accept a `context.Context` for cancellation and timeouts.

The base URL port is optional: `http` defaults to `17993` (PalDefender's
default) and `https` to `443`. Hosts must be IP addresses or RFC 1123 DNS
hostnames.

### Options

| Option                  | Description                                                        |
|-------------------------|--------------------------------------------------------------------|
| `WithTimeout(d)`        | HTTP client timeout (default: 30s)                                 |
| `WithDisplayAddress(a)` | Address reported in requests                                       |
| `WithHTTPClient(c)`     | Inject a custom `http.Client` (its `Timeout` is preserved; `Close` becomes a no-op) |
| `WithRecipeResolver(r)` | Recipe resolver for `GiveRecipeMaterials`                          |

The internally created HTTP client never follows redirects (3xx responses
surface as `*APIError`) and ignores environment proxies (`HTTP_PROXY` /
`HTTPS_PROXY`); a client injected via `WithHTTPClient` keeps its own redirect
and proxy policy.

> Note: `WithDisplayAddress` and the `Sender` field on `Broadcast` /
> `SendPlayerMessage` are client-side extensions not present in the official
> PalDefender REST API documentation; they are only sent when configured.
> They are intended to attribute actions to an issuer (for example in
> banlist records), but server support is not guaranteed by the
> documentation.

## Response models

Response models are fully typed against the official REST API documentation.
Two payloads need special attention:

- `ForgetTechResponse.Forgotten` is a `ForgottenTechs` union: the API returns
  either an array of technology IDs or the string `"All"` (exposed via the
  `All` flag).
- `GuildStorage.Slots` keeps dynamic numeric slot keys as a
  `map[string]GuildSlot`.

Guild camp pals expose `GuildCampPal.PhysicalHealth`, whose JSON key follows
the official documentation spelling `phisical_health`.

`GivePalEggs` accepts tuple inputs: a single slice of scalar values (for
example `[]any{"PalEgg_Fire_01", "Foxparks", 12}`) is interpreted as one
`(egg_id, pal_id_or_template, level)` tuple; pass `GivePalEgg` objects or maps
to grant multiple eggs.

When granting items, repeated item IDs across the inputs are combined into a
single grant entry of the summed count (keeping the first-seen order), so
`GiveItems(ctx, id, "Money", []any{"Money", 2})` sends one `Money` grant of
`3`.

## Errors

HTTP errors are returned as `*APIError` with `StatusCode`, `Method`, `Path`
and `ResponseBody`. When the response body follows the documented error
envelope (`{"Error": {"Code", "Message", "Details"}}`), it is additionally
exposed through the typed `Envelope *ErrorEnvelope` field; non-envelope
bodies leave it nil and remain available via `ResponseBody`. Error bodies are
capped at 4 KiB, so bodies larger than 4 KiB are truncated and `Envelope` can
be nil even when the body follows the envelope shape. Successful response
bodies are capped at 10 MiB; larger responses fail with an error and the
limit is not configurable. Redirect responses (3xx) surface as `*APIError`
instead of being followed.

## Platform support

The client targets 64-bit platforms, matching the Windows-based dedicated
servers that PalDefender supports. Counts are stored as `int`, and `int64`
inputs are converted directly without a range check. The 100% coverage
gate in `make check` runs on the native architecture.

## Development

```bash
make check
# or, without Make:
go test -count=1 -race ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --config .golangci.yml --timeout 5m ./...
```

## License

BSD 3-Clause — see [LICENSE](LICENSE).
