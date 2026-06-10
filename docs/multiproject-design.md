# Мультипроектный MCP поверх одного BSL LS — дизайн и план

Статус: согласовано, **реализовано и проверено end-to-end** (Phases 0–6 — зелёные).

> Готово в коде (`go build/vet/test ./...` — зелёно) и **проверено в рантайме против настоящего
> BSL LS v1.0.0-rc.1** на платформе 1С 8.3.27.2074 (2026-06-10): демон (`ProjectManager` +
> `multiproject.go`), bridge-роутинг, пер-проектный `lsp_status`, MCP-тулы `project_add/close/list`,
> фильтр `project_root`, Docker (JDK 21 Adoptium, `mcp-state`, `MULTI_PROJECT`, drop `-c`).
>
> **Подтверждено эмпирически:** пустой `initialize` НЕ индексирует родителя (`projects:[]`, idle);
> `project/add` → indexing → **ready**; A остаётся ready, пока греется B (накопительно);
> атрибуция `$/progress` через сериализованный прогрев помечает нужный проект; `documentSymbol`
> роутится в нужный проект; `workspace/symbol` спанит оба тёплых проекта; `project/close` убирает B
> (A остаётся); `state.json` персистится; стадия образа с `temurin-21-jre` собирается
> (`java -version` → 21.0.x). См. Phase 6.

Цель: один контейнер = один процесс BSL LS = N 1С-конфигураций (workspace-папок),
вместо схемы «контейнер на проект». Модель **накопительная**: проекты живут до явного
закрытия или выключения контейнера; на старте греется только последний использованный.

> Требует сервера **BSL LS v1.0+** (мультипроект-механизмы — `ServerContextProvider`,
> per-workspace `@Scope("workspace")` конфиг, `didChangeWorkspaceFolders` — появились в v1.0).

---

## 1. Принципы

- Проекты **накапливаются** и остаются тёплыми (живые сессии, без реиндекса при переключении).
- На **boot** греется только **last-used** (персист в `state.json`); остальное — лениво по первому вызову.
- Маршрутизация запросов — **по URI файла** (делает сам bsl-ls). Параметра проекта на файловых вызовах нет.
- Закрытие проекта — **только** командой `project_close` либо выключением контейнера.
- **Обратная совместимость:** `MULTI_PROJECT=0` (default) → текущее однопроектное поведение.

## 2. На чём держится в BSL LS (ссылки на исходники v1.0.0-rc.1)

| Механизм | Где | Что |
|---|---|---|
| Контекст на проект + роутинг по URI | `context/ServerContextProvider.java:60-62,194-208` | `Map<URI,ServerContext>`; `getServerContext` = O(1) по documentIndex, иначе префикс `documentUri.startsWith(workspaceUri)` |
| Свой `configurationRoot` на проект | `ServerContextProvider.java:108-145` | `getCustomConfigurationRoot(lsc, rootPath)` |
| Add/remove в рантайме | `BSLWorkspaceService.java:127-140` | `didChangeWorkspaceFolders` → `addWorkspace`/`removeWorkspace` (+ per-folder populate) |
| Eager populate всех при старте | `BSLLanguageServer.java:196-204` | на `initialized` идёт `populateContext` по всем контекстам — **этого избегаем** |
| Per-workspace конфиг | `LanguageServerConfiguration.java:82,200-223` | `@Scope("workspace")`; приоритет: `app.configuration.path` (CLI `-c`) → `<root>/.bsl-language-server.json` → `~/.bsl-language-server.json` |

## 3. Архитектура

```
MCP client (agent)
   │ file-tools (URI),  project_add/close/list,  lsp_status[project_root]
   ▼
mcp-lsp-bridge (Go)
   │ ProjectRouter: deriveProjectRoot(uri) → авто-add; per-project readiness gate
   ▼ TCP :9999  (API: textDocument/*, project/add|close|list|status)
lsp-session-manager (Go)
   │ ProjectManager: реестр active + state.json + атрибуция $/progress
   │ → workspace/didChangeWorkspaceFolders
   ▼ stdio
BSL LS v1.0 (один процесс, N ServerContext, роутинг по URI)
```

Новые сущности: **ProjectManager** (демон), **ProjectRouter** (bridge).

## 4. Состояние и персистентность

`state.json` в **смонтированном volume** (переживает `stop/start` и `rm/recreate`):

```json
{ "version": 1, "last_project": "/projects/projA", "active": ["/projects/projA","/projects/projB"] }
```
- `last_project` — что греть на boot; `active` — диагностика/опц. восстановление.
- Пути в container-виде. Запись атомарна (temp+rename), при смене активного и add/close.
- Расположение: отдельный том, напр. `mcp-state:/var/lib/mcp-lsp-bridge/state.json` (НЕ в `/projects`, который ro).

## 5. FSM проекта

`unknown → indexing → ready` (+ `closing → removed`). Управляется ProjectManager.

## 6. Per-project readiness (критично)

`$/progress` от bsl-ls (по коду) не несёт имени workspace. Подход — **атрибуция по очереди add**:
демон инициирует прогревы **сериализованно**; окно индексации между «отправили `add P`» и
«глобальная индексация вернулась в idle» относится к `P` → `P=ready`. Если эмпирически прогресс
окажется различим по проектам — улучшим; стартуем с сериализации.

`CheckReadyOrReturn` становится **пер-проектным**: gate по `deriveProjectRoot(uri)`; на `indexing`
— ответ «retry-after» (механизм уже есть). Готовый A обслуживается, пока индексируется B.

## 7. `lsp_status` — пер-проектный (важно)

Сейчас `lsp_status`/`BuildLSPStatus` (`mcpserver/tools/readiness.go`) — **глобальный**: один
`Indexing.State` и один `Ready` на всех. В мультипроекте это врёт (индексация B гасит готовность A
и лочит вызовы ко всем проектам через общий gate). Меняем:

1. Демон `session/status` → пер-проектно: `{ projects:[{root,state,progress,last_used}], overall:{...} }`.
2. `LSPStatus` → добавить `Projects []ProjectStatus`; `Indexing`/`Ready` оставить как overall (совместимость).
3. Тул `lsp_status` → опц. аргумент `project_root` (задан → один проект; нет → все + last_used).
4. `CheckReadyOrReturn` → gate по проекту из URI.

При `MULTI_PROJECT=0`/одном проекте — поведение как сейчас (`Projects` = один элемент).

## 8. Конфигурация и Docker

- **Убрать CLI `-c`** — иначе перебивает per-project конфиги. Общее (`language: ru`, `v8platform`
  c `binPath` к `.hbk` и `targetVersion`) → глобальный `~/.bsl-language-server.json`; специфичное
  (`configurationRoot`, диагностики) → per-project `<projX>/.bsl-language-server.json`.
- Монтировать **родителя** `/projects` (под ним `projA/`, `projB/`…) — одно path-mapping на всё.
- Volume для `state.json`. Env: `MULTI_PROJECT={0|1}`.

## 9. API (демон ↔ bridge) и MCP-тулы

API демона (`handleAPIRequest`): `project/add {root}`, `project/close {root}`, `project/list`,
`project/status {root}` (под капотом — `workspace/didChangeWorkspaceFolders`).

MCP-тулы: `project_add`, `project_close`, `project_list`; **авто-add** прозрачно из файловых тулов
при неизвестном проекте. Файловые тулы — без изменений сигнатур. `workspace_symbols` — опц.
фильтр `project_root` (иначе спанит все тёплые проекты).

## 10. Инварианты / edge-cases

1. **Непересекающиеся корни** — префикс+`findFirst()` ошибётся на вложенных; запрет при add.
2. **Первый boot без истории** — `initialize` с пустыми folders и null `rootUri` → ничего не
   индексируется. ⚠️ Проверить, что fallback `BSLLanguageServer:162-167` не поднимает весь `/projects`
   как один «root»; иначе — не использовать fallback, добавлять только через `add`.
3. **deriveProjectRoot(uri)** — подъём до маркера (`Configuration.xml`/`src/cf`/`.bsl-language-server.json`/`.git`),
   затем матч против монтирований; результат — container-путь.
4. **Re-add после рестарта демона** — bsl-ls рестартует с демоном; на boot читаем `state.json`,
   греем `last_project` (остальное лениво).
5. **workspace/symbol спанит все тёплые** — задокументировано; фильтр опционально.
6. **Память при close** — подтвердить эмпирически, что `removeWorkspace` освобождает heap
   (changelog: фикс destruction-callbacks `WorkspaceBeanScope`).
7. **CPU при прогреве** — `AnalyzeProjectOnStart` грузит CPU; учесть в `-Xmx`/CPU-лимитах.

## 11. Полный лайфцикл

```
boot → read state.json → init(folders:[], rootUri:null)        # ничего не греем
     → if last_project: didChangeWorkspaceFolders(add) → populate  # один прогрев
call(uri):
     p = deriveProjectRoot(uri)
     if p not active: add p → "indexing, retry"; mark active; write state
     if p indexing:   "indexing, retry"
     else:            forward (роутинг по URI — авто)
project_close(p): remove p → free memory
shutdown: всё гаснет с процессом
```

---

## 12. План работ (фазы)

Легенда: ✅ сделано в коде (собирается, юнит-тесты зелёные) · ⏳ требует рантайм-валидации.

**✅/⏳ Phase 0 — Предусловие: сервер v1.0** *(инфра; ⏳ валидация сборки образа)*
JDK 17→21; pin `BSL_LS_VERSION=1.0.0-rc.1`; `.hbk` + глобальный `~/.bsl-language-server.json`
(`language`,`v8platform`); убрать `-c`. *Verify:* контейнер стартует, hover по платформенному типу.

**✅ Phase 1 — Демон: ядро мультипроекта**
`ProjectManager` (FSM + `state.json` атомарно); `initialize` без folders/rootUri (под `MULTI_PROJECT=1`,
гард от fallback); исходящие `didChangeWorkspaceFolders`; пер-проектная атрибуция `$/progress`;
пер-проектный `session/status`; file-watcher → родитель. *Verify:* юнит ProjectManager; ручной
add двух проектов → оба ready; роутинг hover.

**✅ Phase 2 — Демон: API + boot-warmup**
`project/add|close|list|status`; на boot греть `last_project`. *Verify:* TCP add/close/list; рестарт → греется last.

**✅ Phase 3 — Bridge: роутинг и пер-проектный gate**
`deriveProjectRoot`; авто-add; `CheckReadyOrReturn`+`LSPStatus.Projects` пер-проектно; project-aware
`lsp_status`. *Verify:* первый вызов нового проекта → indexing→результат; B не глушит A.

**✅ Phase 4 — MCP-тулы**
`project_add`/`project_close`/`project_list`; опц. `project_root` в `workspace_symbols`. *Verify:* `go test ./mcpserver/tools/`.

**✅ Phase 5 — Docker/конфиг**
Монтирование родителя; volume `mcp-state`; убрать `-c`; глобальный+per-project конфиги; `MULTI_PROJECT`.
*Verify:* `docker compose up`, два проекта одним контейнером.

**✅ Phase 6 — Эмпирическая валидация** (проверено e2e против BSL LS v1.0.0-rc.1, 2026-06-10)
Память до/после close; вложенные корни; нагрузка (прогрев A при живом B); `go build/vet/test ./...`.

Зависимости: 0→1→2→3→4; 5 параллельно; 6 в конце. Код Phase 1–4 не зависит от Phase 0 (стандартный LSP).

## 13. Риски / откат

- `MULTI_PROJECT=0` (default) — поведение и API текущие; новый код неактивен.
- Неизвестные (валидировать в 1/6): реальное освобождение памяти на `removeWorkspace`; пустой
  `initialize` не индексирует родитель; точность атрибуции `$/progress` (иначе строгая сериализация).
- Откат: фича за флагом; `MULTI_PROJECT=0` возвращает однопроектную схему без отката кода.
