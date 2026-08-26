# Hatch: Installation and Operations Guide

Hatch gives a headless Linux server a small Chromium desktop over RDP. It is designed for interactive OAuth flows where a CLI, MCP server, or agent starts a callback listener such as `http://127.0.0.1:8765/callback`.

## Architecture

```text
Mac / PC / iPad
      |
      | RDP over Tailscale, VPN, or SSH
      v
Linux server
  |
  +-- MCP/CLI --> 127.0.0.1:<callback-port>
  |
  +-- Docker --network=host
        |
        +-- xrdp -> Xorg -> Openbox -> Chromium
                         |
                         +--> 127.0.0.1:<callback-port>
```

Host networking is essential. Chromium shares the Linux host network namespace, so a redirect to loopback reaches the OAuth callback process running directly on the server.

## Requirements

- Linux host with Docker Engine
- Docker Compose v2
- About 1 GB free memory while Chromium runs
- Private connectivity through Tailscale, another VPN, or SSH
- An RDP client

## Install

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch
docker build -t hatch:local .
```

Generate an RDP password before starting the container:

```bash
RDP_PASSWORD="$(openssl rand -hex 24)"
printf 'RDP user: oauth\nRDP password: %s\n' "$RDP_PASSWORD"
```

Treat the password as a secret.

## Start and stop

```bash
docker run -d \
  --name hatch \
  --network host \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  -e RDP_PASSWORD="$RDP_PASSWORD" \
  hatch:local
docker ps --filter name=hatch
docker logs -f hatch
```

Stop Hatch with:

```bash
docker stop hatch
docker rm hatch
```

## Secure RDP access

Do not expose TCP/3389 directly to the public Internet.

With Tailscale, connect your RDP client to the server's Tailscale address on port 3389. Restrict the port with Tailscale grants/ACLs and the host firewall.

An SSH tunnel is another option:

```bash
ssh -L 13389:127.0.0.1:3389 user@server
```

Then point your RDP client at `127.0.0.1:13389`.

## Complete an OAuth login

Start the OAuth flow in your normal SSH shell:

```bash
my-mcp-server login
```

When it prints an authorization URL and waits for its local callback, connect to Hatch using RDP. Chromium starts automatically. Paste the authorization URL into Chromium and complete authentication.

The provider can redirect to a URL such as:

```text
http://127.0.0.1:8765/callback?code=...
```

Because Hatch uses `network_mode: host`, that request reaches the listener on the Linux host. The callback port does not need to be exposed publicly.

## Why bridge networking is not used

With normal Docker bridge networking, container `127.0.0.1` and host `127.0.0.1` are different network namespaces. That breaks OAuth clients which require a loopback redirect. Do not replace `network_mode: host` with a simple `ports:` mapping for this use case.

## Chromium sandbox

Hatch keeps Chromium's sandbox enabled. If an unusually restrictive host prevents Chromium from starting, diagnose the host first. As a last resort, set this in `.env`:

```dotenv
CHROMIUM_EXTRA_FLAGS=--no-sandbox
```

Disabling the sandbox reduces browser security.

## Browser persistence

Hatch is disposable by default. Browser cookies and sessions disappear when the container is replaced. If persistent sessions are required, mount `/home/oauth/.config/chromium` as a Docker volume. Persistent browser profiles contain sensitive authentication material and must be protected accordingly.

## Troubleshooting

Check Hatch:

```bash
docker compose ps
docker compose logs hatch
docker exec hatch ps aux
```

Verify host networking:

```bash
docker inspect hatch --format '{{.HostConfig.NetworkMode}}'
```

The result should be `host`.

Verify the OAuth listener from the Linux host:

```bash
ss -lntp | grep 8765
```

From the xterm inside Hatch, test it with:

```bash
curl -v http://127.0.0.1:8765/
```

A protocol-specific `400` or `404` can still prove connectivity. `Connection refused` means nothing is listening on that address and port.

## Updating

```bash
git pull
docker build --pull -t hatch:local .
docker stop hatch
docker rm hatch
docker run -d \
  --name hatch \
  --network host \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  -e RDP_PASSWORD="$RDP_PASSWORD" \
  hatch:local
```

Rebuild regularly so the image receives Chromium and Debian security updates.

## Docker Compose option

If you prefer Docker Compose:

```bash
cp .env.example .env
```

Edit `.env` and set a long random `RDP_PASSWORD`, then start Hatch:

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f hatch
```

Stop Hatch with:

```bash
docker compose down
```

## GitHub Container Registry

The included `.github/workflows/docker.yml` builds the image on pull requests and publishes it to GHCR on pushes to `main` and version tags. The published image name is:

```text
ghcr.io/everydaydevopsio/hatch
```

## Recommended operating model

Use Hatch only over a private network or SSH tunnel. Keep its Chromium profile ephemeral. Use a long random RDP password. Start Hatch when interactive authentication is needed and stop it afterward. OAuth access and refresh tokens should remain in the MCP application's normal credential store rather than in Hatch.
