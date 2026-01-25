# MCP LSP Bridge — Полная инструкция для ИИ-агента

> **Назначение**: Автономная установка, настройка и использование MCP LSP Bridge.
> **Принцип**: Агент делает всё сам, без участия человека где возможно.

---

## Часть 1: Установка и настройка

### 1.1 Проверка предусловий

```bash
# Проверить Docker
docker --version

# Проверить docker-compose
docker compose version
```

Если Docker не установлен — сообщи пользователю и останови выполнение.

### 1.2 Создание .env файла

Скопируй `env.example` в `.env` и настрой:

```bash
cp env.example .env
```

**Обязательные параметры для редактирования:**

| Параметр | Описание | Пример |
|----------|----------|--------|
| `MCP_PROJECT_NAME` | Уникальный ID проекта | `my-project` |
| `HOST_PROJECTS_ROOT` | Путь к коду 1С на хосте | `D:/Projects/1C/MyConfig` |
| `WORKSPACE_ROOT` | Путь внутри контейнера | `/projects` или `/projects/src;/projects/ext1` |

**Пример .env:**

```env
MCP_PROJECT_NAME=my-project
MCP_CONTAINER_PREFIX=mcp-lsp
MCP_LSP_BRIDGE_TAG=latest

HOST_PROJECTS_ROOT=D:/Projects/1C/MyConfig
PROJECTS_ROOT=/projects
WORKSPACE_ROOT=/projects

PROJECTS_MOUNT_MODE=rw

MCP_LSP_BSL_JAVA_XMX=6g
MCP_LSP_BSL_JAVA_XMS=2g
MCP_LSP_LOG_LEVEL=error
```

**Несколько каталогов** (основная конфа + расширения):
```env
WORKSPACE_ROOT=/projects/main-config;/projects/extension1;/projects/extension2
```

### 1.3 Сборка и запуск контейнера

```bash
# Сборка образа (первый раз или после обновлений)
docker compose build

# Запуск контейнера
docker compose up -d

# Проверка статуса
docker compose ps
```

**Имя контейнера** формируется как: `${MCP_CONTAINER_PREFIX}-${MCP_PROJECT_NAME}`
Например: `mcp-lsp-my-project`

### 1.4 Проверка работоспособности

```bash
# Проверить что контейнер запущен
docker ps | grep mcp-lsp

# Проверить health (BSL LS готов)
docker inspect --format='{{.State.Health.Status}}' mcp-lsp-my-project

# Тест MCP bridge
docker exec -i mcp-lsp-my-project mcp-lsp-bridge --help
```

Ожидаемый health status: `healthy` (может занять 1-2 минуты при первом запуске).

### 1.5 Настройка MCP клиента (Cursor)

Создай/обнови файл `.cursor/mcp.json` в проекте:

```json
{
  "mcpServers": {
    "lsp-bsl-bridge": {
      "type": "stdio",
      "command": "docker",
      "args": [
        "exec",
        "-i",
        "mcp-lsp-my-project",
        "mcp-lsp-bridge"
      ],
      "env": {}
    }
  }
}
```

**Важно**: Замени `mcp-lsp-my-project` на реальное имя контейнера из `.env`.

### 1.6 Проверка подключения MCP

После настройки используй tool `lsp_status`:

```
lsp_status
```

Ожидаемый ответ:
- `status: connected`
- `indexing: complete` (или прогресс в %)

Если `indexing` в процессе — дождись завершения перед тяжёлыми операциями.

---

## Часть 2: Работа с кодом (Token-Efficient Workflow)

### 2.1 Главный принцип

```
❌ АНТИПАТТЕРН (много токенов):
   symbol_explore(query="МояПроцедура", detail_level="full")
   → Возвращает ВСЁ: код, документацию, references
   
✅ ОПТИМАЛЬНО (экономия ~70%):
   1. project_analysis(analysis_type="workspace_symbols", query="МояПроцедура", limit=5)
      → Только координаты: имя + файл + строка
   2. definition(uri=..., line=..., character=...)
      → Точные координаты определения
   3. get_range_content(uri=..., start_line=..., end_line=...)
      → Только нужный фрагмент кода
```

### 2.2 Доступные Tools

| Tool | Назначение | Токены |
|------|-----------|--------|
| `lsp_status` | Статус LSP, прогресс индексации | Минимальные |
| `project_analysis` | Поиск символов, обзор проекта | Средние |
| `symbol_explore` | Детальный поиск + код + references | **Высокие** |
| `definition` | Координаты определения символа | **Низкие** |
| `get_range_content` | Извлечение кода по координатам | Зависит от диапазона |
| `hover` | Документация/сигнатура символа | Средние |
| `call_hierarchy` | Вызывающие/вызываемые (1 уровень) | Средние |
| `call_graph` | Полный граф вызовов | **Высокие** |
| `document_diagnostics` | Синтаксические ошибки, предупреждения, проверка кода | Зависит от количества |
| `code_actions` | Предложения по исправлению | Низкие |
| `prepare_rename` | Проверка возможности переименования | Низкие |
| `rename` | Переименование (preview/apply) | Средние |
| `did_change_watched_files` | Уведомление LSP об изменениях | Минимальные |

### 2.3 Workflows по задачам

#### Найти и понять процедуру/функцию

```
1. project_analysis
   analysis_type="workspace_symbols"
   query="ИмяПроцедуры"
   limit=5
   → Координаты

2. hover (если нужна документация)
   → Сигнатура + описание

3. get_range_content (если нужен код)
   start_line=N, end_line=N+30
   → Тело процедуры
```

#### Навигация "Go to Definition"

```
1. definition
   uri=..., line=..., character=...
   → Координаты определения

2. get_range_content (при необходимости)
   → Код определения
```

#### Анализ зависимостей

```
1. call_hierarchy (НЕ call_graph!)
   → Кто вызывает + что вызывает (1 уровень)

2. Если нужно глубже — call_graph С ЛИМИТАМИ:
   depth_up=2, depth_down=2, max_nodes=30
```

#### Синтаксический контроль и проверка кода

**`document_diagnostics` — основной инструмент для проверки кода.**

Возвращает:
- Синтаксические ошибки (незакрытые скобки, неизвестные идентификаторы)
- Предупреждения BSL LS (неиспользуемые переменные, deprecated методы)
- Стилистические замечания (форматирование, именование)
- Позиции ошибок (строка, символ, severity)

```
1. document_diagnostics
   uri="file:///projects/.../Module.bsl"
   → Список диагностик: [{ message, range, severity, code }]

2. Для каждой ошибки severity=1 (Error):
   get_range_content (±3 строки вокруг ошибки)
   → Контекст для понимания проблемы

3. code_actions (на позиции ошибки)
   → Варианты автоматического исправления
```

**Типичные severity:**
- 1 = Error (синтаксическая ошибка, блокирует выполнение)
- 2 = Warning (предупреждение, код работает но есть проблема)
- 3 = Information (информационное сообщение)
- 4 = Hint (подсказка по стилю)

#### Обзор нового проекта

```
1. lsp_status → проверить готовность

2. project_analysis(analysis_type="workspace_analysis", query="entire_project")
   → Структура проекта

3. project_analysis(analysis_type="document_symbols", query="путь/к/Модулю.bsl")
   → Список процедур
```

#### Рефакторинг (rename)

```
1. prepare_rename → проверка

2. rename(apply=false) → preview

3. rename(apply=true) → применение (ТОЛЬКО после preview)
```

#### Проверка кода перед коммитом

```
1. Для каждого изменённого .bsl файла:
   document_diagnostics(uri="file:///...")
   
2. Проверить severity=1 (Error):
   - Если есть → код не готов к коммиту
   - Показать ошибки пользователю

3. Опционально проверить severity=2 (Warning):
   - Предупредить о потенциальных проблемах
```

### 2.4 Лимиты

| Параметр | Рекомендация | Максимум |
|----------|--------------|----------|
| `limit` в project_analysis | 5-10 | 100 |
| `limit` в symbol_explore | 3 | — |
| `depth_up/depth_down` в call_graph | 2-3 | 5 |
| `max_nodes` в call_graph | 30-50 | 500 |

### 2.5 После внешних изменений файлов

Если файлы менялись вне LSP (git, генерация, ручные правки):

```
did_change_watched_files
uri="file:///path/to/changed/file.bsl"
change_type=2
```

---

## Часть 3: Стоп-лист (запрещено)

1. **НЕ читай файлы целиком** — используй `get_range_content`
2. **НЕ запускай `call_graph` без лимитов** — всегда `depth_*` и `max_nodes`
3. **НЕ используй `symbol_explore(detail_level="full")` для простого поиска**
4. **НЕ делай `rename(apply=true)` без preview**
5. **НЕ игнорируй `lsp_status`** — проверяй готовность индекса
6. **НЕ запускай тяжёлые операции пока indexing не завершён**

---

## Часть 4: Troubleshooting

### Контейнер не запускается

```bash
docker compose logs -f
```

### Health status unhealthy

BSL LS не смог запуститься. Проверь:
- Достаточно ли памяти (`MCP_LSP_BSL_JAVA_XMX`)
- Правильный ли путь `HOST_PROJECTS_ROOT`

### MCP не подключается

1. Проверь имя контейнера в `mcp.json`
2. Проверь что контейнер запущен: `docker ps`
3. Тест вручную: `docker exec -i <container> mcp-lsp-bridge`

### LSP возвращает пустые результаты

1. Проверь `lsp_status` — возможно индексация не завершена
2. После изменения файлов — вызови `did_change_watched_files`

---

## Справочные материалы

- Архитектура: `docs/codebase-guide.md`
- Конфигурация: `docs/configuration.md`
- Детали tools: `docs/tools/tools-reference.md`
- Tool → LSP mapping: `docs/tools/lsp-methods-map.md`
- Пути Docker ↔ Host: `MCP_PATH_FORMATS_GUIDE.md`
