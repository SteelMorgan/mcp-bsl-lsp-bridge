FROM golang:1.24.2-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build mcp-lsp-bridge
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/mcp-lsp-bridge .

# Build lsp-session-manager daemon
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/lsp-session-manager ./cmd/lsp-session-manager


# === Separate stage for BSL LS download (cached independently) ===
FROM debian:bookworm-slim AS bsl-ls-downloader

RUN apt-get update \
  && apt-get install -y --no-install-recommends wget ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Download BSL Language Server from GitHub releases
# Use "latest" to always get the newest version, or specify version like "0.28.1"
ARG BSL_LS_VERSION=latest
RUN mkdir -p /opt/bsl-ls \
  && if [ "$BSL_LS_VERSION" = "latest" ]; then \
       BSL_LS_URL=$(wget -qO- https://api.github.com/repos/1c-syntax/bsl-language-server/releases/latest | grep -o '"browser_download_url": *"[^"]*-exec.jar"' | head -1 | cut -d'"' -f4); \
     else \
       BSL_LS_URL="https://github.com/1c-syntax/bsl-language-server/releases/download/v${BSL_LS_VERSION}/bsl-language-server-${BSL_LS_VERSION}-exec.jar"; \
     fi \
  && echo "Downloading BSL LS from: $BSL_LS_URL" \
  && wget -q -O /opt/bsl-ls/bsl-language-server.jar "$BSL_LS_URL" \
  && echo "BSL LS downloaded successfully"


# === Final stage ===
FROM debian:bookworm-slim

# Install xz-utils first for unpacking s6-overlay, then other packages
# Also install locales for UTF-8 support (critical for Cyrillic filenames and content)
RUN apt-get update \
  && apt-get install -y --no-install-recommends xz-utils ca-certificates openjdk-17-jre-headless procps netcat-openbsd locales \
  && rm -rf /var/lib/apt/lists/* \
  && sed -i '/ru_RU.UTF-8/s/^# //g' /etc/locale.gen \
  && sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
  && locale-gen

# Install s6-overlay for process supervision
ARG S6_OVERLAY_VERSION=3.1.6.2
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz \
  && tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz \
  && rm /tmp/s6-overlay-*.tar.xz

# Create non-root user (match existing conventions)
RUN useradd -m -s /bin/sh user

# Copy BSL LS from downloader stage (cached independently from Go binaries)
RUN mkdir -p /opt/bsl-ls
COPY --from=bsl-ls-downloader /opt/bsl-ls/bsl-language-server.jar /opt/bsl-ls/bsl-language-server.jar
RUN chown -R user:user /opt/bsl-ls

# Copy built binaries (changes here won't trigger BSL LS re-download)
COPY --from=build /out/mcp-lsp-bridge /usr/bin/mcp-lsp-bridge
COPY --from=build /out/lsp-session-manager /usr/bin/lsp-session-manager

# Store version info for runtime checks
RUN java -jar /opt/bsl-ls/bsl-language-server.jar --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' > /opt/bsl-ls/VERSION || echo "unknown" > /opt/bsl-ls/VERSION

# Default locations used by the bridge
RUN mkdir -p /home/user/.config/mcp-lsp-bridge /home/user/.local/share/mcp-lsp-bridge/logs \
  && chown -R user:user /home/user/.config /home/user/.local

COPY docker/lsp_config.json /home/user/.config/mcp-lsp-bridge/lsp_config.json
COPY docker/bsl-ls.json /home/user/.config/mcp-lsp-bridge/bsl-ls.json

# Also copy config to /etc for root user access (docker exec runs as root)
RUN mkdir -p /etc/mcp-lsp-bridge
COPY docker/lsp_config.json /etc/mcp-lsp-bridge/lsp_config.json
COPY docker/bsl-ls.json /etc/mcp-lsp-bridge/bsl-ls.json

# === s6-overlay service definitions ===

# lsp-proxy service - starts BSL LS and proxies TCP to stdio
COPY docker/s6-rc.d/ /etc/s6-overlay/s6-rc.d/

# Fix permissions and line endings
RUN find /etc/s6-overlay/s6-rc.d -type f -exec sed -i 's/\r$//' {} \; \
  && find /etc/s6-overlay/s6-rc.d -name "run" -exec chmod +x {} \; \
  && find /etc/s6-overlay/s6-rc.d -name "finish" -exec chmod +x {} \;

# Environment variables
ENV S6_KEEP_ENV=1
ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2
ENV S6_CMD_WAIT_FOR_SERVICES_MAXTIME=0

# UTF-8 locale for Cyrillic support
ENV LANG=ru_RU.UTF-8
ENV LANGUAGE=ru_RU:ru
ENV LC_ALL=ru_RU.UTF-8

WORKDIR /home/user

# s6-overlay as init - manages lsp-proxy daemon
ENTRYPOINT ["/init"]

# Container stays alive, MCP calls via `docker exec ... mcp-lsp-bridge`
CMD ["sleep", "infinity"]
