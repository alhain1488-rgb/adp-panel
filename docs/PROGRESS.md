# PROGRESS — журнал выполнения

Короткие записи по завершении каждой фазы: что сделано, что проверено. Ведёт Claude Code.

> **СОСТОЯНИЕ:** **Все фазы 0–9 завершены + пост-релизные фичи.** Панель развёрнута на VPS человека
> (Ubuntu 22.04), работает по HTTPS с доверенным сертификатом Let's Encrypt (через sslip.io-хостнейм,
> домена нет); движки провижинятся, клиенты/подписки/VPN проверены рабочим трафиком; UI отревьюен
> человеком. Проект запушен в **публичный** GitHub-репозиторий `alhain1488-rgb/adp-panel`, ветка
> `build/mvp`; установка одной командой (`install.sh`). Последние фичи — **бэкапы** (ручной
> Export/Import + авто-отправка в Telegram) и **очистка ноды перед установкой** (опция при добавлении
> сервера), см. записи «Пост-9» ниже. **НА VPS развёрнута v0.8.4.0**; локально готовы, но ещё НЕ
> запушены/не задеплоены: **локализация RU/EN + AmneziaWG 2.0** (v0.9.1.0) — верифицированы, ждут
> `git push` + редеплой. Версия — `frontend/src/version.ts` (сейчас 0.9.1.0; бампать при
> правках). Docker — через **Colima** локально;
> на VPS — `docker compose up` (backend + Caddy). Данные деплоя (IP/SSH) — только в сессии, в
> репозиторий не попали (проверено сканом всей истории). Дальше — по запросам человека.

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
- **Интеграция sync (Docker, build-тег `integration`) — прогнано вживую ✓:**
  `internal/sync/validate_integration_test.go` берёт **байты из `plan()`** (ровно то, что пушит
  sync) и валидирует их реальными движками: **`xray -test`** → «Configuration OK» (Xray 26.3.27),
  **`sing-box check`** принял Hysteria2-конфиг (с self-signed сертом). Оба теста PASS.
  Запуск: `go test -tags=integration -run Integration ./internal/sync/ -v`.

## Фаза 7 — Локальная сборка целиком (фронт на реальном API) ✅

**Сделано:**
- **Фронт → реальный API.** Клиент (`src/api/client.ts`) ходит по относительным `/api|/sub` с
  Bearer-токеном; моки (MSW) управляются `VITE_USE_MOCKS` (dev — по умолчанию вкл; продакшн-образ
  собирается с `VITE_USE_MOCKS=false`). Сверил карту вызовов фронта с backend — единственная дыра
  (`/api/settings`) закрыта; `useDashboard` не вызывается (Dashboard убран), 2FA-эндпоинты на месте.
- **Backend: `/api/settings`** (GET/PUT) — поверх KV-таблицы `settings` + дефолты из конфига
  (domain, subscription_base_url, sync_interval_seconds, theme; `hysteria_engine` read-only).
- **Backend: Swagger** — `/swagger` (Swagger UI из CDN) + `/openapi.yaml` (встроенная копия
  канона через `go:embed`; синхронизация `make sync-openapi`).
- **Caddy + web-образ** (`deploy/Caddyfile`, `deploy/web.Dockerfile`): собирает фронт без моков,
  отдаёт статику, reverse-proxy `@backend` (`/api|/sub|/swagger|/openapi.yaml|/healthz|/readyz`)
  через `handle`-блоки (до SPA-fallback), TLS: локально internal-CA, на VPS — авто Let's Encrypt.
  `docker-compose.yml` расширен сервисом `web` (80/443) + volume-ы Caddy. `.dockerignore`.
- **README.md** (назначение, требования, локальный старт, env, добавление протокола, движок
  Hysteria2, troubleshooting), **`scripts/smoke.sh`** (SPEC §12), **Makefile**.

**Проверено (вживую на поднятом стеке):**
- `go test ./...` ✓ (+ тесты `/api/settings` и Swagger), `go vet` ✓, `gofmt` ✓;
  фронт `build`/`lint`/`test` ✓.
- `docker compose up --build` поднял **backend (healthy) + web (Caddy)**. По HTTPS через Caddy:
  `/healthz` → `{"status":"ok","version":"0.8.0.0"}`; `/swagger` → Swagger UI; `/openapi.yaml` →
  спецификация; `/` и `/clients` → SPA (реальный API, не моки); `/sub/{token}` → base64-подписка.
- **`./scripts/smoke.sh` — PASS:** health → логин → сервер → inbound-ы (VLESS + Hysteria2) →
  клиент → гранты → `/sub/{token}` вернул 2 URI (`vless://` + `hysteria2://`).
- **Визуальная проверка** (Vite-превью против реального API): страница входа и вкладка
  дображены под бренд — лого **ЧЕРЕМША** + «Absolutely Disgusting Panel» (было «Xray Panel» +
  щит), убрана демо-подсказка; `index.html` title/favicon обновлены. Вход реальным админом →
  страница серверов с живыми данными из БД (сервер smoke-node с 2 inbound-ами). Консоль чистая
  (Login-тест зелёный). Версия панели → **0.8.0.1** (backend `main.go` зеркалит фронт).

## Фаза 8 — Деплой на VPS ✅ — точка ревью человеком

**Сделано (данные VPS — только в сессии, в репозиторий не коммитились):**
- Развёрнуто по SSH на **Ubuntu 22.04** VPS человека: добавлен swap (сборка на 1 CPU/1 ГБ RAM),
  установлен Docker, перенесён проект (`git archive HEAD` — без `.env`), сгенерированы **свежие
  секреты в `.env` на сервере**, поднят `docker compose up` (backend + Caddy).
- **Домена нет** → HTTPS через `sslip.io`-хостнейм (`<ip>.sslip.io`): Caddy выпустил **доверенный
  сертификат Let's Encrypt** (TLS-ALPN-01). Панель доступна снаружи по HTTPS, вход работает.
- `scripts/smoke.sh` против прод-бэкенда — PASS.

**Фиксы, найденные на живом деплое (провижининг + VPN):**
- `provision`: движки падали на `tput` (нет `$TERM` в PTY-less SSH) → задан `TERM=xterm`; затем
  `unzip` не ставился без `apt-get update` на свежей ноде → добавлен apt-prep (ожидание
  dpkg-lock + `apt update` + предустановка `unzip/curl/ca-certificates/openssl`); в ошибки
  провижининга добавлен stdout. Провижининг реальной ноды → `installed` (xray + sing-box running).
- **Sync не запускался** (не было триггера) → добавлен **авто-sync** при изменении инбаундов/клиентов.
  Причина «ничего не работает» была именно в этом: конфиги не пушились на ноду.
- **hysteria2-URI**: убран `/`-путь и IP-`sni`, добавлены `alpn=h3` и `security=tls` — заработало
  в реальном клиенте. VLESS Reality и Hysteria2 проверены **рабочим VPN-трафиком** (sing-box-клиент
  через ноду → exit-IP = IP ноды).
- **UI-ревью человеком** на живой панели → адаптивная карточка сервера (десктоп — модалка без
  reflow сетки, мобилка — inline), одинаковые тайлы/боксы, список inbounds без горизонтального
  скролла, крупный логотип, новые иконки (EthernetPort / check-engine). Версия → 0.8.1.3.

**Проверено:** человек подтвердил, что панель работает на его VPS по HTTPS и VPN поднимается.

## Фаза 9 — Git (push) ✅

**Сделано:**
- Просканирована **вся история коммитов** на секреты/данные деплоя (IP VPS/ноды, пароли, ключи) —
  чисто; `.env` никогда не коммитился, gitignore на месте.
- Создан **приватный** GitHub-репозиторий, ветка `build/mvp` запушена (`gh repo create --private`).

**Definition of Done проекта достигнут:** пройдены три ревью (мок-фронт → локальная работа →
панель на VPS человека); проект выгружен в git; тесты/линтеры зелёные; README полный.

---

## Пост-9 — Бэкапы (Export/Import) ✅ (v0.8.2.0)

Мотивация: человек случайно снёс панель на VPS (переустановил ОС) и потерял данные. Нужен
переезд «одной командой»: `install.sh` → импорт бэкапа → все серверы/инбаунды/клиенты на месте,
подписки можно раздать заново.

**Сделано (backend `internal/backup`):**
- **Export** — консистентный снимок SQLite (`VACUUM INTO`) → tar.gz(`meta.json`+`panel.db`) →
  шифрование парольной фразой: argon2id → AES-256-GCM. Формат-файл `.adpbak`
  (`magic|salt|nonce|ciphertext`, AAD=magic; неверная фраза → провал GCM-тега, не частичная выдача).
- **Умный Import** — расшифровка → **пере-шифрование секретных колонок** (`admins.totp_secret_enc`,
  `servers.ssh_secret_enc`, `servers.ssh_passphrase_enc`) со *старого* мастер-ключа (лежит в архиве)
  на ключ *нового* сервера → инвариант «ключ только из `PANEL_ENCRYPTION_KEY`» сохранён. Восстановление
  стейджится как `<db>.incoming`; на старте `db.Open` атомарно подменяет БД (удаляет старую + wal/shm,
  rename). **Живая БД не повреждается, если импорт падает на любом шаге.** Перезапуск — через
  SIGTERM self (graceful shutdown → Docker `restart: unless-stopped` поднимает с восстановленной БД).
- Эндпоинты (защищённые JWT): `POST /api/backup/export` (скачивание), `POST /api/backup/import`
  (multipart). Аудит-лог на обе операции.

**Сделано (frontend):** вкладка **Backup** в Settings — Export (фраза → скачать файл) и Restore
(файл + фраза → диалог подтверждения → импорт → «Restore applied» с числом серверов/инбаундов/клиентов
и инструкцией перезайти с креды из бэкапа). MSW-стабы для мок-режима.

**Проверено:**
- `go test ./...`, `go vet`, `gofmt` — зелёные. Юнит-тест доказывает ключевую гарантию:
  export на ключе A → import на ключе B → секреты читаются под B и **не** под A; +неверная фраза,
  +битый архив, +пустые колонки пропускаются, +staged-restore подменяется на старте.
- `npm run build/lint/test` — зелёные. UI проверен в браузере end-to-end (export качает blob;
  restore → confirm → 200 → success-диалог).
- Adversarial-ревью диффа (3 измерения × verify, 10 агентов) — **0 подтверждённых findings**.
- OpenAPI/Swagger обновлены, встроенная копия синхронизирована.

## Пост-9 — Авто-бэкап в Telegram (Stage 2) ✅ (v0.8.3.0)

**Сделано (backend `internal/backup/telegram.go`):**
- Отправка зашифрованного `.adpbak` в Telegram-чат через Bot API (`sendDocument`, multipart).
- Конфиг в KV-`settings`: bot token и парольная фраза **шифруются at-rest** мастер-ключом (как все
  секреты); chat id, enabled, interval_hours, last_at/last_error/last_ok — статус последнего прогона.
- Фоновый планировщик (тикер 10 мин, гейт по `enabled` + интервалу от `last_at`) в `main.go`,
  живёт до shutdown. Метод `RunNow` — для кнопки «Backup now» и планировщика.
- Эндпоинты (JWT): `GET/PUT /api/backup/telegram` (статус/конфиг, **секреты наружу не отдаются**;
  пустой token/passphrase в PUT = «оставить прежний»), `POST /api/backup/telegram/run`. Аудит.

**Сделано (frontend):** карточка «Automatic backup to Telegram» на вкладке Backup — enable-тоггл,
bot token, chat id, интервал, парольная фраза (плейсхолдеры «stored — leave blank to keep»),
«Send test now», строка статуса последнего прогона с бейджем sent/failed. MSW-стабы.

**Проверено:** юнит-тесты (config round-trip с шифрованием секретов, `RunNow` против mock Bot API
через httptest — успех и ошибка записываются в статус, отсутствие конфига → ошибка) — зелёные;
`go test/vet/gofmt` и `npm build/lint/test` — зелёные; UI проверен в браузере end-to-end (сохранение
конфига → «Send test now» → «Last run: … sent»), чистая консоль на свежей загрузке.

---

## Пост-9 — Очистка ноды перед установкой (opt-in wipe) ✅ (v0.8.4.0)

Мотивация: на новой ноде может стоять чужой прокси/панель, занимающий нужные порты или память.

**Сделано (backend):**
- `provision.CmdCleanNode` — **прицельный** best-effort скрипт (всегда `exit 0`, не роняет установку):
  останавливает/отключает известные прокси/VPN/панели (xray, v2ray, sing-box, hysteria, trojan,
  shadowsocks, tuic, wireguard, openvpn, 3x-ui/x-ui/s-ui, marzban…), убивает их процессы (`pkill -x`),
  удаляет **все Docker-контейнеры**, сносит их бинарники/конфиги/unit-файлы (только конкретные пути,
  без переменных → не может раскрыться в `/`). **ОС, SSH, сеть, nginx/apache не трогает.**
- Запускается **только по флагу** и **только на поддерживаемой ОС** (после OS-check, перед apt-prep):
  `Provision(ctx, r, host, hy, wipe)`. Флаг проброшен `create → StartProvision → Provision`. Не
  персистится (одноразовое действие); в аудит пишется `{"wipe":true|false}`.

**Сделано (frontend):** на форме Add Server (только при создании) чекбокс «Wipe existing proxy setup
before installing» с предупреждением о необратимости; шлёт `wipe_existing`. По умолчанию выключен.

**Проверено:** новые юнит-тесты провижина (wipe=true → чистка выполняется перед install; wipe=false и
unsupported OS → чистка НЕ выполняется); `go test/vet/gofmt` и `npm build/lint/test` — зелёные; UI —
чекбокс рендерится/переключается на Add, отсутствует на Edit, чистая консоль; **adversarial-ревью
безопасности деструктивного скрипта — 0 findings**. OpenAPI обновлён.

## Пост-9 — AmneziaWG 2.0 как третий движок + локализация ✅ (v0.9.1.0)

Два локальных коммита поверх origin: `afacd58` (локализация RU/EN + селектор форматов direct-domains)
и `67caf16` (AmneziaWG 2.0, v0.9.0.0). Затем **верификация и корректирующие правки** (v0.9.1.0, эта запись).

**AmneziaWG — что верифицировано и исправлено:**
- **Формат `vpn://` сверен с исходником AmneziaVPN** (`amnezia-client` tag `4.8.19.0`, а не master —
  на master ещё нет 2.0-параметров). Исправлено в `internal/amneziawg/vpnlink.go`: контейнер
  `amnezia-awg2` (не `amnezia-awg`); obfuscation-ключи в `last_config` — **короткие** (`Jc/Jmin/Jmax/
  S1–S4/H1–H4/I1–I5`), значения **строками**; base64url **без паддинга** (`RawURLEncoding`); `qCompress`
  level 8; добавлены поля `mtu/persistent_keep_alive/allowed_ips/clientId/client_ip`, убран top-level
  `port`. Импорт у AmneziaVPN проходит по наличию `containers` (config_version не нужен). **Кросс-проверка:**
  независимый Python-декодер (как importController) успешно разобрал сгенерированную ссылку — все поля/типы
  совпали.
- **`DefaultParams()` — точные дефолты AmneziaVPN 2.0** (`Jc=3,Jmin=10,Jmax=30; S1=15,S2=18,S3=20,S4=23;
  H1..H4` из protocols_defs; `I1` = крафт-пакет «под iCloud DNS»). Разделение серверного/клиентского
  `.conf` подтверждено серверным шаблоном Amnezia (`server_scripts/awg/`): на сервере активны
  `Jc/S/H`, а `I1–I5` закомментированы → мой сервер их не пишет (верно), клиент пишет (верно).
- **Sync-движок:** `applyEngine` теперь `enable`+`restart` (динамический `awg-quick@awgN` переживает
  ребут ноды); добавлен `reconcileAWG` — удаление/выключение AWG-inbound-а сносит осиротевший сервис
  и конфиг (`systemctl disable --now` + `rm` + чистка idempotency-ключа), т.к. per-interface сервисы
  сами не «самолечатся» как единый xray/sing-box. Новый тест на тир-даун + `store.DeleteSetting`.
- **Провижининг ноды переработан** (`CmdInstallAWG`): основной путь — DKMS-модуль ядра из PPA
  `ppa:amnezia/ppa` (`amneziawg` + `amneziawg-tools`, нужны `linux-headers-$(uname -r)`), Go не требуется;
  fallback (Debian/без заголовков) — сборка tools из исходников + userspace `amneziawg-go` с установкой
  свежего Go (apt-овый часто слишком стар для `@latest`). Итоговая строка `[awg] tools/kmod/go` для
  проверки на деплое. Скрипт валиден `bash -n`/`sh -n`.

**Проверено:** backend `go build/vet/gofmt/test` — 13 пакетов зелёные; frontend `build/lint/test` (7/7);
превью AWG-UI в браузере — форма inbound (порт 51820, AWG-режим, скрыты reality/sniffing/preview) и
секция клиента (`vpn://` + QR + скачивание `.conf`), консоль чистая. **Осталось:** проверить `[awg]`-статус
провижина на живой ноде при деплое → затем `git push` + редеплой на VPS.

## Пост-9 — Отправка конфига клиенту в Telegram ✅ (v0.9.5.0)

Клиент получает свою подписку прямо в личку Telegram. Bot API не даёт боту писать пользователю
первым, поэтому — привязка по персональному deep-link: панель отдаёт клиенту ссылку
`t.me/<бот>?start=<токен>`; клиент жмёт **Start** → фоновый long-poll `getUpdates` на **том же
бот-токене, что и авто-бэкап**, ловит `/start <токен>`, сохраняет `chat_id` и **сразу отправляет
конфиг** (ссылка-подписка + QR, ссылка в `<code>` для тапа-копирования). Дальше оператор может
переслать конфиг в любой момент кнопкой на карточке клиента.

- Аддитивная миграция `0005` (`tg_chat_id`/`tg_username`/`tg_link_token`; существующим клиентам токен
  выдаётся сразу через `randomblob`). Новый движок/протокол не заводился — только client-level поля.
- Bot: `SendMessageTo`/`SendPhotoTo`/`SendClientConfig`/`BotUsername` (getMe, кэш) + `RunLinkPoller`
  (офсет `getUpdates` персистится — рестарт не пере-доставляет привязки). Эндпоинты
  `GET /clients/{id}/telegram-link`, `POST …/telegram-config`, `POST …/telegram-unlink`.
- UI: секция «Telegram» на вкладке «Подключение» — персональная ссылка, статус привязки (`@username`),
  «Отправить конфиг» / «Отвязать».

**Проверено:** backend `go build/vet/gofmt/test` зелёные (+ тесты store-линковки, service token/QR,
разбор апдейтов и линковки поллера); frontend `tsc/lint/build/test` (7/7); превью — привязанное
(`@nick_tg`) и непривязанное состояние, тост отправки, консоль чистая.

## Пост-9 — Самообслуживание клиента через бота ✅ (v0.9.6.0)

Привязанный клиент теперь может **сам** получить свой конфиг из бота, без участия оператора:
постоянная reply-кнопка «🔄 Получить конфиг» + команда `/config` (синонимы `/getconfig`, `/link`,
`/get`, `/sub`). Поллер по chat_id находит клиента (`GetClientByTelegramChatID`) и переиспользует ту
же доставку, что и при первой привязке, — всегда актуальная ссылка + QR. Повторный `/start` от уже
привязанного чата тоже пере-отправляет конфиг; непривязанный чат получает подсказку открыть ссылку.
Команда `/config` регистрируется в меню бота (`setMyCommands`), клавиатура крепится к каждому конфигу.

- Backend: `GetClientByTelegramChatID`, `sendPhoto(...reply_markup)`, `SendClientConfig` с клавиатурой,
  `isConfigRequest`, обработка команды/кнопки/bare-`/start` в `handleUpdate`, `setMyCommands`.
- UI: подсказка оператору под бейджем привязки, что клиент может запросить конфиг сам.

**Проверено:** backend `go build/vet/gofmt/test` зелёные (+ тесты: reverse-lookup по chat_id,
config-команда/кнопка/bare-start, клавиатура в конфиге); frontend `tsc/lint/build/test` (7/7);
превью — подсказка на месте, версия `0.9.6.0`, консоль чистая.

## Пост-9 — Страница «Система» (метрики хоста панели) ✅ (v0.9.7.0)

Отдельная страница после «Настроек» (иконка пульса; дефолт по-прежнему «Серверы») показывает нагрузку
на сервер, где работает панель: CPU (load / ядра), память, диск, аптайм — карточки с барами (жёлтый
>70%, красный >90%), средняя нагрузка 1/5/15 + Swap, и блок «Сервер» (домен, ядра, RAM, диск, ядро,
архитектура, версия). Автообновление каждые 4 сек.

- Backend: пакет `internal/sysinfo` (чтение `/proc/loadavg|meminfo|uptime` + `statfs`; на Linux `/proc`
  отражает **хост**, не контейнер) + эндпоинт `GET /api/system` (`systemHandler`, отдаёт метрики +
  версию + домен). Чистый Go, без зависимостей; парсеры покрыты тестами.
- Frontend: `useSystemInfo` (poll 4с), страница `System.tsx`, роут `/system`, пункт навигации.

**Проверено:** backend `go build/vet/gofmt/test` зелёные (+ `parseLoadAvg/MemInfo/Uptime`, `Collect`);
frontend `tsc/lint/build/test` (7/7); превью — 4 карточки в ряд на десктопе (grid xl:4) и стопкой на
узком, бары/аптайм/детали на месте, версия `0.9.7.0`, консоль чистая.
