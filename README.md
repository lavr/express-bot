# express-bot

CLI и HTTP-сервер для отправки сообщений в корпоративный мессенджер eXpress через BotX API.

Поддерживает вебхуки от Alertmanager и Grafana.

## Установка

### Homebrew (macOS / Linux)

```bash
brew install lavr/tap/express-bot
```

### Docker

```bash
docker pull lavr/express-bot
```

### Go

```bash
go install github.com/lavr/express-bot@latest
```

### Из исходников

```bash
git clone https://github.com/lavr/express-bot.git
cd express-bot
go build -o express-bot .
```

## Команды

| Команда | Описание |
|---|---|
| `send` | Отправить сообщение и/или файл в чат |
| `serve` | Запустить HTTP-сервер (API + вебхуки) |
| `chats list` | Показать список чатов бота |
| `chats info` | Показать детальную информацию о чате |
| `chats alias` | Управление алиасами чатов (set, list, rm) |
| `bot ping` | Проверить авторизацию и доступность API |
| `bot info` | Показать информацию о боте |
| `bot list` | Показать боты из конфига |
| `bot add` | Добавить бота в конфиг |
| `bot rm` | Удалить бота из конфига |
| `user search` | Найти пользователя по email, HUID или AD-логину |

## send — отправка сообщений

```bash
# Текст как аргумент
express-bot send "Сборка #42 прошла успешно"

# Из файла
express-bot send --body-from report.txt

# Из stdin
echo "Deploy OK" | express-bot send

# С файлом-вложением
express-bot send --file report.pdf "Отчёт за март"

# Файл из stdin
cat image.png | express-bot send --file - --file-name image.png

# Все параметры через флаги
express-bot send --host express.company.ru --bot-uuid UUID --secret KEY --chat-id UUID "Hello"
```

При успехе утилита завершается молча (exit 0). Ошибки выводятся в stderr (exit 1).

### Флаги send

```
--chat-id       UUID или алиас целевого чата
--body-from     прочитать сообщение из файла
--file          путь к файлу-вложению (или - для stdin)
--file-name     имя файла (обязательно при --file -)
--status        статус уведомления: ok или error (по умолчанию: ok)
--silent        без push-уведомления получателю
--stealth       стелс-режим (сообщение видно только боту)
--force-dnd     доставить даже при DND
--no-notify     не отправлять уведомление вообще
--metadata      произвольный JSON для notification.metadata
```

## serve — HTTP-сервер

Запускает HTTP-сервер с эндпоинтами для отправки сообщений и приёма вебхуков.

```bash
express-bot serve --config config.yaml
express-bot serve --config config.yaml --listen :9090
express-bot serve --config config.yaml --api-key env:MY_API_KEY
```

### Эндпоинты

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/healthz` | Проверка здоровья |
| `POST` | `{basePath}/send` | Отправка сообщения (JSON / multipart) |
| `POST` | `{basePath}/alertmanager` | Приём вебхуков от Alertmanager |
| `POST` | `{basePath}/grafana` | Приём вебхуков от Grafana |

Все `POST`-эндпоинты требуют авторизации: `Authorization: Bearer <key>` или `X-API-Key: <key>`.

### Docker

```bash
# Отправить сообщение
docker run --rm lavr/express-bot send \
  --host express.company.ru --bot-uuid UUID --secret KEY \
  --chat-id UUID "Hello from Docker"

# С конфигом
docker run --rm -v ./config.yaml:/config.yaml lavr/express-bot \
  send --config /config.yaml --chat-id UUID "Hello"

# HTTP-сервер
docker run --rm -p 8080:8080 -v ./config.yaml:/config.yaml lavr/express-bot \
  serve --config /config.yaml
```

## Конфигурация

Параметры загружаются слоями, каждый следующий перекрывает предыдущий:

1. **YAML-файл** (`express-bot.yaml` в текущей директории или `~/.config/express-bot/config.yaml`)
2. **Переменные окружения**
3. **Флаги командной строки**

### Файл конфигурации

```yaml
bots:
  mybot:
    host: express.company.ru
    id: 054af49e-5e18-4dca-ad73-4f96b6de63fa
    secret: my-bot-secret

chats:
  ops-alerts: 1a2b3c4d-5e6f-7890-abcd-ef1234567890
  deploy: 2b3c4d5e-6f7a-8901-bcde-f12345678901

cache:
  type: file                              # none | file | vault (по умолчанию: file)
  file_path: $TMPDIR/express-bot-token    # поддерживает переменные окружения
  ttl: 31536000                           # секунды (по умолчанию: 1 год)

server:
  listen: ":8080"
  base_path: /api/v1
  api_keys:
    - name: monitoring
      key: env:MONITORING_API_KEY
  alertmanager:
    default_chat_id: ops-alerts           # UUID или алиас (опционально)
    error_severities: [critical, warning] # по умолчанию
  grafana:
    default_chat_id: ops-alerts
    error_states: [alerting]              # по умолчанию
```

По умолчанию кэш пишется в файл `.express-bot-token-cache.json` в текущей директории.

Путь к конфигу можно указать явно: `--config /path/to/config.yaml`

### Переменные окружения

| Переменная | Описание |
|---|---|
| `EXPRESS_HOST` | Хост сервера eXpress |
| `EXPRESS_BOT_ID` | UUID бота |
| `EXPRESS_SECRET` | Секрет бота |
| `EXPRESS_CACHE_TYPE` | Тип кэша: `none`, `file`, `vault` |
| `EXPRESS_CACHE_FILE_PATH` | Путь к файлу кэша токенов |
| `EXPRESS_CACHE_TTL` | TTL кэша в секундах |
| `EXPRESS_SERVER_LISTEN` | Адрес для прослушивания (serve) |
| `EXPRESS_SERVER_BASE_PATH` | Базовый путь (serve) |
| `EXPRESS_SERVER_API_KEY` | API-ключ (serve) |

### Общие флаги

```
--host          хост сервера eXpress
--bot-uuid      UUID бота
--bot           имя бота из конфига
--secret        секрет бота (литерал, env:VAR или vault:path#key)
--config        путь к файлу конфигурации
--no-cache      отключить кэширование токена
--format        формат вывода: text или json (по умолчанию: text)
-v / -vv / -vvv уровень подробности логирования
```

## Секреты

Значение `--secret` (и поле `secret` в конфиге) поддерживает три формата:

```bash
# Литеральное значение
express-bot send --secret "my-secret-key" "Hello"

# Из переменной окружения
express-bot send --secret env:EXPRESS_BOT_SECRET "Hello"

# Из HashiCorp Vault (KV v2)
express-bot send --secret "vault:secret/data/express#bot_secret" "Hello"
```

Для Vault необходимы переменные `VAULT_ADDR` и `VAULT_TOKEN`.

## Кэширование токенов

По умолчанию токен кэшируется в файл `.express-bot-token-cache.json` в текущей директории (TTL — 1 год).

### Файловый кэш

```yaml
cache:
  type: file
  file_path: $TMPDIR/express-bot-token  # опционально, поддерживает env vars
  ttl: 31536000
```

### Vault кэш

```yaml
cache:
  type: vault
  vault_url: https://vault.example.com
  vault_path: secret/data/express-bot/tokens
  ttl: 31536000
```

### Отключение кэша

```bash
express-bot send --no-cache "Hello"
```

Или в конфиге: `cache.type: none`.

## Тестирование

```bash
go test ./...
```
