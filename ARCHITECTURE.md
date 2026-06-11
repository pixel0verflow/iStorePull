# Architecture

iStorePull is `ipatool` minus authentication. Apple's login (GrandSlam SRP +
FairPlay-SAP token minting) needs Apple's private key material and can't be
reproduced by third parties. iStorePull sidesteps it entirely: you supply a real
session captured from Apple Configurator, and the tool replays the well-understood
store endpoints. Pure Go, no cgo, single static binary.

## Data flow

```
.chlz/.har/paste ─▶ charles ─▶ Session ─▶ store (replay MZFinance) ─▶ DownloadItem ─▶ ipa (assemble) ─▶ .ipa
                                  │            ▲
                          credential          httpx (transport)
                       (load/save 0600)

itunes (public lookup/search) ─▶ adamId          (no session needed)
```

Packages depend strictly downward (`credential` → `charles`/`store` → `cmd`), and
each compiles and tests standalone.

## Packages (bottom → top)

### `pkg/credential` — the core data model
- `Session`: `XToken` (the real store credential), cookie jar, `StoreFront`,
  `DSID`, `GUID` (bound to the token), `UserAgent`, `AppleID`.
- `Valid()` enforces the four required fields. `Headers()` / `CookieHeader()` /
  `HTTPCookies()` render the session onto outgoing requests.
- `store.go`: persists to `~/.istorepull/session.json`, mode `0600`. Depends on
  nothing; everything else depends on it.

### `pkg/charles` — capture → `Session`
Three entry points converge on one `buildSession`:
- `dump.go` — shared `flow` model, `pickFlow` (prefer a store-path flow carrying an
  `X-Token`), `buildSession` (maps headers → `Session`, pulls `guid` from the URL).
- `headers.go` `ParseHeaders` (pasted raw headers), `har.go` `ParseHAR`
  (HAR + Charles-JSON), `chlz.go` `ParseDump` (ZIP of `N-meta.json`; dispatches
  `.har`/`.json` to `ParseHAR`).
- Store markers: `volumeStoreDownloadProduct`, `DownloadDispatch…`, `buyProduct`,
  `MZFinance.woa`.

### `pkg/httpx` — store transport
- `client.go` — a cookie-jar HTTP client. **Key trick:** `CheckRedirect` returns
  `ErrUseLastResponse` (never auto-follow), and `postPreservingBody` manually
  re-issues the **POST** across Apple's pod (`pN-buy`) 302 redirects. Go's default
  redirect handling downgrades POST → GET and drops the body; this keeps it.
  `PostPlist` marshals the body, POSTs, and decodes the response.
- `plist.go` — `MarshalPlist` / `UnmarshalPlist`; `sanitize` trims stray leading
  bytes before the `<?xml` / `<plist` / `bplist` marker.

### `pkg/store` — MZFinance replay (the heart)
- `types.go` — `Sinf`, `DownloadItem`, `VersionList`, the wire `songItem` /
  `productResponse`, and `toDownloadItem`.
- `payload.go` — `downloadPayload` plist:
  `creditDisplay / guid / salableAdamId / pricingParameters=STDQ [/ externalVersionId]`.
- `client.go` — the `Client` interface and implementation.
  `DownloadProduct(adamID, externalVersionID)` (`""` = current build) POSTs, checks
  `failureType`, and returns the first `songList` item.
- `errors.go` — maps `failureType` codes to typed sentinels: 2042/2034/2060 →
  `ErrSessionExpired`, 9610 → `ErrNoLicense`, 2059 → `ErrUnavailable`. All
  `errors.Is`-matchable.
- `versions.go` — `Versions` reads `softwareVersionExternalIdentifiers` off a
  current product response. `ResolveVersions` probes ids → version strings
  (throttled, skips builds Apple no longer serves). `FindExternalID` resolves a
  version string → external id, scanning newest-first.

### `pkg/itunes` — public iTunes API
No credentials. `LookupBundle` / `LookupID` / `Search`. Used to turn a bundle id
into an adam id.

### `pkg/ipa` — IPA assembly
`Build(BuildInput)`:
1. open the downloaded zip, find `Payload/<App>.app/`;
2. sinf destination = `Manifest.plist` `SinfPaths` if present, else
   `SC_Info/<CFBundleExecutable>.sinf`;
3. copy every entry verbatim, write the sinf blobs, add `iTunesMetadata.plist`
   (plus `apple-id` / `userName`). Nothing is decrypted — the `sinf` is passed
   through exactly as Apple delivers it.

### `cmd` — cobra CLI (thin over the packages)
- `root.go` — command tree, global flags (`--session` / `--storefront` /
  `--verbose`), exit codes (2 dead token, 3 build not served, 4 bad input),
  `loadSession`.
- `token.go` (`import` / `info`), `lookup.go` (`lookup` + `search`), `versions.go`,
  `download.go`.
- `common.go` — `resolveAdamID` (bundle → id via `itunes`) and the vermap cache
  (`~/.istorepull/vermap/<adamId>.json`).
- `download.go` orchestrates the full pull: resolve adamId → resolve external id →
  `DownloadProduct` → stream with a progress bar → verify `md5` → `ipa.Build` →
  move to `-o`.

## Testing

Per-package, stdlib `testing` plus `httptest` — no live Apple calls in CI:
- `charles`: golden parses for `.chlz` / HAR / pasted headers;
- `credential`: save/load round-trip and 0600 permission;
- `store`: pod-redirect body preservation, `failureType` → typed errors, version
  list parsing, all against canned plist responses;
- `ipa`: synthetic-zip build asserting sinf path + `iTunesMetadata.plist`;
- `itunes`: lookup/search against canned JSON.

Lint is golangci-lint v2 (`.golangci.yml`); CI is `.github/workflows/ci.yml`
(build, `go test -race`, lint).

## Why this shape

Login is the one part that can't be reproduced, so `credential` is *given* (captured
from Configurator), not earned. Everything downstream is plain, well-understood
HTTP. Keeping the dependency graph strictly downward makes each layer independently
testable and keeps the binary cgo-free and cross-platform.
