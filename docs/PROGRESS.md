# PROGRESS — журнал выполнения

Короткие записи по завершении каждой фазы: что сделано, что проверено. Ведёт Claude Code.

> **СОСТОЯНИЕ (для возобновления после сжатия контекста / в новой сессии):**
> Готовы **Фазы 0–6**. **Следующая — Фаза 7** (локальная сборка целиком: фронт на реальном API,
> Caddy, Swagger, README, smoke). Чтобы продолжить: прочитать этот файл + `git log --oneline`,
> затем идти по `docs/ROADMAP.md`. Инварианты и правила — в `CLAUDE.md`. Правило версий —
> `frontend/src/version.ts` (бампать всегда; сейчас 0.7.0.0, backend `main.go` зеркалит).
> Docker поднят через **Colima** (`colima start` после ребута). Локальный `.env` уже есть (gitignore).
> Коммиты — только локальные, **push не делаем до Фазы 9**.
> **Долг:** интеграционный тест sync против нод (`go test -tags=integration -run Integration
> ./internal/sync/ -v`) написан, но ещё не прогонялся вживую — нужен поднятый Colima.

---

## Фаза 0 — Контракт и каркас фронта ✅

**Сделано:**
- Структура репозитория из `SPEC.md §11` (backend/, frontend/, deploy/, test/, docs/).
  Канонические доки перенесены: `CLAUDE.md` (корень), `docs/SPEC.md`, `docs/ROADMAP.md`,
  `.claude/settings.json`. Git инициализирован на ветке `build/mvp`. `.gitignore`, `.env.example`.
- Доки Фазы 0: `docs/ARCHITECTURE.md` (ключевые решения — SSH-конфиг-механизм, мультидвижковый
  sync, реестр протоколов, модель доступа, шифрование, выбор sing-box для Hysteria2),
  `docs/ERD.md` (mermaid из SPEC §4), `docs/QUESTIONS.md` (Q1 движок Hysteria2=sing-box дефолт,
  Q2 TLS-стратегия, Q3 предустановка движка, Q4 Docker не установлен в среде, Q5/Q6 git/деплой),
  пустой `docs/PROGRESS.md`.
- **`docs/openapi.yaml`** — полный контракт всех эндпоинтов из SPEC §10.
- Frontend scaffold: Vite + React + TS + Tailwind + shadcn/ui (компоненты вручную), React Router,
  TanStack Query. Типы сгенерированы из openapi (`openapi-typescript` → `src/api/schema.ts`),
  типизированный API-клиент (`src/api/client.ts`, `hooks.ts`). Мок-слой MSW (`src/mocks/`) —
  handlers покрывают **весь** контракт, реалистичные мок-данные (4 сервера, 9 inbound-ов всех
  5 протоколов включая 2×Hysteria2, 5 клиентов, аудит), генерация connection-URI и подписки.

**Проверено:**
- `npx @redocly/cli lint openapi.yaml` — валиден (0 ошибок).
- `npm run build` на скелете — проходит.
- Мок-слой отвечает по контракту (проверено на живом приложении в Фазе 1).

## Фаза 1 — Мок-фронт (полный UI на моках) ✅ — точка ревью человеком

**Сделано:**
- Все страницы на мок-данных: **Login** (+ шаг TOTP), **Dashboard** (карточки серверов, метрики
  CPU/RAM/диск, счётчики, время синхронизации), **Servers** + **ServerDetail** (inbound-ы всех
  протоколов включая Hysteria2, формы, «проверить»/«рестарт движка»), **Clients** + **ClientDetail**
  (создание одной кнопкой, выдача доступа к inbound-ам чекбоксами, подписка, QR, скачать, ротация
  токена, per-inbound ссылки), **Settings** (домен/базовый URL, тема, 2FA-энролл с QR), **Logs**
  (аудит с пагинацией).
- Тёмная/светлая тема через CSS-переменные, переключатель сохраняется в localStorage.
  Современный минимализм, адаптивный layout с сайдбаром.
- Реалистичные мок-данные для всех сценариев (online/error-серверы, пустые состояния, disabled-клиент).

**Проверено:**
- `npm run build` — зелёный. `npm run test` — 7/7 зелёные (buildLink по всем протоколам, Login-флоу).
- `npx tsc -b` — 0 ошибок. `npm run lint` — 0 ошибок.
- `npm run dev` поднимает приложение **без backend** (MSW). Вживую пройден флоу:
  Login → 2FA (123456) → Dashboard → Servers → ClientDetail (подписка + `vless://`/`hysteria2://`/`ss://`
  URI + QR + credentials); переключение тёмная/светлая тема работает.

> **СТОП.** Ожидается ревью человеком: посмотреть фронт, поправить UI/UX. Дальше (Фаза 2, backend) —
> только после одобрения.

**Правки по итогам ревью (Фаза 1):** одностраничный UI серверов (карточки раскрываются на месте,
без сайдбара и дашборда; Клиенты/Логи/Настройки — иконки внизу слева; тема/выход — иконки вверху
справа); темы серо-оранжевая + папирусная; контраст статус-индикаторов; автоустановка движков при
добавлении сервера (провижининг, +контракт `/install` и `provision_status`); структурированная форма
inbound с видимыми дефолтами и live-превью (в духе 3x-ui) + поля Hysteria2/sing-box для сервера;
рандомный SNI из пула 10 доменов; лого = мем «ЧЕРЕМША»; версия приложения под заголовком
(`frontend/src/version.ts`, правило автобампа — CLAUDE.md §11).

## Фаза 2 — Фундамент backend ✅

**Сделано:**
- `config` — чтение/валидация env; обязательные `PANEL_ENCRYPTION_KEY` (32 байта base64),
  `PANEL_JWT_SECRET`, `PANEL_ADMIN_PASSWORD` — падение с понятной ошибкой при отсутствии;
  остальное с дефолтами; вывод `SubBaseURL` из домена.
- `crypto` — AES-256-GCM (`Encrypt`/`Decrypt`, случайный nonce, аутентификация) и bcrypt (cost 12).
- `db` + `migrations` — SQLite через **modernc.org/sqlite** (pure-Go, без cgo); встроенный раннер
  миграций (embed FS, таблица `schema_migrations`, транзакция на миграцию, идемпотентно); схема
  `0001_init` из SPEC §4 (все таблицы + provision/hysteria-поля); FK включены (pragma).
- `logging` — slog JSON. `httpapi` — chi-роутер, middleware, `/healthz`, `/readyz` (проверка БД).
- `cmd/panel` — старт с graceful shutdown. `backend/Dockerfile` (multi-stage, CGO_ENABLED=0,
  alpine, non-root). `docker-compose.yml` (backend + volume, healthcheck).

**Проверено:**
- `go vet ./...` ✓, `gofmt -l .` пусто ✓, `go test ./...` ✓ (config: valid/missing/bad-key;
  crypto: round-trip/wrong-key/tamper/bcrypt; db: миграции/идемпотентность/FK; httpapi: health).
- Локальный запуск бинарника: `/healthz`,`/readyz` → 200, БД мигрирует.
- **`docker compose up --build`** → контейнер **healthy**, миграции применены, `curl /healthz` → 200.

**Отклонения от SPEC §3 (обоснование):** вместо `golang-migrate` — минимальный встроенный раннер
(проще, без внешней зависимости и cgo, полностью покрывает «аддитивные миграции при старте»);
драйвер `modernc.org/sqlite` вместо cgo-`mattn` (простые сборки/образы). Docker — через Colima (Q4).

## Фаза 3 — Аутентификация и аудит ✅

**Сделано:**
- `store` — **рукописный типизированный слой доступа** (вместо `sqlc`, см. ниже): модели/методы
  для `admins` (get/create/count/set-TOTP) и `audit_logs` (insert/list/count); таймстампы —
  RFC3339-строки, управляются в Go.
- `auth` — логин по bcrypt; **JWT** (HS256, `golang-jwt/v5`) access-токен + короткоживущий
  2FA-challenge-токен; **TOTP** (`pquerna/otp`): setup (otpauth-URL + QR через `skip2/go-qrcode`),
  enable, verify — секрет хранится **зашифрованным** (AES-GCM); middleware `RequireAuth`
  (bearer → admin в контексте). Первичный админ сидится из env при первом старте.
- `httpapi` — эндпоинты контракта: `POST /api/auth/login` (→ `need_2fa`/`tokens`/`challenge_id`),
  `/2fa/verify`, `/2fa/setup`, `/2fa/enable`, `/logout`, `GET /api/auth/me`; `GET /api/logs`
  (пагинация). **Rate-limit** на login/verify (`go-chi/httprate`, 15/мин/IP). Аудит пишется на
  login/2fa.enable/logout.

**Проверено:**
- `go vet` ✓, `gofmt` ✓, `go test ./...` ✓ (login успех/провал, `/me` с токеном/без, **полный цикл
  TOTP** setup→enable→login-с-2FA→verify, bad-code, аудит-запись, `/api/logs` + требование авторизации).
- Живой end-to-end (curl): сид админа из env, login→JWT, wrong→401, `/me` 200, `2fa/setup`→otpauth+секрет,
  `/api/logs`→запись login (detail-объект, IP, UA, RFC3339). Swagger UI появится в Фазе 7.

**Решение по слою данных:** взят рукописный `store` вместо `sqlc` — трение `sqlc`+`modernc`+SQLite на
`DATETIME` (scan в `time.Time`) при малом объёме запросов; рукописный слой даёт контроль над
сканированием и единый RFC3339-формат. SPEC §3 допускает альтернативу с обоснованием.

## Фаза 4 — Управление серверами + провижининг ✅

**Сделано:**
- Миграция `0002` — аддитивная колонка `servers.engines_json` (кэш статуса движков, обновляется
  при check/provision). CRUD `servers` в `store` (+ `ApplyCheck`, `SetProvision`); SSH-секреты
  шифруются при сохранении, в DTO не отдаются.
- `ssh` — `Runner`/`Dialer` за интерфейсом: реальная реализация на `golang.org/x/crypto/ssh`
  (ключ/пароль, exec, stdin), мок в `ssh/sshtest` для юнит-тестов.
- `provision` — автоустановка движков (SPEC §5.1): определение ОС (**только Debian/Ubuntu**,
  иначе понятная ошибка без частичной установки), установка xray-core и sing-box, генерация
  **self-signed** серта (`/etc/sing-box/self.crt`), идемпотентность (пропуск при наличии).
- `servers.Service` — Create (шифрование + авто-провижининг в фоне), Get/List/Update/Delete,
  Provision, Check (доступность + IP + гео [`ip-api`, за интерфейсом] + статус движков),
  Stats (CPU/RAM/диск через `/proc` + `free`/`df`, парсер), RestartEngine.
- `httpapi` — эндпоинты контракта: `GET/POST /api/servers`, `GET/PUT/DELETE /api/servers/{id}`,
  `POST /check|/install|/restart-xray`, `GET /stats`; аудит на create/update/delete/check/install/restart.

**Проверено:**
- `go vet` ✓, `gofmt` ✓, `go test ./...` ✓ (провижининг: свежая установка/OS-gate/идемпотентность
  на мок-раннере; сервис: шифрование секрета, dial-ошибка→status error, парсер метрик; HTTP:
  CRUD, check→online+движки+IP, stats, provision→installed, guard 401).
- Живой end-to-end (docker compose, v0.5.0.0): login→create (`installing`, секреты не в ответе)→
  авто-провижининг к недоступной ноде корректно переходит в `failed` с текстом SSH-ошибки; list;
  guard 401. Реальная установка движков будет проверена в Фазах 5–6 на dockerized-Debian-ноде.

## Фаза 5 — Inbound-ы и реестр протоколов ✅

**Сделано:**
- `protocols` — реестр + интерфейс `Protocol` (`Name`, `Engine`, `BuildInbound`, `BuildLink`).
  Адаптеры: **VLESS** (Reality/TLS/транспорты), **VMess**, **Trojan**, **Shadowsocks** (движок
  xray) и **Hysteria2** (движок sing-box). Каждый регистрируется в своём `init()`. Генерация
  **Reality-ключей** (X25519) и short-id на бэкенде (`reality.go`). Общие хелперы xray
  (`buildXrayStream`, `linkQuery`, `xrayInboundFragment`).
- `store` — CRUD inbound-ов (JSON-поля `settings/stream_settings/sniffing`, `client_count`).
  `inbounds.Service` — валидация протокола через реестр, автогенерация Reality-ключей при создании
  (заглушки `<generated>` заменяются реальными). `httpapi` — эндпоинты контракта:
  `GET/POST /api/servers/{id}/inbounds`, `GET/PUT/DELETE /api/inbounds/{id}` (+ `engine` в DTO), аудит.
- `test/xray-node`, `test/hysteria-node` — dockerized-ноды + README.

**Проверено:**
- `go vet` ✓, `gofmt` ✓, `go test ./...` ✓ (все 5 адаптеров: engine/BuildInbound-JSON/BuildLink-URI;
  inbound CRUD + генерация Reality-ключей на бэкенде + отказ на неизвестном протоколе + движок hysteria).
- **Интеграция (Docker, build-тег `integration`):** конфиг **VLESS Reality + VMess + Trojan**,
  собранный реестром, принят реальным **`xray -test`** (Xray 26.3.27 → «Configuration OK»);
  конфиг **Hysteria2** принят **`sing-box check`** (с реальным self-signed сертом).
  Запуск: `go test -tags=integration -run Integration ./internal/protocols/`.

## Фаза 6 — Клиенты, доступы, подписка и мультидвижковый sync ✅

**Сделано:**
- `store` — `clients.go` (CRUD клиентов, enable/disable, rotate-token, гранты через
  `client_inbounds`: `SetClientInbounds`/`GrantedInboundIDs`/`ListClientGrants`;
  `ListActiveClientInbounds` — только `grant.enabled AND inbound.enabled` для подписки;
  `ListInboundGrantedClients` — включённые клиенты для sync). `settings.go`
  (`GetSetting`/`SetSetting` для хэшей sync). `servers.SetSync` (штамп `last_sync_*`).
  Схема **не менялась** — таблицы `clients`/`client_inbounds` заложены в `0001_init`.
- `clients.Service` — генерация UUID (v4), пароля (hex-16) и токена (base64url-24) на бэкенде;
  CRUD, гранты, rotate; `Links` — сборка URI по активным inbound-ам через реестр протоколов.
- `subscription.Service` — `Build(token)` → base64 из `\n`-склеенных URI (xray + `hysteria2://`);
  выключенный клиент → пусто; неизвестный токен → 404. Единственный публичный эндпоинт.
- `sync.Service` — **мультидвижковый**: собирает enabled-inbound-ы ноды, группирует по движкам,
  строит `config.json` Xray и sing-box-конфиг из inbound-ов + выданных клиентов; по SSH:
  `mkdir -p` → бэкап → запись (`cat >`) → валидация (`xray -test` / `sing-box check`) →
  при ошибке **восстановление бэкапа** без рестарта → `systemctl restart`. **Идемпотентность**
  по sha256-хэшу (в `settings`). Провижининг-гейт: пока не `installed` — не пушит
  (`ErrNotProvisioned`). Реализует `Connector` через новый `servers.Connect`.
- `httpapi` — `/api/clients` (CRUD, enable/disable, `PUT /inbounds`, `rotate-token`, `links`,
  `qrcode` PNG, `config` download), публичный `GET /sub/{token}` (rate-limit 60/мин),
  `POST /api/servers/{id}/sync`. `subscription_url` в DTO из `SubBaseURL`.
- `docs/openapi.yaml` — добавлен `POST /api/servers/{id}/sync` + схема `SyncResult`; типы фронта
  перегенерированы (`npm run gen:api`).

**Проверено:**
- `go vet` ✓, `gofmt` ✓, `go test ./...` ✓. Новые тесты: store (клиенты/гранты/фильтрация
  выключенных), `clients` (уникальность креденшелов, Links, rotate), `subscription`
  (base64/пусто/404), `sync` (пуш обоих движков + команды write/validate/restart; идемпотентный
  skip; восстановление бэкапа при отказе валидации; `ErrNotProvisioned`; форма конфигов с
  креденшелами клиента), **httpapi e2e**: клиент → гранты на оба движка → `/sub/{token}` = 2 URI
  (`vless://` + `hysteria2://`); disable → пусто; rotate → старый токен 404; `/sync` → рестарт xray.
- Frontend: `npm run build` ✓, `npm run lint` ✓ (0 ошибок), `npm run test` ✓.
- **Интеграция sync (Docker, build-тег `integration`, ждёт живого прогона на Colima):**
  `internal/sync/validate_integration_test.go` берёт **байты из `plan()`** (ровно то, что пушит
  sync) и валидирует их реальными `xray -test` / `sing-box check`.
  Запуск: `go test -tags=integration -run Integration ./internal/sync/ -v`.
