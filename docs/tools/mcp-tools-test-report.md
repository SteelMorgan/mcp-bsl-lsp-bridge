# Отчёт по тестированию MCP tools (mcp-lsp-bridge)

✅ Skill used: `mcp-testing`

Контекст:
- MCP server: `project-0-mcp-lsp-bridge-lsp-bsl-bridge`
- Workspace (container): `/projects/test-workspace`
- Tool list (ожидаемая): `call_hierarchy`, `code_actions`, `document_diagnostics`, `format_document`, `get_range_content`, `hover`, `implementation`, `prepare_rename`, `project_analysis`, `rename`, `signature_help`, `symbol_explore`, `workspace_diagnostics`

Файлы-фикстуры:
- F1: `file:///projects/test-workspace/Ext/SessionModule.bsl`
- F2: `file:///projects/test-workspace/AccumulationRegisters/_ДемоОстаткиТоваровВМестахХранения/Ext/ManagerModule.bsl`
- F3: `file:///projects/test-workspace/AccountingRegisters/_ДемоЖурналПроводокБухгалтерскогоУчета/Ext/RecordSetModule.bsl`

---

## 1) get_range_content

### Case 1.1: happy (первые строки F1)
- **Вход**: start_line=0 start_character=0 end_line=22 end_character=400 strict=false
- **Ожидание**: вернёт текст первых строк модуля, без ошибки.
- **Факт**: PASS
- **Заметки**: end_line должен быть в пределах файла; clamp по end_character работает, но end_line за границей даёт ошибку.

### Case 1.2: negative (strict=true, выход за границы по character)
- **Вход**: start_line=0 start_character=0 end_line=0 end_character=9999 strict=true
- **Ожидание**: ошибка из-за строгой проверки границ.
- **Факт**: PASS (ошибка: `invalid end character on line 0: 9999 (line length: 107)`)

---

## 2) project_analysis

### Case 2.1: workspace_analysis (workspace_uri с пробелами)
- **Вход**: analysis_type=`workspace_analysis`, query=`entire_project`, workspace_uri=`file:///D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace`
- **Ожидание**: ненулевое число файлов/символов (в workspace много `.bsl`).
- **Факт**: FAIL (вернул `Total files: 0`, `bsl: 0 files (NaN%)`)
- **Заметки**: `workspace_uri` должен быть **с пробелами**, без `%20`, иначе валидатор пути падает.

### Case 2.2: document_symbols (F1)
- **Вход**: analysis_type=`document_symbols`, query=`file:///projects/test-workspace/Ext/SessionModule.bsl`, workspace_uri=`file:///D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace`
- **Ожидание**: увидит область и процедуру.
- **Факт**: PASS (нашёл `ОбработчикиСобытий` и `УстановкаПараметровСеанса`)

### Case 2.3: text_search (заявлен в описании)
- **Вход**: analysis_type=`text_search`, query=`СтандартныеПодсистемыСервер.УстановкаПараметровСеанса`, workspace_uri=`file:///D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace`
- **Ожидание**: поиск текста по проекту (либо error “не поддерживается”).
- **Факт**: FAIL (ошибка: `Unknown analysis type: text_search`)
- **Заметки**: несоответствие описания tool и реальной реализации (в switch нет кейса `text_search`).

### Case 2.4: workspace_symbols (по имени процедуры)
- **Вход**: analysis_type=`workspace_symbols`, query=`УстановкаПараметровСеанса`, workspace_uri=`file:///D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace`, limit=20
- **Ожидание**: вернёт список совпадений и координаты.
- **Факт**: PASS (18 результатов, включая F1; есть “Recommended hover coordinate”)

---

## 3) symbol_explore

### Case 3.1: basic поиск по имени символа
- **Вход**: query=`УстановкаПараметровСеанса`, detail_level=`basic`
- **Ожидание**: вернёт список совпадений.
- **Факт**: PASS (18 matches + TOC)

### Case 3.2: file_context фильтрация
- **Вход**: query=`УстановкаПараметровСеанса`, file_context=`SessionModule`
- **Ожидание**: сузит результаты до файла F1.
- **Факт**: FAIL (ошибка: `File context error: File 'SessionModule' not found`)
- **Заметки**: похоже, `file_context` ищет по локальному FS пути без учёта container path mapping.

---

## 4) document_diagnostics

### Case 4.1: baseline (F1)
- **Вход**: uri=`file:///projects/test-workspace/Ext/SessionModule.bsl`
- **Ожидание**: либо пусто, либо список предупреждений/ошибок.
- **Факт**: FAIL (таймаут: `context deadline exceeded`)
- **Заметки**: допустимо на старте, пока LS “читает проект”, но важно чтобы это не переводило клиента в перманентный `state=error`.

### Case 4.2: после рестарта MCP (проверка “не отравляет readiness”)
- **Вход**: те же параметры
- **Ожидание**: даже если таймаут, последующие вызовы других tools не должны падать в `state="error"` навсегда.
- **Факт**: PARTIAL (таймаут воспроизводится; readiness продолжает возвращать `starting` — требуется наблюдение дольше, чтобы поймать переход в ready и проверить hover/signature_help)

---

## Блокер, найденный в процессе тестов

При таймауте LSP-запроса (`context deadline exceeded`) LSP-клиент уходит в состояние `error`, и последующие инструменты начинают возвращать readiness-пейлоад со `state="error"`, то есть **вся работа MCP фактически блокируется до рестарта процесса**.

Исправление внесено в код (автовосстановление клиента при `!connected`/`error`), но чтобы оно подхватилось, нужно **перезапустить MCP-сервер в Cursor** (чтобы поднялся новый процесс `mcp-lsp-bridge` в контейнере).

