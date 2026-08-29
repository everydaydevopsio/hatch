FROM debian:12-slim AS guacd-builder

ARG GUACAMOLE_VERSION=1.6.0
ARG GUACAMOLE_SERVER_SHA256=8bc45675da96d7b6f39728160181e3d4ff3c08f460f6d26de5805b642bf13f2b

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      freerdp2-dev \
      libcairo2-dev \
      libjpeg62-turbo-dev \
      libossp-uuid-dev \
      libpango1.0-dev \
      libpng-dev \
      libssh2-1-dev \
      libssl-dev \
      libswscale-dev \
      libtelnet-dev \
      libtool-bin \
      libvncserver-dev \
      libwebsockets-dev \
      uuid-dev \
      wget \
    && wget -O /tmp/guacamole-server.tar.gz "https://archive.apache.org/dist/guacamole/${GUACAMOLE_VERSION}/source/guacamole-server-${GUACAMOLE_VERSION}.tar.gz" \
    && echo "${GUACAMOLE_SERVER_SHA256}  /tmp/guacamole-server.tar.gz" | sha256sum -c - \
    && mkdir -p /tmp/guacamole-server \
    && tar -xzf /tmp/guacamole-server.tar.gz -C /tmp/guacamole-server --strip-components=1 \
    && cd /tmp/guacamole-server \
    && CFLAGS="-Wno-error=deprecated-declarations" ./configure --prefix=/usr/local \
    && make -j"$(nproc)" \
    && make install DESTDIR=/opt/guacd-root

FROM guacamole/guacamole:1.6.0 AS guacamole-web

FROM debian:12-slim AS guacamole-jwt-extension

ARG GUAC_JWT_VERSION=1.5.4

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      wget \
    && mkdir -p /opt/guacamole-jwt/extensions /opt/guacamole-jwt/lib \
    && wget -O "/opt/guacamole-jwt/extensions/guacamole-auth-jwt-${GUAC_JWT_VERSION}.jar" \
      "https://github.com/aiden0z/guacamole-auth-jwt/releases/download/v${GUAC_JWT_VERSION}/guacamole-auth-jwt-${GUAC_JWT_VERSION}.jar" \
    && wget -O /opt/guacamole-jwt/lib/jackson-annotations-2.12.7.jar \
      https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-annotations/2.12.7/jackson-annotations-2.12.7.jar \
    && wget -O /opt/guacamole-jwt/lib/jackson-core-2.12.7.jar \
      https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-core/2.12.7/jackson-core-2.12.7.jar \
    && wget -O /opt/guacamole-jwt/lib/jackson-databind-2.12.7.1.jar \
      https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-databind/2.12.7.1/jackson-databind-2.12.7.1.jar \
    && wget -O /opt/guacamole-jwt/lib/jjwt-api-0.12.5.jar \
      https://repo1.maven.org/maven2/io/jsonwebtoken/jjwt-api/0.12.5/jjwt-api-0.12.5.jar \
    && wget -O /opt/guacamole-jwt/lib/jjwt-impl-0.12.5.jar \
      https://repo1.maven.org/maven2/io/jsonwebtoken/jjwt-impl/0.12.5/jjwt-impl-0.12.5.jar \
    && wget -O /opt/guacamole-jwt/lib/jjwt-jackson-0.12.5.jar \
      https://repo1.maven.org/maven2/io/jsonwebtoken/jjwt-jackson/0.12.5/jjwt-jackson-0.12.5.jar \
    && printf '%s\n' \
      "d9a19365f3c47408e9f9ac789f9ce6b3f77033f76d1e028e902590802f37905c  /opt/guacamole-jwt/extensions/guacamole-auth-jwt-${GUAC_JWT_VERSION}.jar" \
      "3cacef714a89f3d68b69fa11263afa55a6aa2fdef1fff93ded22caa16b54687c  /opt/guacamole-jwt/lib/jackson-annotations-2.12.7.jar" \
      "3987a6a335046e226e56b81d69668fb5a91b155ea7fd96b0851adbb7d4ac1ca6  /opt/guacamole-jwt/lib/jackson-core-2.12.7.jar" \
      "3f504cac405ce066d5665ff69541484d5322f35ac7a7ec6104cf86a01008e02d  /opt/guacamole-jwt/lib/jackson-databind-2.12.7.1.jar" \
      "3032441a9875835bf8158075637662a56849f06bb52dd76b0a029d14ef345b03  /opt/guacamole-jwt/lib/jjwt-api-0.12.5.jar" \
      "35171014f6705954b0ecd3d61ef9a24419325eadb68048fd2a23227412987afa  /opt/guacamole-jwt/lib/jjwt-impl-0.12.5.jar" \
      "1bd2f0607f42c2db2cecee9487744b057eb65d1e6ca37bf5eaf7b74598899010  /opt/guacamole-jwt/lib/jjwt-jackson-0.12.5.jar" \
      | sha256sum -c - \
    && rm -rf /var/lib/apt/lists/*

FROM debian:12-slim

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    RDP_USER=oauth \
    HATCH_HTTPS_PORT=443 \
    HATCH_START_URL=about:blank \
    HATCH_MAC_SHORTCUTS=1 \
    HATCH_GUAC_LAUNCH_TTL_SECONDS=43200 \
    CHROMIUM_EXTRA_FLAGS="" \
    CATALINA_HOME=/usr/local/tomcat \
    GUACAMOLE_HOME=/etc/guacamole \
    GUACD_HOSTNAME=127.0.0.1 \
    GUACD_PORT=4822 \
    WEBAPP_CONTEXT=guacamole \
    PATH=/usr/local/tomcat/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    LD_LIBRARY_PATH=/usr/local/lib

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      chromium \
      chromium-sandbox \
      dbus-x11 \
      default-jre-headless \
      fonts-liberation \
      libcairo2 \
      libfreerdp-client2-2 \
      libfreerdp2-2 \
      libjpeg62-turbo \
      libossp-uuid16 \
      libpango-1.0-0 \
      libpangocairo-1.0-0 \
      libpng16-16 \
      libssh2-1 \
      libswscale6 \
      libtelnet2 \
      libvncclient1 \
      libwebsockets17 \
      libwinpr2-2 \
      locales \
      nginx-light \
      openbox \
      openssl \
      procps \
      supervisor \
      xbindkeys \
      xdotool \
      x11-xserver-utils \
      xorg \
      xorgxrdp \
      xrdp \
      xterm \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/xrdp /var/log/supervisor /etc/guacamole /etc/hatch/tls \
    && chmod 0755 /run/xrdp

RUN adduser xrdp ssl-cert || true

COPY --from=guacamole-web /opt/guacamole /opt/guacamole
COPY --from=guacamole-web /usr/local/tomcat /usr/local/tomcat
COPY --from=guacamole-jwt-extension /opt/guacamole-jwt/extensions /etc/guacamole/extensions
COPY --from=guacamole-jwt-extension /opt/guacamole-jwt/lib /etc/guacamole/lib
COPY --from=guacd-builder /opt/guacd-root/usr/local /usr/local
COPY --from=guacd-builder /opt/guacd-root/usr/lib/x86_64-linux-gnu/freerdp2 /usr/lib/x86_64-linux-gnu/freerdp2

COPY config/supervisord.conf /etc/supervisor/conf.d/hatch.conf
COPY config/startwm.sh /usr/local/bin/hatch-startwm
COPY config/chromium-launch.sh /usr/local/bin/hatch-chromium
COPY config/login-shell.sh /usr/local/bin/hatch-login-shell
COPY scripts/guacamole-config.sh /usr/local/bin/hatch-guacamole-config
COPY scripts/entrypoint.sh /usr/local/bin/hatch-entrypoint
COPY scripts/healthcheck.sh /usr/local/bin/hatch-healthcheck

RUN ldconfig \
    && rm -f /etc/nginx/sites-enabled/default /etc/nginx/conf.d/default.conf \
    && chmod +x /usr/local/bin/hatch-entrypoint /usr/local/bin/hatch-startwm /usr/local/bin/hatch-chromium /usr/local/bin/hatch-login-shell /usr/local/bin/hatch-guacamole-config /usr/local/bin/hatch-healthcheck \
    && cp /etc/xrdp/startwm.sh /etc/xrdp/startwm.sh.dist \
    && printf '#!/bin/sh\nexec /usr/local/bin/hatch-startwm\n' > /etc/xrdp/startwm.sh \
    && chmod +x /etc/xrdp/startwm.sh \
    && sed -i 's/^port=3389$/port=tcp:\/\/127.0.0.1:3389/' /etc/xrdp/xrdp.ini

EXPOSE 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/usr/local/bin/hatch-healthcheck"]

ENTRYPOINT ["/usr/local/bin/hatch-entrypoint"]
CMD ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
