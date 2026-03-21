package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- buildAPIBody ---

func TestBuildAPIBody_NoBodyGET(t *testing.T) {
	body, err := buildAPIBody(apiBodyParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.method != "GET" {
		t.Errorf("expected GET, got %s", body.method)
	}
	if body.data != nil {
		t.Errorf("expected nil data, got %v", body.data)
	}
}

func TestBuildAPIBody_ExplicitMethod(t *testing.T) {
	body, err := buildAPIBody(apiBodyParams{method: "DELETE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.method != "DELETE" {
		t.Errorf("expected DELETE, got %s", body.method)
	}
}

func TestBuildAPIBody_FieldsAutoPost(t *testing.T) {
	body, err := buildAPIBody(apiBodyParams{
		fields: []string{"name=test", "chat_type=group"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.method != "POST" {
		t.Errorf("expected POST, got %s", body.method)
	}
	if body.contentType != "application/json" {
		t.Errorf("expected application/json, got %s", body.contentType)
	}

	var obj map[string]any
	if err := json.Unmarshal(body.data, &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["name"] != "test" || obj["chat_type"] != "group" {
		t.Errorf("unexpected JSON body: %s", string(body.data))
	}
}

func TestBuildAPIBody_InvalidFieldFormat(t *testing.T) {
	_, err := buildAPIBody(apiBodyParams{
		fields: []string{"invalid-no-equals"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid field format") {
		t.Errorf("expected invalid field format error, got: %v", err)
	}
}

func TestBuildAPIBody_RawMode(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "body.json")
	os.WriteFile(path, []byte(`{"raw":"data"}`), 0644)

	body, err := buildAPIBody(apiBodyParams{
		inputFile: path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.method != "POST" {
		t.Errorf("expected POST, got %s", body.method)
	}
	if body.contentType != "" {
		t.Errorf("expected empty content type in raw mode, got %s", body.contentType)
	}
	if string(body.data) != `{"raw":"data"}` {
		t.Errorf("unexpected data: %s", string(body.data))
	}
}

func TestBuildAPIBody_RawModeStdin(t *testing.T) {
	body, err := buildAPIBody(apiBodyParams{
		inputFile: "-",
		stdin:     strings.NewReader("stdin data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body.data) != "stdin data" {
		t.Errorf("expected 'stdin data', got %q", string(body.data))
	}
}

func TestBuildAPIBody_InputAndFieldsMutualExclusion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "body.json")
	os.WriteFile(path, []byte("data"), 0644)

	_, err := buildAPIBody(apiBodyParams{
		inputFile: path,
		fields:    []string{"key=val"},
	})
	if err == nil || !strings.Contains(err.Error(), "--input and -f/-F are mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

// --- hasAuthHeader ---

func TestHasAuthHeader(t *testing.T) {
	tests := []struct {
		headers []string
		want    bool
	}{
		{nil, false},
		{[]string{"Content-Type:application/json"}, false},
		{[]string{"Authorization:Bearer token"}, true},
		{[]string{"authorization:Bearer token"}, true},
		{[]string{"X-Custom:val", "Authorization:tok"}, true},
	}
	for _, tt := range tests {
		got := hasAuthHeader(tt.headers)
		if got != tt.want {
			t.Errorf("hasAuthHeader(%v) = %v, want %v", tt.headers, got, tt.want)
		}
	}
}

// --- buildHTTPRequest ---

func TestBuildHTTPRequest_BasicGET(t *testing.T) {
	body := &apiBody{method: "GET"}
	req, err := buildHTTPRequest(t.Context(), body, "/api/v3/test", "https://example.com", "mytoken", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("expected GET, got %s", req.Method)
	}
	if req.URL.String() != "https://example.com/api/v3/test" {
		t.Errorf("unexpected URL: %s", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer mytoken" {
		t.Errorf("unexpected Authorization: %s", req.Header.Get("Authorization"))
	}
}

func TestBuildHTTPRequest_CustomHeaders(t *testing.T) {
	body := &apiBody{method: "GET", contentType: "application/json"}
	req, err := buildHTTPRequest(t.Context(), body, "/test", "https://host", "tok", []string{
		"Content-Type:text/plain",
		"X-Custom:value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Custom header overrides Content-Type from body
	if req.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("expected text/plain, got %s", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("X-Custom") != "value" {
		t.Errorf("expected value, got %s", req.Header.Get("X-Custom"))
	}
}

func TestBuildHTTPRequest_ManualAuth(t *testing.T) {
	body := &apiBody{method: "GET"}
	headers := []string{"Authorization:token mytoken"}
	req, err := buildHTTPRequest(t.Context(), body, "/test", "https://host", "auto-tok", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// User-provided auth should be used, not auto token
	if req.Header.Get("Authorization") != "token mytoken" {
		t.Errorf("expected user Authorization, got %s", req.Header.Get("Authorization"))
	}
}

// --- runApi with httptest ---

func apiTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func apiTestConfig(t *testing.T, host string) string {
	t.Helper()
	return writeTestConfig(t, fmt.Sprintf(`
bots:
  default:
    host: %s
    id: test-bot-id
    token: test-token
`, host))
}

func TestRunApi_GETRequest(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.WriteHeader(200)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `{"status":"ok"}`) {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestRunApi_POSTWithJSONBody(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			t.Errorf("invalid JSON body: %v", err)
		}
		if obj["name"] != "test" {
			t.Errorf("expected name=test, got %v", obj["name"])
		}
		fmt.Fprint(w, `{"result":"created"}`)
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{
		"--config", cfgPath,
		"-X", "POST",
		"-f", "name=test",
		"-f", "chat_type=group",
		"/api/v3/chats/create",
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "created") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestRunApi_IncludeHeaders(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "response-val")
		fmt.Fprint(w, "body")
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "-i", "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "200") {
		t.Errorf("expected status 200 in output: %s", out)
	}
	if !strings.Contains(out, "X-Custom: response-val") {
		t.Errorf("expected X-Custom header in output: %s", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("expected body in output: %s", out)
	}
}

func TestRunApi_SilentMode(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "should not appear")
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "--silent", "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("expected empty output with --silent, got: %s", stdout.String())
	}
}

func TestRunApi_NonSuccessExitError(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":"not found"}`)
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "/api/v3/missing"}, deps)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got: %v", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected code 1, got %d", exitErr.Code)
	}
	// Body should still be in stdout
	if !strings.Contains(stdout.String(), "not found") {
		t.Errorf("expected error body in stdout: %s", stdout.String())
	}
}

func TestRunApi_BinaryResponse(t *testing.T) {
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(binaryData)
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "/api/v3/files/download"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout.Bytes()) != string(binaryData) {
		t.Errorf("binary data mismatch")
	}
}

func TestRunApi_CustomHeader(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "my-value" {
			t.Errorf("expected X-Custom=my-value, got %s", r.Header.Get("X-Custom"))
		}
		fmt.Fprint(w, "ok")
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, _, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "-H", "X-Custom:my-value", "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunApi_PrettyPrintJSON(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"a":1,"b":2}`)
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, stdout, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "--format", "json", "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "  \"a\": 1") {
		t.Errorf("expected pretty-printed JSON, got: %s", out)
	}
}

func TestRunApi_RawInput(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "<xml>data</xml>" {
			t.Errorf("expected raw XML body, got: %s", string(body))
		}
		if r.Header.Get("Content-Type") != "application/xml" {
			t.Errorf("expected application/xml, got %s", r.Header.Get("Content-Type"))
		}
		fmt.Fprint(w, "ok")
	})

	tmp := t.TempDir()
	inputPath := filepath.Join(tmp, "body.xml")
	os.WriteFile(inputPath, []byte("<xml>data</xml>"), 0644)

	cfgPath := apiTestConfig(t, ts.URL)
	deps, _, _ := testDeps()

	err := runApi([]string{
		"--config", cfgPath,
		"-X", "POST",
		"--input", inputPath,
		"-H", "Content-Type:application/xml",
		"/api/v3/smartapps/event",
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunApi_Timeout(t *testing.T) {
	ts := apiTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response — but test just checks the request goes through
		fmt.Fprint(w, "ok")
	})

	cfgPath := apiTestConfig(t, ts.URL)
	deps, _, _ := testDeps()

	err := runApi([]string{"--config", cfgPath, "--timeout", "5s", "/api/v3/test"}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Validation errors ---

func TestRunApi_MissingEndpoint(t *testing.T) {
	deps, _, _ := testDeps()
	err := runApi([]string{}, deps)
	if err == nil || !strings.Contains(err.Error(), "endpoint required") {
		t.Errorf("expected 'endpoint required', got: %v", err)
	}
}

func TestRunApi_EndpointMustStartWithSlash(t *testing.T) {
	deps, _, _ := testDeps()
	err := runApi([]string{"api/v3/test"}, deps)
	if err == nil || !strings.Contains(err.Error(), "endpoint must start with /") {
		t.Errorf("expected 'endpoint must start with /', got: %v", err)
	}
}

func TestRunApi_FullURLForbidden(t *testing.T) {
	deps, _, _ := testDeps()
	err := runApi([]string{"https://example.com/api/v3/test"}, deps)
	if err == nil || !strings.Contains(err.Error(), "endpoint must be a path, not a full URL") {
		t.Errorf("expected full URL error, got: %v", err)
	}
}

func TestRunApi_UnexpectedArgs(t *testing.T) {
	deps, _, _ := testDeps()
	err := runApi([]string{"/api/v3/test", "extra"}, deps)
	if err == nil || !strings.Contains(err.Error(), "unexpected arguments") {
		t.Errorf("expected unexpected arguments error, got: %v", err)
	}
}

func TestRunApi_SecretTokenMutualExclusion(t *testing.T) {
	deps, _, _ := testDeps()
	err := runApi([]string{"--secret", "s", "--token", "t", "/api/v3/test"}, deps)
	if err == nil || !strings.Contains(err.Error(), "--secret and --token are mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

// --- reorderArgs with repeatable flags ---

func TestReorderArgs_RepeatableFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var fields stringSlice
	var method string
	fs.Var(&fields, "f", "")
	fs.StringVar(&method, "X", "", "")

	result := reorderArgs(fs, []string{"-f", "a=1", "-f", "b=2", "/api/endpoint"})
	// Endpoint should be at the end
	if len(result) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(result), result)
	}
	if result[4] != "/api/endpoint" {
		t.Errorf("expected endpoint last, got: %v", result)
	}
}

// --- outputResponse ---

func TestOutputResponse_SilentWithInclude(t *testing.T) {
	deps, stdout, _ := testDeps()
	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Status:     "200 OK",
		Header:     http.Header{"X-Test": {"val"}},
		Body:       io.NopCloser(strings.NewReader("body")),
	}

	err := outputResponse(deps, resp, true, true, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "200 OK") {
		t.Errorf("expected status in output: %s", out)
	}
	// Body should be suppressed
	if strings.Contains(out, "body") {
		t.Errorf("expected body suppressed with --silent: %s", out)
	}
}

// --- ExitError ---

func TestExitError(t *testing.T) {
	err := &ExitError{Code: 1}
	if err.Error() != "exit status 1" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
	var e *ExitError
	if !errors.As(err, &e) {
		t.Error("expected errors.As to work with ExitError")
	}
}

// --- stringSlice ---

func TestStringSlice(t *testing.T) {
	var s stringSlice
	s.Set("a")
	s.Set("b")
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Errorf("expected [a, b], got %v", s)
	}
	if s.String() != "a, b" {
		t.Errorf("expected 'a, b', got %q", s.String())
	}
}
