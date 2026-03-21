package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lavr/express-botx/internal/botapi"
	"github.com/lavr/express-botx/internal/config"
	vlog "github.com/lavr/express-botx/internal/log"
	"github.com/lavr/express-botx/internal/token"
)

// ExitError allows commands to exit with a specific code without printing
// "error: ..." to stderr. Used by api command for non-2xx HTTP responses.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// maxRequestBodySize is the maximum allowed request body size (50 MB).
const maxRequestBodySize = 50 * 1024 * 1024

// stringSlice implements flag.Value for repeatable string flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// apiBody holds the constructed request body.
type apiBody struct {
	data        []byte // serialized body (JSON, raw) or nil for GET
	contentType string // Content-Type or "" (raw mode — user sets via -H)
	method      string // final HTTP method (after auto-selection)
}

type apiBodyParams struct {
	method    string    // HTTP method (or empty for auto-selection)
	fields    []string  // -f key=value
	inputFile string    // --input (path, "-" for stdin)
	stdin     io.Reader // deps.Stdin
}

func buildAPIBody(p apiBodyParams) (*apiBody, error) {
	hasFields := len(p.fields) > 0
	hasInput := p.inputFile != ""
	isMultipart := hasInput && strings.HasPrefix(p.inputFile, "@")

	// Validate mutual exclusions
	if hasInput && !isMultipart && hasFields {
		return nil, fmt.Errorf("--input and -f/-F are mutually exclusive (use --input @file for multipart)")
	}

	// Determine method
	method := p.method
	if method == "" {
		if hasFields || hasInput {
			method = "POST"
		} else {
			method = "GET"
		}
	}

	// No body
	if !hasFields && !hasInput {
		return &apiBody{method: method}, nil
	}

	// Raw mode: --input without @ prefix
	if hasInput && !isMultipart {
		var data []byte
		var err error
		if p.inputFile == "-" {
			data, err = io.ReadAll(io.LimitReader(p.stdin, maxRequestBodySize+1))
		} else {
			data, err = os.ReadFile(p.inputFile)
		}
		if err != nil {
			return nil, fmt.Errorf("reading input: %w", err)
		}
		if len(data) > maxRequestBodySize {
			return nil, fmt.Errorf("request body too large (max 50MB)")
		}
		return &apiBody{data: data, method: method}, nil
	}

	// JSON mode: -f fields without multipart
	if hasFields && !isMultipart {
		obj := make(map[string]any)
		for _, f := range p.fields {
			key, val, ok := strings.Cut(f, "=")
			if !ok {
				return nil, fmt.Errorf("invalid field format %q (expected key=value)", f)
			}
			obj[key] = val
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("marshaling JSON body: %w", err)
		}
		return &apiBody{data: data, contentType: "application/json", method: method}, nil
	}

	return &apiBody{method: method}, nil
}

func hasAuthHeader(headers []string) bool {
	for _, h := range headers {
		key, _, ok := strings.Cut(h, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "authorization") {
			return true
		}
	}
	return false
}

func buildHTTPRequest(ctx context.Context, body *apiBody, endpoint, baseURL, tok string, headers []string) (*http.Request, error) {
	url := baseURL + endpoint

	var bodyReader io.Reader
	if body.data != nil {
		bodyReader = bytes.NewReader(body.data)
	}

	req, err := http.NewRequestWithContext(ctx, body.method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set Authorization if not overridden by user
	if tok != "" && !hasAuthHeader(headers) {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	// Set Content-Type from body
	if body.contentType != "" {
		req.Header.Set("Content-Type", body.contentType)
	}

	// Apply custom headers (may override defaults)
	for _, h := range headers {
		key, val, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header format %q (expected key:value)", h)
		}
		req.Header.Set(strings.TrimSpace(key), strings.TrimSpace(val))
	}

	return req, nil
}

func runApi(args []string, deps Deps) error {
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)

	var flags config.Flags
	var method string
	var fields stringSlice
	var headers stringSlice
	var inputFile string
	var jqExpr string
	var include bool
	var reqTimeout time.Duration
	var silent bool

	globalFlags(fs, &flags)
	fs.StringVar(&method, "X", "", "HTTP method")
	fs.StringVar(&method, "method", "", "HTTP method")
	fs.Var(&fields, "f", "string field for JSON body (key=value, repeatable)")
	fs.Var(&fields, "field", "string field for JSON body (key=value, repeatable)")
	fs.Var(&headers, "H", "custom HTTP header (key:value, repeatable)")
	fs.Var(&headers, "header", "custom HTTP header (key:value, repeatable)")
	fs.StringVar(&inputFile, "input", "", "file with request body (- for stdin)")
	fs.StringVar(&jqExpr, "q", "", "jq expression for filtering JSON response")
	fs.StringVar(&jqExpr, "jq", "", "jq expression for filtering JSON response")
	fs.BoolVar(&include, "i", false, "show HTTP status and response headers")
	fs.BoolVar(&include, "include", false, "show HTTP status and response headers")
	fs.DurationVar(&reqTimeout, "timeout", 0, "HTTP request timeout (overrides config)")
	fs.BoolVar(&silent, "silent", false, "suppress response body output")

	fs.Usage = func() {
		fmt.Fprintf(deps.Stderr, `Usage: express-botx api [options] <endpoint>

Make authenticated HTTP requests to eXpress BotX API.

Examples:
  express-botx api /api/v3/botx/chats/list
  express-botx api -X POST /api/v3/botx/chats/create -f name=test
  express-botx api /api/v3/botx/chats/list -q '.result[].name'

Options:
`)
		fs.PrintDefaults()
	}

	if hasHelpFlag(args) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	applyVerboseFlag(flags)

	// Validate endpoint
	if fs.NArg() == 0 {
		return fmt.Errorf("endpoint required")
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args()[1:], " "))
	}
	endpoint := fs.Arg(0)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return fmt.Errorf("endpoint must be a path, not a full URL")
	}
	if !strings.HasPrefix(endpoint, "/") {
		return fmt.Errorf("endpoint must start with /")
	}

	// Validate jq expression early (before auth/request)
	if jqExpr != "" {
		if err := validateJQ(jqExpr); err != nil {
			return fmt.Errorf("invalid jq expression: %w", err)
		}
	}

	// Validate secret/token mutual exclusion
	if flags.Secret != "" && flags.Token != "" {
		return fmt.Errorf("--secret and --token are mutually exclusive")
	}

	manualAuth := hasAuthHeader(headers)

	// Load config
	cfg, err := config.Load(flags)
	if err != nil {
		if !manualAuth || flags.Host == "" {
			return err
		}
		// Manual auth with --host: create minimal config
		cfg = &config.Config{}
		cfg.Host = flags.Host
		cfg.Format = flags.Format
	}
	if err := cfg.ValidateFormat(); err != nil {
		return err
	}

	// Build request body (before auth — validate inputs first)
	body, err := buildAPIBody(apiBodyParams{
		method:    method,
		fields:    fields,
		inputFile: inputFile,
		stdin:     deps.Stdin,
	})
	if err != nil {
		return err
	}

	// Authenticate (skip if manual Authorization header)
	var authToken string
	var authCache token.Cache
	if !manualAuth {
		t, c, authErr := authenticate(cfg)
		if authErr != nil {
			return authErr
		}
		authToken = t
		authCache = c
	}

	// Determine timeout
	httpTimeout := cfg.HTTPTimeout()
	if reqTimeout > 0 {
		httpTimeout = reqTimeout
	}

	baseURL := botapi.ResolveBaseURL(cfg.Host)
	ctx := context.Background()

	// Build and execute request
	req, err := buildHTTPRequest(ctx, body, endpoint, baseURL, authToken, headers)
	if err != nil {
		return err
	}

	vlog.V1("api: %s %s", body.method, req.URL.String())

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 retry
	if resp.StatusCode == http.StatusUnauthorized && !manualAuth {
		resp.Body.Close()

		if cfg.BotToken != "" {
			return fmt.Errorf("bot token rejected (401), re-configure token")
		}

		vlog.V1("api: 401, refreshing token")
		newTok, err := refreshToken(cfg, authCache)
		if err != nil {
			return fmt.Errorf("refreshing token: %w", err)
		}
		authToken = newTok

		req, err = buildHTTPRequest(ctx, body, endpoint, baseURL, authToken, headers)
		if err != nil {
			return err
		}

		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("request failed after token refresh: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("unauthorized after token refresh (401)")
		}
	}

	return outputResponse(deps, resp, include, silent, jqExpr, cfg.Format)
}

func outputResponse(deps Deps, resp *http.Response, include, silent bool, jqExpr, format string) error {
	if include {
		fmt.Fprintf(deps.Stdout, "%s %s\n", resp.Proto, resp.Status)
		for key, vals := range resp.Header {
			for _, v := range vals {
				fmt.Fprintf(deps.Stdout, "%s: %s\n", key, v)
			}
		}
		fmt.Fprintln(deps.Stdout)
	}

	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300

	if !silent {
		if isSuccess {
			if jqExpr != "" {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("reading response: %w", err)
				}
				if err := applyJQ(deps.Stdout, deps.Stderr, data, jqExpr); err != nil {
					return err
				}
			} else if format == "json" && isJSONContentType(resp.Header.Get("Content-Type")) {
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("reading response: %w", err)
				}
				prettyPrintJSON(deps.Stdout, data)
			} else {
				io.Copy(deps.Stdout, resp.Body)
			}
		} else {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}
			if jqExpr != "" {
				if err := applyJQ(deps.Stdout, deps.Stderr, data, jqExpr); err != nil {
					deps.Stdout.Write(data)
				}
			} else if format == "json" && isJSONContentType(resp.Header.Get("Content-Type")) {
				prettyPrintJSON(deps.Stdout, data)
			} else {
				deps.Stdout.Write(data)
			}
		}
	}

	if !isSuccess {
		return &ExitError{Code: 1}
	}
	return nil
}

func isJSONContentType(ct string) bool {
	return strings.Contains(ct, "application/json")
}

func prettyPrintJSON(w io.Writer, data []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		w.Write(data)
		return
	}
	buf.WriteByte('\n')
	w.Write(buf.Bytes())
}

// validateJQ checks if a jq expression is syntactically valid.
// Stub for MVP — full validation comes with gojq in the next task.
func validateJQ(expr string) error {
	if expr == "" {
		return fmt.Errorf("empty expression")
	}
	return nil
}

// applyJQ applies a jq expression to JSON data.
// Stub for MVP — full implementation comes with gojq in the next task.
func applyJQ(stdout io.Writer, stderr io.Writer, data []byte, expr string) error {
	stdout.Write(data)
	return nil
}
