# Hatch

A small Dockerized browser desktop for completing interactive OAuth flows on a headless Linux server.

Hatch runs Chromium behind Apache Guacamole and serves the desktop over HTTPS from the container. The internal path is:

```text
Browser
  -> HTTPS on container port 443
  -> nginx
  -> Guacamole on 127.0.0.1:8080
  -> guacd on 127.0.0.1:4822
  -> xrdp on 127.0.0.1:3389
  -> Openbox + Chromium
```

The container generates a self-signed HTTPS certificate, a random desktop password, and a Guacamole JWT signing secret at startup unless you provide your own values.

## Go CLI

Hatch now includes a Go CLI that manages Hatch containers through the Docker Engine API.

Build it with:

```bash
make deps
make build
```

`make build` builds the local `hatch:local` container image first, then writes the CLI binary to `bin/hatch`.
For a full local development prerequisite check, including Docker and E2E test tools, run `make setup`.

Check prerequisites and initialize the public hostname with:

```bash
hatch init
hatch init devbox.tailnet.ts.net
```

The hostname is stored in:

```text
~/.config/hatch/hatch.yaml
```

The config stores the public hostname and, optionally, an HTTPS port. `hatch init devbox.tailnet.ts.net:8443` is equivalent to `hatch init devbox.tailnet.ts.net 8443`. If the port is omitted, each `hatch open` command uses an available dynamic port.

Launch a browser desktop directly at a URL:

```bash
hatch open 'https://example.com/oauth/authorize?...'
```

To force a specific HTTPS port for one session, use:

```bash
hatch open --port 8443 'https://example.com/oauth/authorize?...'
```

If the requested port is already in use, Hatch exits with an error instead of selecting a different port.

The CLI checks that Docker is installed and that the Docker daemon responds before any Docker-dependent operation. If Docker is missing, commands tell the user to run `hatch init`; `hatch init` then provides installation instructions for macOS, Windows, and Linux. On Linux it detects common distributions from `/etc/os-release`. If Docker is installed but stopped, Hatch prints platform-specific startup instructions immediately.

Additional commands:

```bash
hatch list
hatch stop <session>
hatch stop --all
```

See [docs/CLI.md](docs/CLI.md) for details.

## Quick Start Without the CLI

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch

docker build -t hatch:local .

docker run -d \
  --name hatch \
  -p 8443:443 \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  hatch:local

docker logs hatch
```

Copy the generated `Hatch access URL` from:

```bash
docker logs hatch
```

If the printed URL contains `<host>`, replace it with your server name or IP and adjust the port if you mapped container port `443` to a different host port. Open:

```text
https://<server>:8443/hatch/?token=<jwt>
```

Accept the self-signed certificate warning. Hatch starts a Guacamole session with the signed JWT and opens the browser desktop without a Guacamole username/password prompt. Treat the access URL and container logs as sensitive because the bearer token grants desktop access until it expires. Direct visits to `/guacamole/` without a token land on the Hatch page instead of the Guacamole login page. The generated RDP credentials are written inside the container at `/var/log/hatch/rdp-credentials.log` for operator recovery.

### Pasting Text from iPad

On an iPad, Guacamole hides its side menu while the remote desktop is open. To paste text into the Hatch desktop:

1. Swipe from the left edge of the Guacamole screen toward the right to open the Guacamole side menu.
2. Paste or type the text into the Guacamole clipboard text box.
3. Tap back into the remote Chromium desktop.
4. Paste in Chromium with the on-screen keyboard shortcut, a hardware keyboard shortcut, or the browser text field's paste action.

Stop Hatch with:

```bash
docker stop hatch
docker rm hatch
```

## OAuth Callback Mode

Some OAuth tools start a local callback listener such as:

```text
http://127.0.0.1:8765/oauth/callback
```

If Chromium inside Hatch must reach a listener running on the Docker host at `127.0.0.1`, run Hatch with Docker host networking. With host networking, Docker ignores `-p`, so choose the HTTPS listener port inside the container:

```bash
docker run -d \
  --name hatch \
  --network host \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  -e HATCH_HTTPS_PORT=8443 \
  hatch:local
```

Open:

```text
https://<server>:8443/hatch/?token=<jwt-from-docker-logs>
```

The RDP service is bound to loopback inside the container. Use the HTTPS Guacamole endpoint instead of connecting an RDP client directly.

## Configuration

Common environment variables:

```text
RDP_USER=oauth
RDP_PASSWORD=
HATCH_GUAC_JWT_SECRET=
HATCH_GUAC_LAUNCH_TTL_SECONDS=43200
HATCH_HTTPS_PORT=443
HATCH_START_URL=about:blank
HATCH_MAC_SHORTCUTS=1
CHROMIUM_EXTRA_FLAGS=
HATCH_TLS_CERT=/etc/hatch/tls/hatch.crt
HATCH_TLS_KEY=/etc/hatch/tls/hatch.key
HATCH_TLS_CN=hatch.local
HATCH_TLS_DAYS=365
```

Leave `RDP_PASSWORD` blank to generate a random password. Leave `HATCH_GUAC_JWT_SECRET` blank to generate a per-container signing secret for Guacamole JWT authentication. `HATCH_GUAC_LAUNCH_TTL_SECONDS` controls how long the generated Hatch access URL remains valid.

`HATCH_MAC_SHORTCUTS=1` maps remote `Super`/Mac-style shortcuts such as `Cmd+V`, `Cmd+C`, and `Cmd+L` to the Linux `Ctrl` shortcuts expected by Chromium. Set it to `0` to disable this shortcut bridge.

To use your own certificate, mount the certificate and key into the container and set `HATCH_TLS_CERT` and `HATCH_TLS_KEY`.

## Docker Compose Option

```bash
cp .env.example .env
docker compose up -d --build
docker compose logs hatch
```

The compose file uses host networking and listens on `${HATCH_HTTPS_PORT:-8443}` directly on the host. Stop it with:

```bash
docker compose down
```

The compose file uses host networking for OAuth callback mode. In this mode Chromium's `127.0.0.1` is the Linux host loopback, so dynamic callback URLs such as `http://127.0.0.1:40397/callback/...` can reach the listener started by the OAuth tool. Docker ignores compose port mappings when host networking is enabled, so Hatch listens on `${HATCH_HTTPS_PORT:-8443}` directly on the host.

## E2E Guacamole Test

Run the smoke test from the repository root:

```bash
scripts/e2e-guacamole.sh
```

The test builds Hatch, starts one temporary container, waits for HTTPS health, confirms direct `/guacamole/` access redirects to Hatch, launches Guacamole through the generated JWT-backed Hatch URL with Playwright, opens the Hatch connection, and verifies Chromium starts at `https://www.google.com`.

Required host tools:

- Docker
- `nc`
- Node.js
- npm

## Project Layout

```text
.
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── cmd/hatch/
│   ├── main.go
│   └── main_test.go
├── config/
│   ├── chromium-launch.sh
│   ├── login-shell.sh
│   ├── startwm.sh
│   └── supervisord.conf
├── docs/
│   └── CLI.md
├── scripts/
│   ├── e2e-guacamole.sh
│   ├── entrypoint.sh
│   ├── guacamole-config.sh
│   └── healthcheck.sh
└── .github/workflows/
    ├── docker.yml
    └── go.yml
```

## Supported Host

The Hatch container targets Linux Docker Engine hosts for OAuth callback mode. The Go CLI itself provides Docker prerequisite guidance on macOS, Linux, and Windows. Docker Desktop host networking is virtualized differently from a native Linux Docker Engine host, so callback behavior involving host loopback should be verified on desktop operating systems.

## Security Notes

Do not publish direct RDP access. Hatch exposes HTTPS for browser access, and xrdp listens on `127.0.0.1:3389` inside the container.

The default certificate is self-signed. Use a reverse proxy, load balancer, or mounted certificate files if you need a publicly trusted certificate. Certbot HTTP-01 validation requires public port 80, while DNS-01 can issue certificates without opening port 80.

When the container is started with `--security-opt no-new-privileges:true`, Hatch automatically adds Chromium's `--no-sandbox` compatibility flag because the setuid sandbox cannot run under that kernel setting. Hatch also adds Chromium's `--test-type` flag in that mode to suppress Chromium's unsupported command-line flag warning.

Browser persistence is off by default. If persistent browser sessions are required, mount `/home/oauth/.config/chromium` as a Docker volume and protect it as sensitive authentication material.

See [INSTALL.md](INSTALL.md) for operational details and troubleshooting.
