# iStorePull

[![ci](https://github.com/pixel0verflow/iStorePull/actions/workflows/ci.yml/badge.svg)](https://github.com/pixel0verflow/iStorePull/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/pixel0verflow/iStorePull?sort=semver)](https://github.com/pixel0verflow/iStorePull/releases/latest)
[![license](https://img.shields.io/github/license/pixel0verflow/iStorePull)](LICENSE)

`ipatool` without authentication.

Apple's login (GrandSlam SRP + FairPlay-SAP token minting) can't be reproduced by
third parties. iStorePull sidesteps it: **you supply a real session captured from
Apple Configurator** (via a Charles/HAR dump or pasted headers), and the tool
replays the well-understood store endpoints to list versions and pull IPAs.

Pure Go, no cgo, single static binary (macOS/Linux/Windows).

> **Scope.** An interoperability tool for *your own* account and devices. It only
> downloads titles your account is entitled to, and never defeats FairPlay — the
> `sinf` is passed through exactly as Apple delivers it. The session file holds a
> live token; treat it as a secret.

## Requirements

- **Go 1.26+** to build (no cgo; cross-compiles to macOS/Linux/Windows).
- **A captured Configurator session.** Capturing it needs a **Mac** running
  [Apple Configurator](https://apps.apple.com/app/apple-configurator/id1037126344),
  a connected iOS device, and a TLS proxy (Charles / Proxyman / mitmproxy) with its
  root certificate trusted. See the
  [capture guide](https://gist.github.com/pixel0verflow/59f733dfe01bbb7478019d5162ea4253)
  for step-by-step instructions.
- An Apple account entitled to the titles you pull. Running the tool itself (replay,
  download, IPA assembly) works on any OS.

## Install

**Homebrew (macOS):**

```sh
brew install pixel0verflow/tap/istorepull
```

**Prebuilt binary:** grab the archive for your OS/arch from
[Releases](https://github.com/pixel0verflow/iStorePull/releases), unpack it, and put
`istorepull` on your `PATH`. macOS universal, Linux and Windows builds (amd64 +
arm64) are published for every tag, with `checksums.txt` to verify.

**From source** (Go 1.26+):

```sh
go build -o istorepull .
```

## Capture a session

### Automatic (macOS, recommended)

```sh
istorepull capture
```

`capture` runs a short-lived HTTPS proxy, trusts a throwaway CA (you'll get one
auth prompt), and points the system proxy at itself. It intercepts **only** the
store hosts — `gsa.apple.com` and everything pinned are tunnelled through
untouched, so Configurator's own auth keeps working. Then, when prompted,
**download or update any app in Apple Configurator**; the session is extracted and
saved automatically, and the proxy/CA are torn down. No Charles, no manual export.

### Manual (any proxy)

Alternatively, capture the traffic yourself in Charles / Proxyman / mitmproxy and
import it. **Do not** intercept `gsa.apple.com` (certificate-pinned); only the
`buy.itunes.apple.com` flow is needed.

```sh
istorepull token import --charles session.chlz     # Charles .chlz/.chls
istorepull token import --har dump.har             # HAR / Charles JSON export
istorepull token import --paste                     # paste raw request headers on stdin
istorepull token info                               # inspect the active session
```

Both paths pull `X-Token`, the cookie jar, `X-Apple-Store-Front`, the DSID and the
bound `guid` out of a store request.

## Use

```sh
# public, no session needed:
istorepull lookup -b com.example.app                # bundle id -> adam id
istorepull lookup -i 357218860                       # adam id  -> bundle id
istorepull search "example" -l 10

# needs an imported session:
istorepull versions -i 357218860                     # list external version ids
istorepull versions -i 357218860 --resolve --last 5  # map the newest 5 ids -> versions

istorepull download -i 357218860                     # current build
istorepull download -i 357218860 --version-id 878571262
istorepull download -b com.example.app --version 17.8.1 -o ./out/
```

The download streams the asset, verifies its `md5`, injects the `sinf` blobs and an
`iTunesMetadata.plist`, and writes a ready-to-install `.ipa`.

## Exit codes

| code | meaning |
|------|---------|
| 0 | ok |
| 2 | session expired/invalid — re-import |
| 3 | build no longer served |
| 4 | bad input |

## Layout

```
cmd/            cobra CLI (token, lookup, search, versions, download)
pkg/credential  Session model + on-disk store (~/.istorepull/session.json, 0600)
pkg/charles     .chlz / .har / pasted-headers -> Session
pkg/httpx       store HTTP client (cookie jar, pod re-POST, plist codec)
pkg/store       MZFinance replay: DownloadProduct, Versions, typed errors
pkg/itunes      public iTunes lookup/search
pkg/ipa         IPA assembly (sinf + iTunesMetadata injection)
```

## Sources for bundle / adam / external-version ids

- https://ipafilezone.su
- https://appstore.bilin.eu.org
- the per-account `softwareVersionExternalIdentifiers` list (what `versions` reads)

## Development

```sh
go test ./...
golangci-lint run ./...
```
