# express-botx

CLI и HTTP-сервер для отправки сообщений в корпоративный мессенджер [eXpress](https://express.ms) через BotX API.

Принимает вебхуки от Alertmanager и Grafana, поддерживает асинхронную отправку через RabbitMQ/Kafka, работает как утилита командной строки или HTTP-сервис.

## Возможности

- **Отправка сообщений** из CLI, скриптов, пайплайнов CI/CD
- **HTTP-сервер** с API для отправки и приёма вебхуков
- **Alertmanager и Grafana** — готовые эндпоинты для мониторинга
- **Асинхронная очередь** — RabbitMQ или Kafka для надёжной доставки
- **Секреты** — поддержка переменных окружения и HashiCorp Vault
- **Kubernetes-ready** — Docker, Helm chart, бинарник


## Quick Start

### Установка бинарной сборки

```bash
curl -sL "https://github.com/lavr/express-botx/releases/latest/download/express-botx-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/').tar.gz" | tar xz
sudo mv express-botx /usr/local/bin/
```

Проект также можно установить из homebrew, собрать из исходников, запустить в готовом контейнере.

### Создание конфига

В конфиге можно сохранить параметры бота и параметры чатов.

Добавить параметры бота в конфиг:
 
```bash
express-botx config bot add \
  --name mybot \
  --host express.company.ru \
  --bot-id 054af49e-5e18-4dca-ad73-4f96b6de63fa \
  --secret my-bot-secret
```

Добавить параметры чата в конфиг:
 
```bash
express-botx config chat add \
  --chat-id aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa \
  --alias alerts
```

Теперь можно отправить сообщение:

```bash
express-botx send "Привет из express-botx!"
```

## Установка

### Бинарник с GitHub

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
curl -sL "https://github.com/lavr/express-botx/releases/latest/download/express-botx-${OS}-${ARCH}.tar.gz" | tar xz
sudo mv express-botx /usr/local/bin/
```

### Homebrew

```bash
brew install lavr/tap/express-botx
```

### Docker

```bash
docker pull lavr/express-botx
```

### Go

```bash
go install github.com/lavr/express-botx@latest
```

### Из исходников

```bash
git clone https://github.com/lavr/express-botx.git
cd express-botx
go build -o express-botx .

# С поддержкой RabbitMQ / Kafka
go build -tags "rabbitmq kafka" -o express-botx .
```

## Использование

### Отправка сообщений (send)

```bash
# Текст как аргумент
express-botx send "Сборка #42 прошла успешно"

# Из stdin
echo "Deploy OK" | express-botx send

# Из файла
express-botx send --body-from report.txt

# С вложением
express-botx send --file report.pdf "Отчёт за март"

# В конкретный чат со статусом ошибки
express-botx send --chat-id alerts "Диск заполнен на 95%" --status error
```

При успехе — exit 0 (молча). При ошибке — сообщение в stderr, exit 1.

### HTTP-сервер (serve)

```bash
# Запуск сервера
express-botx serve

# На другом порту
express-botx serve --listen :9090
```

Эндпоинты (все POST требуют `Authorization: Bearer <key>`):

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/healthz` | Проверка здоровья |
| `POST` | `/api/v1/send` | Отправка сообщения |
| `POST` | `/api/v1/alertmanager` | Вебхук Alertmanager |
| `POST` | `/api/v1/grafana` | Вебхук Grafana |

```bash
# Отправка через API
curl -X POST http://localhost:8080/api/v1/send \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"chat_id": "alerts", "message": "Deploy OK"}'
```

### Очереди (enqueue / worker)

Для надёжной асинхронной доставки express-botx поддерживает работу через RabbitMQ или Kafka. HTTP-сервер кладёт сообщения в очередь, worker забирает и отправляет в BotX API.

```bash
# Producer: HTTP → очередь
express-botx serve --enqueue

# Consumer: очередь → BotX API
express-botx worker
```

Подробнее: [docs/async-queues.md](docs/async-queues.md)

### Управление конфигурацией

```bash
express-botx config bot add --name prod --host express.company.ru --bot-id UUID --secret SECRET
express-botx config chat add --chat-id UUID --alias deploy --bot prod
express-botx config apikey add --name monitoring
express-botx config show
```

Полный список команд: [docs/commands.md](docs/commands.md)

## Конфигурация

Минимальный конфиг (`<os.UserConfigDir>/express-botx/config.yaml`, например `~/.config/express-botx/config.yaml` на Linux или `~/Library/Application Support/express-botx/config.yaml` на macOS):

```yaml
bots:
  prod:
    host: express.company.ru
    id: 054af49e-5e18-4dca-ad73-4f96b6de63fa
    token: eyJhbGci...

chats:
  alerts:
    id: aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
    bot: prod
    default: true
```

Параметры загружаются слоями (каждый следующий перекрывает предыдущий):

1. **YAML-файл** (`--config`, `EXPRESS_BOTX_CONFIG`, `./express-botx.yaml`, `<os.UserConfigDir>/express-botx/config.yaml`)
2. **Переменные окружения** (`EXPRESS_BOTX_HOST`, `EXPRESS_BOTX_BOT_ID`, ...)
3. **Флаги командной строки** (`--host`, `--bot-id`, ...)

Полный референс конфигурации: [docs/configuration.md](docs/configuration.md)

## Интеграции

### Alertmanager

```yaml
# alertmanager.yml
receivers:
  - name: express
    webhook_configs:
      - url: http://express-botx:8080/api/v1/alertmanager
        send_resolved: true
        http_config:
          bearer_token: "<api-key>"
```

### Grafana

Contact point → Webhook:
- **URL:** `http://express-botx:8080/api/v1/grafana`
- **Authorization Header:** `Bearer <api-key>`

Подробнее: [docs/integrations.md](docs/integrations.md)

## Деплой

### Docker

```bash
# HTTP-сервер
docker run -d -p 8080:8080 -v ./config.yaml:/config.yaml \
  lavr/express-botx serve --config /config.yaml

# Отправка из CLI
docker run --rm lavr/express-botx send \
  --host express.company.ru --bot-id UUID --secret KEY \
  --chat-id UUID "Hello from Docker"
```

### Kubernetes (Helm)

```bash
helm install express-botx oci://ghcr.io/lavr/charts/express-botx -f values.yaml
```

### systemd

```ini
# /etc/systemd/system/express-botx.service
[Unit]
Description=express-botx HTTP server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/express-botx serve --config /etc/express-botx/config.yaml
Restart=always
RestartSec=5
User=express-botx
Group=express-botx

[Install]
WantedBy=multi-user.target
```

Подробнее: [docs/deployment.md](docs/deployment.md)

## Документация

| Документ | Описание |
|----------|----------|
| [docs/commands.md](docs/commands.md) | Все команды и флаги |
| [docs/configuration.md](docs/configuration.md) | Полный референс конфигурации |
| [docs/integrations.md](docs/integrations.md) | Alertmanager, Grafana, примеры |
| [docs/deployment.md](docs/deployment.md) | Docker, Helm, systemd, docker-compose |
| [docs/async-queues.md](docs/async-queues.md) | RabbitMQ, Kafka, архитектура очередей |
| [QUICKSTART.md](QUICKSTART.md) | Пошаговые сценарии настройки |

## Лицензия

MIT
