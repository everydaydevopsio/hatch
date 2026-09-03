<p align="center">
  <img src=".github/assets/icon.svg" alt="Hatch project icon" width="128">
</p>

<h1 align="center">Hatch</h1>

<p align="center">
  <strong>A disposable browser desktop for completing localhost OAuth flows on a headless Linux server.</strong>
</p>

<p align="center">
  <a href="https://github.com/everydaydevopsio/hatch/actions/workflows/go.yml"><img src="https://github.com/everydaydevopsio/hatch/actions/workflows/go.yml/badge.svg" alt="Go tests"></a>
  <a href="https://github.com/everydaydevopsio/hatch/actions/workflows/docker.yml"><img src="https://github.com/everydaydevopsio/hatch/actions/workflows/docker.yml/badge.svg" alt="Container build"></a>
  <a href="https://github.com/everydaydevopsio/hatch/pkgs/container/hatch"><img src="https://img.shields.io/badge/container-ghcr.io-2496ED" alt="GHCR container"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/everydaydevopsio/hatch" alt="License"></a>
</p>

Hatch gives a headless Linux server a small Chromium desktop that you open through HTTPS. It is designed for interactive OAuth flows where a CLI, MCP server, or agent starts a callback listener on server loopback, such as `http://127.0.0.1:8765/callback`.

The Go CLI creates an ephemeral Docker session, opens Chromium at the authorization URL, and prints a signed browser-access URL. Complete the provider login from your laptop, tablet, or phone. Chromium follows the final localhost redirect back to the waiting process on the server.

## The problem Hatch solves

A command running on a remote server often prints an OAuth authorization URL and waits for a callback on `127.0.0.1`. Opening that URL in your local browser sends the callback to your laptop instead of the remote process.

Hatch places the interactive browser on the server while making its desktop available through a normal web browser:

```text
Your browser
    |
    | HTTPS
    v
nginx
    |
    v
Apache Guacamole -> guacd -> xrdp -> Openbox -> Chromium
                                           |
                                           | localhost OAuth callback
                                           v
                               CLI, MCP server, or agent
```

The Hatch CLI runs its container with host networking. Chromium therefore shares the Linux host's loopback view and can reach the callback listener.

## Quick start

The current CLI builds and launches the local `hatch:local` image.

Requirements:

- A Linux host with Docker Engine.
- Go 1.24 or newer to build the CLI.
- A private hostname or IP reachable from your browser. Tailscale or another VPN is recommended.
- About 1 GB of free memory while Chromium runs.

Build and install:

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch

make setup
make build
sudo install -m 0755 bin/hatch /usr/local/bin/hatch
```

Initialize Hatch with the hostname used to reach the server:

```bash
hatch init devbox.tailnet.ts.net
```

Add a fixed HTTPS port when required:

```bash
hatch init devbox.tailnet.ts.net:8443
```

Launch a browser desktop directly at an OAuth authorization URL:

```bash
hatch open 'https://example.com/oauth/authorize?...'
```

Hatch prints output similar to:

```text
Session:      8ac4d911
Container:    hatch-8ac4d911
Start URL:    https://example.com/oauth/authorize?...
Browser URL:  https://devbox.tailnet.ts.net:18000/hatch/?token=eyJ...
Stop with:    hatch stop 8ac4d911
```

Open the `Browser URL`, accept the self-signed certificate warning when applicable, and complete authentication.

## CLI commands

| Command | Purpose |
| --- | --- |
| `hatch init [hostname[:port]\|hostname port]` | Check Docker and save the public hostname with an optional default port |
| `hatch open [--port port] <url>` | Start an ephemeral browser-desktop session |
| `hatch list` | List Hatch-managed containers |
| `hatch stop <session>` | Stop one session by ID or unique prefix |
| `hatch stop --all` | Stop every Hatch-managed session |
| `hatch help` | Show command help |

Port selection follows this order:

1. `--port` supplied to `hatch open`.
2. The port saved by `hatch init`.
3. An available port from `18000-18999`.

The CLI stores configuration at `~/.config/hatch/hatch.yaml`, with directory mode `0700` and file mode `0600`.

## Session behavior

Each CLI-managed session:

- uses the `hatch:local` image;
- runs with Docker host networking;
- gets a short session ID and Docker labels;
- receives 1 GiB of shared memory;
- enables `no-new-privileges`;
- uses a one-hour Guacamole launch-token lifetime;
- waits for the HTTPS health check before printing the access URL.

Hatch creates a random RDP password and Guacamole JWT secret unless you provide values for a manually managed container. The access URL is a bearer credential. Anyone who has it can open the desktop until its launch token expires.

## Run the container without the CLI

Build the image:

```bash
docker build -t hatch:local .
```

For a browser that must reach a callback listener on Linux host loopback:

```bash
docker run -d \
  --name hatch \
  --network host \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e HATCH_HTTPS_PORT=8443 \
  -e HATCH_START_URL='https://example.com/oauth/authorize?...' \
  hatch:local
```

Read the generated access URL:

```bash
docker logs hatch
```

A manual container defaults to a 12-hour launch-token lifetime. Set `HATCH_GUAC_LAUNCH_TTL_SECONDS` to shorten it.

The repository also publishes `ghcr.io/everydaydevopsio/hatch`. The CLI currently uses the locally built image so its binary and container can evolve together without registry authentication.

## Configuration

Common container environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `RDP_USER` | `oauth` | Linux user for the desktop session |
| `RDP_PASSWORD` | generated | RDP password stored in the protected recovery log |
| `HATCH_GUAC_JWT_SECRET` | generated | Guacamole JWT signing secret; must be at least 32 characters |
| `HATCH_GUAC_LAUNCH_TTL_SECONDS` | `43200` | Launch-token lifetime for manually started containers |
| `HATCH_HTTPS_PORT` | `443` | HTTPS listener inside the container or on the host in host-network mode |
| `HATCH_START_URL` | `about:blank` | Initial Chromium URL |
| `HATCH_MAC_SHORTCUTS` | `1` | Map common Mac shortcuts to Linux Chromium shortcuts |
| `CHROMIUM_EXTRA_FLAGS` | empty | Additional Chromium command-line flags |
| `HATCH_TLS_CERT` | `/etc/hatch/tls/hatch.crt` | Certificate path |
| `HATCH_TLS_KEY` | `/etc/hatch/tls/hatch.key` | Private-key path |
| `HATCH_TLS_CN` | `hatch.local` | Common name for the generated certificate |
| `HATCH_TLS_DAYS` | `365` | Lifetime of the generated certificate |

Use [INSTALL.md](INSTALL.md) for custom TLS mounts, Docker Compose, browser-profile persistence, clipboard instructions, and troubleshooting.

## Supported environments

OAuth callback mode targets a native Linux Docker Engine host. Docker Desktop on macOS and Windows virtualizes host networking differently, so a container's `127.0.0.1` may not reach a callback listener on the desktop host.

The Go CLI can detect missing or stopped Docker installations on macOS, Windows, and common Linux distributions. That guidance helps with setup, but it does not change the callback networking limitation.

## Security

Hatch exposes a complete interactive browser session. Treat it like temporary administrative access.

- Keep the HTTPS endpoint on Tailscale, another VPN, or a trusted private network.
- Do not expose xrdp directly. It listens on loopback inside the container.
- Protect container logs because they contain the signed access URL.
- Replace the self-signed certificate when users cannot verify the server through another trusted channel.
- Keep browser profiles ephemeral unless persistence is required.
- Protect any persisted Chromium profile as authentication material.
- Stop sessions when the OAuth flow finishes.

With `no-new-privileges` enabled, Chromium cannot use its setuid sandbox. Hatch adds the compatibility flags documented in [INSTALL.md](INSTALL.md). Restrict network exposure and keep the image updated.

## Documentation

| Guide | Purpose |
| --- | --- |
| [CLI guide](docs/CLI.md) | Command behavior, Docker checks, ports, defaults, and examples |
| [Installation and operations](INSTALL.md) | Manual container setup, TLS, Compose, troubleshooting, and security |
| [Product requirements](PRD.md) | Product scope and design intent |

## Development

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch

make setup
make test
make vet
make build
```

Run the Guacamole end-to-end smoke test with:

```bash
make e2e
```

The test builds Hatch, starts a temporary HTTPS container, launches Guacamole through the signed URL, opens the RDP connection, and verifies Chromium starts at the configured URL.

## License

Hatch is available under the [MIT License](LICENSE).
