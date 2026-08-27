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

The container generates a self-signed HTTPS certificate and a random desktop password at startup unless you provide your own values.

## Quick Start

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

Open:

```text
https://<server>:8443/guacamole/
```

Accept the self-signed certificate warning, then sign in with the `Guacamole user` and `Guacamole password` printed by `docker logs hatch`. The generated RDP credentials are also written inside the container at `/var/log/hatch/rdp-credentials.log`.

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
https://<server>:8443/guacamole/
```

The RDP service is bound to loopback inside the container. Use the HTTPS Guacamole endpoint instead of connecting an RDP client directly.

## Configuration

Common environment variables:

```text
RDP_USER=oauth
RDP_PASSWORD=
GUAC_USER=
GUAC_PASSWORD=
HATCH_HTTPS_PORT=443
HATCH_START_URL=about:blank
HATCH_MAC_SHORTCUTS=1
CHROMIUM_EXTRA_FLAGS=
HATCH_TLS_CERT=/etc/hatch/tls/hatch.crt
HATCH_TLS_KEY=/etc/hatch/tls/hatch.key
HATCH_TLS_CN=hatch.local
HATCH_TLS_DAYS=365
```

Leave `RDP_PASSWORD` blank to generate a random password. Leave `GUAC_USER` and `GUAC_PASSWORD` blank to reuse the RDP credentials for the Guacamole login.

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

The test builds Hatch, starts one temporary container, waits for HTTPS health, logs into Guacamole with Playwright, opens the Hatch connection, and verifies Chromium starts at `https://www.google.com`.

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
├── config/
│   ├── chromium-launch.sh
│   ├── login-shell.sh
│   ├── startwm.sh
│   └── supervisord.conf
├── scripts/
│   ├── e2e-guacamole.sh
│   ├── entrypoint.sh
│   ├── guacamole-config.sh
│   └── healthcheck.sh
└── .github/workflows/docker.yml
```

## Supported Host

This project targets Linux hosts running Docker Engine. Docker Desktop behaves differently because host networking is virtualized and is not the intended deployment environment for OAuth callback mode.

## Security Notes

Do not publish direct RDP access. Hatch exposes HTTPS for browser access, and xrdp listens on `127.0.0.1:3389` inside the container.

The default certificate is self-signed. Use a reverse proxy, load balancer, or mounted certificate files if you need a publicly trusted certificate. Certbot HTTP-01 validation requires public port 80, while DNS-01 can issue certificates without opening port 80.

When the container is started with `--security-opt no-new-privileges:true`, Hatch automatically adds Chromium's `--no-sandbox` compatibility flag because the setuid sandbox cannot run under that kernel setting. Hatch also adds Chromium's `--test-type` flag in that mode to suppress Chromium's unsupported command-line flag warning.

Browser persistence is off by default. If persistent browser sessions are required, mount `/home/oauth/.config/chromium` as a Docker volume and protect it as sensitive authentication material.

See [INSTALL.md](INSTALL.md) for operational details and troubleshooting.
