# Hatch: Installation and Operations Guide

Hatch gives a headless Linux server a small Chromium desktop in the browser. It is designed for interactive OAuth flows where a CLI, MCP server, or agent starts a callback listener such as `http://127.0.0.1:8765/callback`.

## Architecture

```text
Browser
  |
  | HTTPS
  v
Docker: hatch
  |
  +-- nginx :443
      |
      +-- Guacamole :8080
          |
          +-- guacd :4822
              |
              +-- xrdp 127.0.0.1:3389 -> Xorg -> Openbox -> Chromium
```

For OAuth callback mode, run the container with Docker host networking so Chromium's `127.0.0.1` is the Linux host's loopback interface.

## Requirements

- Linux host with Docker Engine
- About 1 GB free memory while Chromium runs
- Private connectivity through Tailscale, another VPN, or SSH
- A browser that can accept a self-signed certificate, unless you provide your own TLS certificate

## Install

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch
make setup
make build
```

`make build` builds the local `hatch:local` container image and the Go CLI at `bin/hatch`.

Hatch generates an RDP password at container startup when `RDP_PASSWORD` is not set. Guacamole access is launched with a signed JWT instead of a reusable Guacamole username/password.

## Start and Stop

```bash
# Optional: set HATCH_START_URL when Chromium should open a specific page.
docker run -d \
  --name hatch \
  -p 8443:443 \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  -e HATCH_START_URL="https://www.google.com" \
  hatch:local
docker ps --filter name=hatch
docker logs hatch
```

Copy the generated `Hatch access URL` from `docker logs hatch`. If the printed URL contains `<host>`, replace it with your server name or IP and adjust the port if you mapped container port `443` to a different host port.

Open `https://<server>:8443/hatch/?token=<jwt>`. Hatch starts a Guacamole session with the signed JWT and opens the browser desktop without a Guacamole username/password prompt. Treat the access URL and container logs as sensitive because the bearer token grants desktop access until it expires. Direct visits to `/guacamole/` without a token land on the Hatch page instead of the Guacamole login page.

### Pasting Text from iPad

On an iPad, Guacamole hides its side menu while the remote desktop is open. To paste text into the Hatch desktop:

1. Swipe from the left edge of the Guacamole screen toward the right to open the Guacamole side menu.
2. Paste or type the text into the Guacamole clipboard text box.
3. Tap back into the remote Chromium desktop.
4. Paste in Chromium with the on-screen keyboard shortcut, a hardware keyboard shortcut, or the browser text field's paste action.

The generated RDP credentials are also written inside the container:

```bash
docker exec hatch cat /var/log/hatch/rdp-credentials.log
```

Stop Hatch with:

```bash
docker stop hatch
docker rm hatch
```

## OAuth Callback Mode

Start the OAuth flow in your normal SSH shell:

```bash
my-mcp-server login
```

When it prints an authorization URL and waits for its local callback, open Hatch in your browser. Paste the authorization URL into Chromium and complete authentication.

If the provider redirects to a URL such as:

```text
http://127.0.0.1:8765/callback?code=...
```

run Hatch with host networking:

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

Open the generated `Hatch access URL` from `docker logs hatch`. Docker host networking ignores `-p`, so `HATCH_HTTPS_PORT` controls the host port in this mode.

## TLS

By default Hatch generates a self-signed certificate in `/etc/hatch/tls`. Set these variables to customize it:

```dotenv
HATCH_TLS_CN=hatch.local
HATCH_TLS_DAYS=365
```

To provide your own certificate:

```bash
docker run -d \
  --name hatch \
  -p 8443:443 \
  -v /srv/hatch/tls:/tls:ro \
  -e HATCH_TLS_CERT=/tls/fullchain.pem \
  -e HATCH_TLS_KEY=/tls/privkey.pem \
  hatch:local
```

Certbot HTTP-01 validation requires public port 80. Use DNS-01 validation if you need certificate issuance without opening port 80, or terminate TLS in a reverse proxy that already owns certificate renewal.

## Configuration

```dotenv
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

Leave `RDP_PASSWORD` blank to generate a password at startup. Leave `HATCH_GUAC_JWT_SECRET` blank to generate a per-container signing secret for Guacamole JWT authentication. `HATCH_GUAC_LAUNCH_TTL_SECONDS` controls how long the generated Hatch access URL remains valid.

Set `HATCH_START_URL` to the page Chromium should open when the remote desktop session starts. The default is `about:blank`.

`HATCH_MAC_SHORTCUTS=1` maps remote `Super`/Mac-style shortcuts such as `Cmd+V`, `Cmd+C`, and `Cmd+L` to the Linux `Ctrl` shortcuts expected by Chromium. Set it to `0` to disable this shortcut bridge.

## Docker Compose Option

```bash
cp .env.example .env
# Optional: edit HATCH_START_URL in .env before starting.
docker compose up -d --build
docker compose ps
docker compose logs hatch
```

The compose file uses host networking and listens on `${HATCH_HTTPS_PORT:-8443}` directly on the host.

Stop Hatch with:

```bash
docker compose down
```

The compose file uses host networking for OAuth callback mode. Open the generated `Hatch access URL` from `docker compose logs hatch` unless you set a different `HATCH_HTTPS_PORT`. Hatch listens on `HATCH_HTTPS_PORT` directly on the host. This is required for callback URLs such as `http://127.0.0.1:40397/callback/...` because Chromium is running inside the Hatch container and must see the Linux host's loopback interface.

## Troubleshooting

Check Hatch:

```bash
docker ps --filter name=hatch
docker logs hatch
docker exec hatch ps aux
```

Verify the HTTPS endpoint:

```bash
curl -kI https://127.0.0.1:8443/
curl -kI https://127.0.0.1:8443/guacamole/
```

Verify listeners inside the container:

```bash
docker exec hatch ss -ltnp
```

Expected internal ports:

```text
0.0.0.0:443          nginx HTTPS
127.0.0.1:4822      guacd
127.0.0.1:3389      xrdp
*:8080              Guacamole Tomcat
```

Verify the OAuth listener from the Linux host:

```bash
ss -lntp | grep 8765
```

From an xterm inside Hatch, test it with:

```bash
curl -v http://127.0.0.1:8765/
```

A protocol-specific `400` or `404` can still prove connectivity. `Connection refused` means nothing is listening on that address and port.

## Chromium Sandbox

If the container is started with `--security-opt no-new-privileges:true`, Chromium cannot use its setuid sandbox, so Hatch automatically adds `--no-sandbox` for that session. Hatch also adds Chromium's `--test-type` flag in that mode to suppress Chromium's unsupported command-line flag warning.

If an unusually restrictive host still prevents Chromium from starting, diagnose the host first. As a last resort, set:

```dotenv
CHROMIUM_EXTRA_FLAGS=--no-sandbox
```

Disabling the sandbox reduces browser security.

## Browser Persistence

Hatch is disposable by default. Browser cookies and sessions disappear when the container is replaced. If persistent sessions are required, mount `/home/oauth/.config/chromium` as a Docker volume. Persistent browser profiles contain sensitive authentication material and must be protected accordingly.

## Updating

```bash
git pull
make build
docker stop hatch
docker rm hatch
# Optional: set HATCH_START_URL when Chromium should open a specific page.
docker run -d \
  --name hatch \
  -p 8443:443 \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  -e HATCH_START_URL="https://www.google.com" \
  hatch:local
docker logs hatch
```

Rebuild regularly so the image receives Chromium, Guacamole, and Debian security updates.

## E2E Guacamole Test

```bash
scripts/e2e-guacamole.sh
```

It builds Hatch, starts one temporary HTTPS container, confirms direct `/guacamole/` access redirects to Hatch, launches Guacamole through the generated JWT-backed Hatch URL with Playwright, opens the Hatch RDP connection, sets `HATCH_START_URL` from `${HATCH_E2E_URL:-https://www.google.com}`, and verifies Chromium opens that value.

## GitHub Container Registry

The included `.github/workflows/docker.yml` builds the image on pull requests and publishes it to GHCR on pushes to `main` and version tags. The published image name is:

```text
ghcr.io/everydaydevopsio/hatch
```

## Recommended Operating Model

Expose only HTTPS. Keep direct RDP private inside the container. Use host networking only when the browser must reach host loopback OAuth callbacks. Keep the Chromium profile ephemeral unless persistence is required and the profile volume is protected as sensitive authentication material.
