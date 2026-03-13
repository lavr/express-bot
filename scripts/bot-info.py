#!/usr/bin/env python3
"""Get bot info and chat list from eXpress BotX API."""

import argparse
import hashlib
import hmac
import json
import sys
import urllib.request


def build_signature(bot_id: str, secret: str) -> str:
    sig = hmac.new(secret.encode(), bot_id.encode(), hashlib.sha256).digest()
    return sig.hex().upper()


def api_get(host: str, path: str, token: str | None = None, params: dict | None = None) -> dict:
    url = f"https://{host}{path}"
    if params:
        url += "?" + "&".join(f"{k}={v}" for k, v in params.items())

    req = urllib.request.Request(url)
    if token:
        req.add_header("Authorization", f"Bearer {token}")

    with urllib.request.urlopen(req, timeout=15) as resp:
        return json.loads(resp.read())


def get_token(host: str, bot_id: str, secret: str) -> str:
    signature = build_signature(bot_id, secret)
    data = api_get(host, f"/api/v2/botx/bots/{bot_id}/token", params={"signature": signature})
    return data["result"]


def get_chats(host: str, token: str) -> list:
    data = api_get(host, "/api/v3/botx/chats/list", token=token)
    return data.get("result", [])


def get_chat_info(host: str, token: str, chat_id: str) -> dict:
    url = f"https://{host}/api/v3/botx/chats/info"
    body = json.dumps({"group_chat_id": chat_id}).encode()

    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Content-Type", "application/json")

    with urllib.request.urlopen(req, timeout=15) as resp:
        return json.loads(resp.read())


def main():
    p = argparse.ArgumentParser(description="Get eXpress bot info and chat list")
    p.add_argument("bot_id", help="Bot UUID")
    p.add_argument("secret", help="Bot secret key")
    p.add_argument("--host", default="express.invitro.ru", help="eXpress host (default: express.invitro.ru)")
    args = p.parse_args()

    try:
        token = get_token(args.host, args.bot_id, args.secret)
    except Exception as e:
        print(f"Error getting token: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"Bot ID:  {args.bot_id}")
    print(f"Host:    {args.host}")
    print(f"Token:   {token[:20]}...")
    print()

    try:
        chats = get_chats(args.host, token)
    except Exception as e:
        print(f"Error getting chats: {e}", file=sys.stderr)
        sys.exit(1)

    if not chats:
        print("No chats found. Add the bot to a chat first.")
        return

    print(f"Chats ({len(chats)}):")
    print("-" * 72)

    for chat_id in chats:
        try:
            info = get_chat_info(args.host, token, chat_id)
            r = info.get("result", {})
            name = r.get("name", "—")
            chat_type = r.get("chat_type", "—")
            members = len(r.get("members", []))
            print(f"  {chat_id}")
            print(f"    name:    {name}")
            print(f"    type:    {chat_type}")
            print(f"    members: {members}")
        except Exception:
            print(f"  {chat_id}")
            print(f"    (could not fetch details)")
        print()


if __name__ == "__main__":
    main()
