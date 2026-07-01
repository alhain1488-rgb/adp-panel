# PROGRESS — журнал выполнения

Короткие записи по завершении каждой фазы: что сделано, что проверено. Ведёт Claude Code.

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
