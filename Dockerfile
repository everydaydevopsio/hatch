FROM debian:13-slim

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    RDP_USER=oauth \
    CHROMIUM_EXTRA_FLAGS=""

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      chromium \
      chromium-sandbox \
      dbus-x11 \
      fonts-liberation \
      locales \
      openbox \
      procps \
      supervisor \
      x11-xserver-utils \
      xorg \
      xorgxrdp \
      xrdp \
      xterm \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/xrdp /var/log/supervisor \
    && chmod 0755 /run/xrdp

RUN adduser xrdp ssl-cert || true

COPY config/supervisord.conf /etc/supervisor/conf.d/hatch.conf
COPY config/startwm.sh /usr/local/bin/hatch-startwm
COPY config/chromium-launch.sh /usr/local/bin/hatch-chromium
COPY config/login-shell.sh /usr/local/bin/hatch-login-shell
COPY scripts/entrypoint.sh /usr/local/bin/hatch-entrypoint
COPY scripts/healthcheck.sh /usr/local/bin/hatch-healthcheck

RUN chmod +x /usr/local/bin/hatch-entrypoint /usr/local/bin/hatch-startwm /usr/local/bin/hatch-chromium /usr/local/bin/hatch-login-shell /usr/local/bin/hatch-healthcheck \
    && cp /etc/xrdp/startwm.sh /etc/xrdp/startwm.sh.dist \
    && printf '#!/bin/sh\nexec /usr/local/bin/hatch-startwm\n' > /etc/xrdp/startwm.sh \
    && chmod +x /etc/xrdp/startwm.sh

EXPOSE 3389

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/usr/local/bin/hatch-healthcheck"]

ENTRYPOINT ["/usr/local/bin/hatch-entrypoint"]
CMD ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
