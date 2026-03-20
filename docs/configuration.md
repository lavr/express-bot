# Конфигурация

Полный справочник по настройке express-botx.

## Приоритет загрузки

Параметры загружаются слоями, каждый следующий перекрывает предыдущий:

1. **YAML-файл** (`--config`, `EXPRESS_BOTX_CONFIG`, `./express-botx.yaml` или `<os.UserConfigDir>/express-botx/config.yaml`)

Автопоиск использует платформенный каталог из `os.UserConfigDir()`:

- Linux: `~/.config/express-botx/config.yaml`
- macOS: `~/Library/Application Support/express-botx/config.yaml`
- Windows: `%AppData%/express-botx/config.yaml`
2. **Переменные окружения**
3. **Флаги командной строки**

## Файл конфигурации

```yaml
bots:
  deploy-bot:
    host: express.company.ru              # или http://localhost:8080 для dev
    id: 054af49e-5e18-4dca-ad73-4f96b6de63fa
    secret: my-bot-secret
  alert-bot:
    host: express.company.ru
    id: 99887766-5544-3322-1100-aabbccddeeff
    token: vault:secret/data/express#alert_token  # статический токен (альтернатива secret)

chats:
  # Короткая форма: только UUID
  chat1: 1a2b3c4d-5e6f-7890-abcd-ef1234567890
  chat2: 2a2b3c4d-6e6f-8890-bbcd-ff1234567890

  # С привязкой к боту: бот подставляется автоматически
  deploy:
    id: 2b3c4d5e-6f7a-8901-bcde-f12345678901
    bot: deploy-bot
  alerts:
    id: 3c4d5e6f-7a8b-9012-cdef-123456789012
    bot: alert-bot

  # Чат по умолчанию — используется когда --chat-id / chat_id не указан
  general:
    id: 4d5e6f7a-8b9c-0123-def0-234567890123
    default: true

cache:
  type: file                              # none | file | vault (по умолчанию: file)
  file_path: $TMPDIR/express-botx-token    # поддерживает переменные окружения
  ttl: 31536000                           # секунды (по умолчанию: 1 год)

server:
  listen: ":8080"
  base_path: /api/v1
  api_keys:
    - name: monitoring
      key: env:MONITORING_API_KEY
  alertmanager:
    default_chat_id: alerts               # UUID или алиас (опционально)
    error_severities: [critical, warning] # по умолчанию
  grafana:
    default_chat_id: alerts
    error_states: [alerting]              # по умолчанию
```

## Переменные окружения

| Переменная | Описание |
|---|---|
| `EXPRESS_BOTX_CONFIG` | Путь к файлу конфигурации |
| `EXPRESS_BOTX_HOST` | Хост сервера eXpress (или URL: `http://host:port`) |
| `EXPRESS_BOTX_BOT_ID` | UUID бота |
| `EXPRESS_BOTX_SECRET` | Секрет бота |
| `EXPRESS_BOTX_TOKEN` | Токен бота (альтернатива секрету) |
| `EXPRESS_BOTX_CACHE_TYPE` | Тип кэша: `none`, `file`, `vault` |
| `EXPRESS_BOTX_CACHE_FILE_PATH` | Путь к файлу кэша токенов |
| `EXPRESS_BOTX_CACHE_TTL` | TTL кэша в секундах |
| `EXPRESS_BOTX_SERVER_LISTEN` | Адрес для прослушивания (serve) |
| `EXPRESS_BOTX_SERVER_BASE_PATH` | Базовый путь (serve) |
| `EXPRESS_BOTX_SERVER_API_KEY` | API-ключ (serve) |
| `EXPRESS_BOTX_VERBOSE` | Уровень логирования: 1-3 |

## Аутентификация

Бот может аутентифицироваться двумя способами.

### Secret (динамический токен)

Приложение хранит `secret` и получает токен через BotX API при каждом запуске. При 401 — автоматический refresh.

```yaml
bots:
  mybot:
    host: express.company.ru
    id: 054af49e-...
    secret: my-secret  # или env:VAR, или vault:path#key
```

```bash
express-botx send --secret "my-secret" --host h --bot-id ID "Hello"
```

### Token (статический токен)

Приложение хранит готовый токен, без обращения к API. Токены eXpress бессрочные. При 401 — ошибка (refresh невозможен без secret).

```yaml
bots:
  mybot:
    host: express.company.ru
    id: 054af49e-...
    token: eyJhbGci...  # или env:VAR, или vault:path#key
```

```bash
express-botx send --token "TOKEN" --host h --bot-id ID "Hello"
```

### Обмен secret на token

По умолчанию `config bot add` обменивает secret на token через API и сохраняет **только token** (secure by default):

```bash
# Secret → token (secret не сохраняется)
express-botx config bot add --host h --bot-id ID --secret SECRET

# Сохранить secret как есть
express-botx config bot add --host h --bot-id ID --secret SECRET --save-secret
```

## Форматы значений секретов

`--secret`, `--token` и поля `secret`/`token` в конфиге поддерживают:

```bash
# Литеральное значение
express-botx send --secret "my-secret-key" "Hello"

# Из переменной окружения
express-botx send --token env:MY_TOKEN "Hello"

# Из HashiCorp Vault (KV v2)
express-botx send --secret "vault:secret/data/express#bot_secret" "Hello"
```

Для Vault необходимы переменные `VAULT_ADDR` и `VAULT_TOKEN`.

## Мульти-бот конфигурация

При нескольких ботах выбор бота определяется по приоритету:

1. Явный `--bot` (CLI) или `"bot"` (API) / `?bot=` (webhooks)
2. Привязка бота к чату (`chats.deploy.bot: deploy-bot`)
3. Единственный бот (авто-выбор)
4. Ошибка

```bash
# Бот из привязки чата — --bot не нужен
express-botx send --chat-id deploy "OK"

# Явный --bot переопределяет привязку
express-botx send --bot alert-bot --chat-id deploy "Срочно!"

# HTTP API — аналогично
curl /api/v1/send -d '{"chat_id":"deploy","message":"OK"}'
curl /api/v1/send -d '{"bot":"alert-bot","chat_id":"deploy","message":"!"}'
curl /api/v1/alertmanager?bot=deploy-bot
```

## Чат по умолчанию

Один чат можно пометить как `default: true`. Он будет использоваться когда `--chat-id` (CLI) или `chat_id` (API) не указан:

```bash
# Управление через CLI
express-botx config chat add --chat-id UUID --alias general --default
express-botx config chat set general UUID --default
express-botx config chat set general UUID --no-default   # снять пометку
express-botx config chat list                             # покажет (default)
```

Приоритет выбора чата в HTTP-сервере:
- `/send`: `chat_id` из запроса → чат по умолчанию → ошибка
- `/alertmanager`, `/grafana`: `?chat_id=` → `default_chat_id` из конфига вебхука → чат по умолчанию → единственный чат → ошибка

## Формат host

```yaml
bots:
  prod:
    host: express.company.ru       # → https://express.company.ru
  local:
    host: http://localhost:8080    # HTTP + порт
  staging:
    host: https://staging.company.ru:8443
```

## Кэширование токенов

По умолчанию токен кэшируется в файл `.express-botx-token-cache.json` в текущей директории (TTL — 1 год).

### Файловый кэш

```yaml
cache:
  type: file
  file_path: $TMPDIR/express-botx-token  # опционально, поддерживает env vars
  ttl: 31536000
```

### Vault кэш

```yaml
cache:
  type: vault
  vault_url: https://vault.example.com
  vault_path: secret/data/express-botx/tokens
  ttl: 31536000
```

### Отключение кэша

```bash
express-botx send --no-cache "Hello"
```

Или в конфиге: `cache.type: none`.

## Конфигурация очереди (async-режим)

Для `enqueue`, `serve --enqueue` и `worker` нужна секция `queue` и, в зависимости от роли, `producer`, `worker` и `catalog`:

### Producer (enqueue / serve --enqueue)

```yaml
queue:
  driver: kafka           # или rabbitmq
  url: broker:9092
  name: express-botx
  reply_queue: express-botx-replies

producer:
  routing_mode: mixed      # direct | catalog | mixed

catalog:
  queue_name: express-botx-catalog
  cache_file: /var/lib/express-botx/catalog.json
  max_age: 10m
```

### Worker

```yaml
queue:
  driver: kafka
  url: broker:9092
  name: express-botx
  group: express-botx

worker:
  retry_count: 3
  retry_backoff: 1s
  shutdown_timeout: 30s
  health_listen: ":8081"

catalog:
  queue_name: express-botx-catalog
  publish: true
  publish_interval: 30s

bots:
  alerts:
    host: express.company.ru
    id: bot-uuid
    secret: env:ALERTS_SECRET
```

Producer не нужны `secret`, `token` и полный список ботов — он не аутентифицируется в BotX API.
