package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/lavr/express-send/internal/auth"
	"github.com/lavr/express-send/internal/botapi"
	"github.com/lavr/express-send/internal/config"
	"github.com/lavr/express-send/internal/input"
	"github.com/lavr/express-send/internal/secret"
	"github.com/lavr/express-send/internal/token"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("subcommand required: send, chats")
	}

	switch os.Args[1] {
	case "send":
		return runSend(os.Args[2:])
	case "chats":
		return runChats(os.Args[2:])
	case "--help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}
}

// globalFlags registers flags common to all subcommands.
func globalFlags(fs *flag.FlagSet, flags *config.Flags) {
	fs.StringVar(&flags.ConfigPath, "config", "", "path to config file (default: ~/.config/express-send/config.yaml)")
	fs.StringVar(&flags.Host, "host", "", "eXpress server host")
	fs.StringVar(&flags.BotID, "bot-id", "", "bot UUID")
	fs.StringVar(&flags.Secret, "secret", "", "bot secret (literal, env:VAR, or vault:path#key)")
	fs.BoolVar(&flags.NoCache, "no-cache", false, "disable token caching")
}

func runSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	var flags config.Flags
	var messageFrom string

	globalFlags(fs, &flags)
	fs.StringVar(&flags.ChatID, "chat-id", "", "target chat UUID")
	fs.StringVar(&messageFrom, "message-from", "", "read message from file")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: express-bot send [options] [message]\n\nSend a message to an eXpress chat.\n\nMessage sources (in priority order):\n  --message-from FILE   Read message from file\n  [message]             Positional argument\n  stdin                 Pipe input (auto-detected)\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, err := config.Load(flags)
	if err != nil {
		return err
	}
	if err := cfg.RequireChatID(); err != nil {
		return err
	}

	// Read message
	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))
	message, err := input.ReadMessage(messageFrom, fs.Args(), os.Stdin, isTerminal)
	if err != nil {
		return err
	}

	// Authenticate
	tok, cache, err := authenticate(cfg)
	if err != nil {
		return err
	}

	// Send message
	client := botapi.NewClient(cfg.Host, tok)
	err = client.SendNotification(context.Background(), cfg.ChatID, message)
	if err != nil {
		// Retry once on 401 with fresh token
		if errors.Is(err, botapi.ErrUnauthorized) {
			tok, err = refreshToken(cfg, cache)
			if err != nil {
				return fmt.Errorf("refreshing token: %w", err)
			}
			client.Token = tok
			err = client.SendNotification(context.Background(), cfg.ChatID, message)
		}
		if err != nil {
			return fmt.Errorf("sending message: %w", err)
		}
	}

	return nil
}

func runChats(args []string) error {
	if len(args) == 0 {
		printChatsUsage()
		return fmt.Errorf("subcommand required: list")
	}

	switch args[0] {
	case "list":
		return runChatsList(args[1:])
	case "--help", "-h":
		printChatsUsage()
		return nil
	default:
		printChatsUsage()
		return fmt.Errorf("unknown subcommand: chats %s", args[0])
	}
}

func runChatsList(args []string) error {
	fs := flag.NewFlagSet("chats list", flag.ContinueOnError)
	var flags config.Flags

	globalFlags(fs, &flags)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: express-bot chats list [options]\n\nList chats the bot is a member of.\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, err := config.Load(flags)
	if err != nil {
		return err
	}

	tok, _, err := authenticate(cfg)
	if err != nil {
		return err
	}

	client := botapi.NewClient(cfg.Host, tok)
	chats, err := client.ListChats(context.Background())
	if err != nil {
		return fmt.Errorf("listing chats: %w", err)
	}

	if len(chats) == 0 {
		fmt.Println("No chats found. Add the bot to a chat first.")
		return nil
	}

	fmt.Printf("Chats (%d):\n", len(chats))
	fmt.Println("------------------------------------------------------------------------")

	for _, chat := range chats {
		fmt.Printf("  %s\n", chat.GroupChatID)
		fmt.Printf("    name:    %s\n", chat.Name)
		fmt.Printf("    type:    %s\n", chat.ChatType)
		fmt.Printf("    members: %d\n", len(chat.Members))
		fmt.Println()
	}

	return nil
}

func printChatsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: express-bot chats <command> [options]

Commands:
  list    List chats the bot is a member of

Run "express-bot chats <command> --help" for details on a specific command.
`)
}

// authenticate resolves the secret, gets or loads a cached token.
func authenticate(cfg *config.Config) (string, token.Cache, error) {
	secretKey, err := secret.Resolve(cfg.Secret)
	if err != nil {
		return "", nil, fmt.Errorf("resolving secret: %w", err)
	}

	signature := auth.BuildSignature(cfg.BotID, secretKey)
	cache := newCache(cfg.Cache)

	ctx := context.Background()
	cacheKey := cfg.BotID

	tok, _ := cache.Get(ctx, cacheKey)
	if tok == "" {
		tok, err = auth.GetToken(ctx, cfg.Host, cfg.BotID, signature)
		if err != nil {
			return "", nil, fmt.Errorf("getting token: %w", err)
		}
		ttl := time.Duration(cfg.Cache.TTL) * time.Second
		cache.Set(ctx, cacheKey, tok, ttl)
	}

	return tok, cache, nil
}

// refreshToken forces a fresh token from the API.
func refreshToken(cfg *config.Config, cache token.Cache) (string, error) {
	secretKey, err := secret.Resolve(cfg.Secret)
	if err != nil {
		return "", fmt.Errorf("resolving secret: %w", err)
	}

	signature := auth.BuildSignature(cfg.BotID, secretKey)
	ctx := context.Background()

	tok, err := auth.GetToken(ctx, cfg.Host, cfg.BotID, signature)
	if err != nil {
		return "", err
	}

	ttl := time.Duration(cfg.Cache.TTL) * time.Second
	cache.Set(ctx, cfg.BotID, tok, ttl)
	return tok, nil
}

func newCache(cfg config.CacheConfig) token.Cache {
	switch cfg.Type {
	case "file":
		path := cfg.FilePath
		if path == "" {
			if dir, err := os.UserCacheDir(); err == nil {
				path = dir + "/express-send/tokens.json"
			} else {
				home, _ := os.UserHomeDir()
				path = home + "/.cache/express-send/tokens.json"
			}
		}
		return &token.FileCache{Path: path}
	case "vault":
		return &token.VaultCache{
			URL:   cfg.VaultURL,
			Path:  cfg.VaultPath,
			Token: os.Getenv("VAULT_TOKEN"),
		}
	default:
		return token.NoopCache{}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: express-bot <command> [options]

Commands:
  send    Send a message to an eXpress chat
  chats   Manage chats (list, ...)

Run "express-bot <command> --help" for details on a specific command.
`)
}
