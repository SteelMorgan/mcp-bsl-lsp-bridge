# Dockerfile для mcp-lsp-bridge с BSL Language Server
FROM alpine:latest

# Устанавливаем Java (OpenJDK 17) и необходимые утилиты
RUN apk add --no-cache \
    openjdk17-jre \
    bash \
    coreutils \
    findutils \
    grep \
    sed

# Создаем пользователя
RUN adduser -D -s /bin/bash user

# Копируем mcp-lsp-bridge
COPY mcp-lsp-bridge-linux /usr/bin/mcp-lsp-bridge
RUN chmod +x /usr/bin/mcp-lsp-bridge

# Создаем директории для конфигурации и кэша
RUN mkdir -p /home/user/.config/mcp-lsp-bridge \
    && mkdir -p /home/user/.local/share/mcp-lsp-bridge/logs \
    && mkdir -p /home/user/.cache/bsl-language-server \
    && mkdir -p /workspace

# Копируем BSL Language Server
COPY bsl-language-server.jar /home/user/bsl-language-server.jar

# Копируем конфигурации
COPY lsp_config.docker.json /home/user/.config/mcp-lsp-bridge/lsp_config.json
COPY mcp_config.docker.json /home/user/.config/mcp-lsp-bridge/mcp_config.json

# Создаем скрипт инициализации для настройки окружения
RUN echo '#!/bin/bash' > /home/user/start.sh && \
    echo 'export JAVA_OPTS="-Xmx4g -Xms1g -XX:+UseG1GC -XX:MaxGCPauseMillis=200"' >> /home/user/start.sh && \
    echo 'export LSP_BRIDGE_LOG_LEVEL=debug' >> /home/user/start.sh && \
    echo 'export WORKSPACE_ROOT=/workspace' >> /home/user/start.sh && \
    echo 'echo "MCP-LSP Bridge starting..."' >> /home/user/start.sh && \
    echo 'echo "Waiting for JSON-RPC input on stdin..."' >> /home/user/start.sh && \
    echo 'exec mcp-lsp-bridge "$@"' >> /home/user/start.sh && \
    chmod +x /home/user/start.sh

# Устанавливаем владельца файлов
RUN chown -R user:user /home/user /workspace

# Переключаемся на пользователя
USER user

# Устанавливаем рабочую директорию
WORKDIR /workspace

# Устанавливаем переменные окружения
ENV JAVA_OPTS="-Xmx4g -Xms1g -XX:+UseG1GC -XX:MaxGCPauseMillis=200"
ENV LSP_BRIDGE_LOG_LEVEL=debug
ENV WORKSPACE_ROOT=/workspace

# Команда по умолчанию
CMD ["/home/user/start.sh"]
