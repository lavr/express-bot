# Интеграции

Подключение express-botx к системам мониторинга.

## Alertmanager

### Настройка express-botx

В конфиге укажите чат для алертов:

```yaml
server:
  listen: ":8080"
  base_path: /api/v1
  api_keys:
    - name: alertmanager
      key: env:ALERTMANAGER_API_KEY
  alertmanager:
    default_chat_id: alerts         # чат по умолчанию для алертов
    error_severities:               # при каких severity ставить статус "error"
      - critical
      - warning
```

### Настройка Alertmanager

Добавьте receiver в `alertmanager.yml`:

```yaml
receivers:
  - name: express
    webhook_configs:
      - url: http://express-botx:8080/api/v1/alertmanager
        send_resolved: true
        http_config:
          bearer_token: "<api-key>"

route:
  receiver: express
  # Или для конкретных алертов:
  routes:
    - match:
        severity: critical
      receiver: express
```

### Несколько чатов

Если нужно отправлять разные алерты в разные чаты, используйте разные receiver'ы с query-параметром `chat_id`:

```yaml
receivers:
  - name: express-infra
    webhook_configs:
      - url: http://express-botx:8080/api/v1/alertmanager?chat_id=infra-alerts
        send_resolved: true
        http_config:
          bearer_token: "<api-key>"

  - name: express-app
    webhook_configs:
      - url: http://express-botx:8080/api/v1/alertmanager?chat_id=app-alerts
        send_resolved: true
        http_config:
          bearer_token: "<api-key>"

route:
  routes:
    - match:
        team: infra
      receiver: express-infra
    - match:
        team: app
      receiver: express-app
```

### Проверка вручную

```bash
curl -X POST http://localhost:8080/api/v1/alertmanager \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "firing",
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "HighCPU",
          "severity": "critical",
          "instance": "server1:9090"
        },
        "annotations": {
          "summary": "CPU usage is above 90%"
        },
        "startsAt": "2026-01-01T00:00:00Z",
        "endsAt": "0001-01-01T00:00:00Z"
      }
    ]
  }'
```

---

## Grafana

### Настройка express-botx

```yaml
server:
  listen: ":8080"
  base_path: /api/v1
  api_keys:
    - name: grafana
      key: env:GRAFANA_API_KEY
  grafana:
    default_chat_id: alerts           # чат по умолчанию
    error_states:                     # при каких состояниях ставить статус "error"
      - alerting
```

### Настройка Grafana

1. Перейдите в **Alerting → Contact points → Add contact point**
2. Выберите тип **Webhook**
3. Заполните:
   - **URL:** `http://express-botx:8080/api/v1/grafana`
   - **HTTP Method:** POST
   - **Authorization Header:** `Bearer <api-key>`
4. Сохраните и привяжите к notification policy

### Несколько чатов

Аналогично Alertmanager — создайте несколько contact point'ов с `?chat_id=`:

- `http://express-botx:8080/api/v1/grafana?chat_id=infra-alerts`
- `http://express-botx:8080/api/v1/grafana?chat_id=app-alerts`

### Проверка вручную

```bash
curl -X POST http://localhost:8080/api/v1/grafana \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "firing",
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "DiskFull"
        },
        "annotations": {
          "summary": "Disk usage above 90%"
        },
        "startsAt": "2026-01-01T00:00:00Z",
        "dashboardURL": "https://grafana.company.ru/d/abc123"
      }
    ]
  }'
```

---

## Произвольные вебхуки через /send

Для систем без специальных эндпоинтов используйте `/send`:

```bash
# JSON
curl -X POST http://express-botx:8080/api/v1/send \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": "deploy",
    "message": "Deploy v1.2.3 completed",
    "status": "ok"
  }'

# С файлом (multipart)
curl -X POST http://express-botx:8080/api/v1/send \
  -H "Authorization: Bearer <api-key>" \
  -F "chat_id=deploy" \
  -F "message=Отчёт за март" \
  -F "file=@report.pdf"
```

### Примеры интеграций

**GitLab CI:**

```yaml
notify:
  stage: notify
  script:
    - |
      curl -sf -X POST "$EXPRESS_BOTX_URL/api/v1/send" \
        -H "Authorization: Bearer $EXPRESS_BOTX_API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"chat_id\": \"deploy\", \"message\": \"Deploy $CI_PROJECT_NAME:$CI_COMMIT_TAG completed\"}"
```

**Jenkins Pipeline:**

```groovy
post {
    success {
        sh '''
            curl -sf -X POST "$EXPRESS_BOTX_URL/api/v1/send" \
              -H "Authorization: Bearer $EXPRESS_BOTX_API_KEY" \
              -H "Content-Type: application/json" \
              -d '{"chat_id": "deploy", "message": "Build #'"$BUILD_NUMBER"' OK"}'
        '''
    }
}
```

**Bash-скрипт (cron):**

```bash
#!/bin/bash
# Мониторинг места на диске
USAGE=$(df -h / | awk 'NR==2 {print $5}' | tr -d '%')
if [ "$USAGE" -gt 90 ]; then
    express-botx send --chat-id alerts --status error \
        "Диск заполнен на ${USAGE}% на $(hostname)"
fi
```
