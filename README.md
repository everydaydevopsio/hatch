# Hatch

A small Dockerized RDP desktop for completing interactive OAuth flows on a headless Linux server.

It runs:

- Chromium
- Openbox
- xrdp + xorgxrdp
- a minimal X11 environment

The container is intentionally run with **Docker host networking**. That makes Chromium's `127.0.0.1` the Linux host's loopback interface, which is the important part for CLI and MCP OAuth flows that start a callback listener such as:

```text
http://127.0.0.1:8765/oauth/callback
```

## How it works

```text
Mac / iPad / workstation
        |
        | RDP over Tailscale, VPN, or SSH
        v
+-----------------------------------------+
| Headless Linux server                   |
|                                         |
|  +-----------------------------------+  |
|  | Docker: hatch                     |  |
|  | xrdp -> Openbox -> Chromium       |  |
|  +----------------+------------------+  |
|                   | host networking     |
|                   v                     |
|             127.0.0.1                   |
|                   |                     |
|                   v                     |
|        MCP/CLI OAuth callback           |
+-----------------------------------------+
```

The MCP server or CLI runs normally on the Linux host. Only the GUI browser runs in Docker.

## Quick start

```bash
git clone https://github.com/everydaydevopsio/hatch.git
cd hatch

docker build -t hatch:local .

docker run -d \
  --name hatch \
  --network host \
  --restart unless-stopped \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER=oauth \
  hatch:local

docker logs hatch
```

Connect an RDP client to:

```text
<server-private-ip>:3389
```

Use the username and generated password printed by `docker logs hatch`. The credentials are also written inside the container at `/var/log/hatch/rdp-credentials.log`.

Stop Hatch with:

```bash
docker stop hatch
docker rm hatch
```

Use a Tailscale/VPN address or an SSH tunnel. **Do not expose TCP/3389 directly to the public Internet.**

See [INSTALL.md](INSTALL.md) for the full installation, security, OAuth workflow, troubleshooting, and production recommendations.

## Typical OAuth flow

SSH to the server and start the application that needs authorization:

```bash
ssh dev-server
my-mcp-server login
```

It may print an authorization URL and then wait on a callback:

```text
Waiting for OAuth callback at http://127.0.0.1:8765/callback
```

Connect to Hatch over RDP, open Chromium, paste the authorization URL, and sign in.

The provider redirects Chromium to `127.0.0.1:8765`. Because the container uses host networking, that request reaches the callback listener running on the Linux host.

## Docker Compose option

```bash
cp .env.example .env
```

Edit `.env` only if you want to override defaults, then start Hatch:

```bash
docker compose up -d --build
docker compose ps
docker compose logs hatch
```

Stop Hatch with:

```bash
docker compose down
```

## Project layout

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
│   ├── entrypoint.sh
│   └── healthcheck.sh
└── .github/workflows/docker.yml
```

## Design constraints

### Host networking is deliberate

Do not replace `network_mode: host` with `ports: - 3389:3389` if the OAuth callback uses loopback addresses. Under bridge networking, Chromium's `127.0.0.1` belongs to the container and no longer points at the MCP service on the host.

### The MCP server stays outside this image

This keeps the browser utility generic. Any host process can use it, regardless of whether the OAuth client is an MCP server, Codex tool, CLI, Python process, Node application, or another agent.

### Browser persistence is off by default

The default container does not persist Chromium's profile. This reduces the amount of session/cookie material left behind. If persistent browser sessions are required, mount `/home/oauth/.config/chromium` as a Docker volume and protect it as sensitive authentication material.

## Supported host

This project targets Linux hosts running Docker Engine. Docker Desktop behaves differently because host networking is virtualized and is not the intended deployment environment for this solution.

## Base image

The image currently uses Debian 13 and Debian packages for Chromium, xrdp, xorgxrdp, Openbox, and Xorg.
