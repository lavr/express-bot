#!/usr/bin/env bash
#
# Пример обработчика callback-ов от Express Platform.
#
# Скрипт получает JSON payload через stdin и метаданные через env-переменные:
#   EXPRESS_CALLBACK_EVENT    — тип события (message, chat_created, cts_login, ...)
#   EXPRESS_CALLBACK_SYNC_ID  — sync_id запроса
#   EXPRESS_CALLBACK_BOT_ID   — bot_id из payload
#   EXPRESS_CALLBACK_CHAT_ID  — group_chat_id из from
#   EXPRESS_CALLBACK_USER_HUID — user_huid из from (если есть)
#
# Использование:
#   Указать в config-local.yaml:
#     server:
#       callbacks:
#         rules:
#           - events: [chat_created, added_to_chat]
#             handler:
#               type: exec
#               command: ./examples/callback-handler.sh
#
# Exit code 0 = успех, != 0 = ошибка (логируется express-botx).

set -euo pipefail

EVENT="${EXPRESS_CALLBACK_EVENT:-unknown}"
SYNC_ID="${EXPRESS_CALLBACK_SYNC_ID:-}"
BOT_ID="${EXPRESS_CALLBACK_BOT_ID:-}"
CHAT_ID="${EXPRESS_CALLBACK_CHAT_ID:-}"
USER_HUID="${EXPRESS_CALLBACK_USER_HUID:-}"

echo "[callback-handler] event=${EVENT} sync_id=${SYNC_ID} bot=${BOT_ID} chat=${CHAT_ID} user=${USER_HUID}"

# Читаем JSON payload из stdin
PAYLOAD=$(cat)

case "$EVENT" in
    chat_created|added_to_chat)
        echo "[callback-handler] Membership event: ${EVENT}"
        echo "$PAYLOAD" | jq -r '.command.body // empty' 2>/dev/null || true
        ;;
    cts_login|cts_logout)
        echo "[callback-handler] Auth event: ${EVENT}"
        ;;
    notification_callback)
        echo "[callback-handler] Delivery notification received"
        echo "$PAYLOAD" | jq -r '.status // empty' 2>/dev/null || true
        ;;
    message)
        echo "[callback-handler] Message received"
        echo "$PAYLOAD" | jq -r '.command.body // empty' 2>/dev/null || true
        ;;
    *)
        echo "[callback-handler] Unhandled event: ${EVENT}"
        ;;
esac

exit 0
