package godville_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/godville"
)

func TestClient_GetHero_Public(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "Sir Testalot",
			"godname": "TestGod",
			"level": 42,
			"max_health": 100,
			"motto": "По нюху!",
			"pet": {"pet_name": "Шушпанчик", "pet_class": "Шушпанчик-вертолёт", "pet_level": 7}
		}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	hero, err := client.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("GetHero failed: %v", err)
	}

	if capturedPath != "/gods/api/TestGod.json" {
		t.Errorf("expected path /gods/api/TestGod.json, got %s", capturedPath)
	}

	if hero.Name != "Sir Testalot" {
		t.Errorf("expected name Sir Testalot, got %s", hero.Name)
	}

	if hero.Level != 42 {
		t.Errorf("expected level 42, got %d", hero.Level)
	}

	if hero.Pet == nil || hero.Pet.Name != "Шушпанчик" {
		t.Errorf("expected pet Шушпанчик, got %+v", hero.Pet)
	}

	if hero.Raw == nil {
		t.Fatal("expected Raw payload to be populated")
	}

	if hero.Raw["motto"] != "По нюху!" {
		t.Errorf("expected motto preserved in Raw, got %v", hero.Raw["motto"])
	}
}

func TestClient_GetHero_Private(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "Sir Testalot",
			"godname": "TestGod",
			"health": 80,
			"godpower": 50,
			"quest": "Найти что-нибудь",
			"quest_progress": 33,
			"diary_last": "12:00 Сегодня я нашёл кирпич.",
			"distance": 1234,
			"town_name": "Сан-Франциско"
		}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	hero, err := client.GetHero(context.Background(), "TestGod", "secret-key")
	if err != nil {
		t.Fatalf("GetHero failed: %v", err)
	}

	if capturedPath != "/gods/api/TestGod/secret-key.json" {
		t.Errorf("expected private path, got %s", capturedPath)
	}

	if hero.Health != 80 {
		t.Errorf("expected health 80, got %d", hero.Health)
	}

	if hero.Godpower != 50 {
		t.Errorf("expected godpower 50, got %d", hero.Godpower)
	}

	if hero.Quest != "Найти что-нибудь" {
		t.Errorf("expected quest preserved, got %q", hero.Quest)
	}

	if hero.QuestProgress != 33 {
		t.Errorf("expected quest_progress 33, got %d", hero.QuestProgress)
	}

	if !strings.Contains(hero.DiaryLast, "нашёл кирпич") {
		t.Errorf("expected diary preserved, got %q", hero.DiaryLast)
	}

	if hero.TownName != "Сан-Франциско" {
		t.Errorf("expected town preserved, got %q", hero.TownName)
	}
}

func TestClient_GetHero_EmptyGodname(t *testing.T) {
	client := godville.NewClient("https://godville.net")

	_, err := client.GetHero(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for empty godname")
	}

	if !errors.Is(err, godville.ErrEmptyGodname) {
		t.Errorf("expected ErrEmptyGodname, got: %v", err)
	}
}

func TestClient_GetHero_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	_, err := client.GetHero(context.Background(), "TestGod", "")
	if err == nil {
		t.Fatal("expected error on HTTP 403")
	}

	if !errors.Is(err, godville.ErrAPI) {
		t.Errorf("expected ErrAPI, got: %v", err)
	}

	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected error to mention rate limit, got: %v", err)
	}
}

func TestClient_GetHero_APIErrorInBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error": "no_such_god"}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	_, err := client.GetHero(context.Background(), "GhostGod", "")
	if err == nil {
		t.Fatal("expected error when API returns JSON error in 200 OK body")
	}

	if !errors.Is(err, godville.ErrAPI) {
		t.Errorf("expected ErrAPI, got: %v", err)
	}
}

func TestClient_GetHero_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	_, err := client.GetHero(context.Background(), "TestGod", "")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestClient_GetHero_GodnameEscaping(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(capturedHandler(&capturedPath, `{"godname": "Имя С Пробелом"}`))
	defer server.Close()

	client := godville.NewClient(server.URL)

	_, err := client.GetHero(context.Background(), "Имя С Пробелом", "")
	if err != nil {
		t.Fatalf("GetHero failed: %v", err)
	}

	// Spaces must be percent-encoded as %20 in the wire path (not "+").
	if !strings.Contains(capturedPath, "%20") || strings.Contains(capturedPath, "+") {
		t.Errorf("expected path to use %%20 escaping, got %s", capturedPath)
	}
}

// capturedHandler captures the raw escaped path for assertions.
func capturedHandler(dst *string, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*dst = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// Regression: real Godville API returns numeric boss_power, boolean
// arena_fight, string souls/relics percent, empty-string pet_level when the
// pet just lost a level. An earlier types.go declared these as the wrong Go
// types and broke decode for any hero with non-trivial state.
func TestClient_GetHero_RealisticPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "Sir Testalot",
			"boss_name": "Жуткий кашалот",
			"boss_power": 50,
			"arena_fight": true,
			"souls_percent": "25",
			"relics_percent": "3",
			"activatables": ["амулет", "посох"],
			"pet": {"pet_name": "Шушпанчик", "pet_class": "вертолёт", "pet_level": ""}
		}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	hero, err := client.GetHero(context.Background(), "TestGod", "secret")
	if err != nil {
		t.Fatalf("realistic payload must decode cleanly, got: %v", err)
	}

	if hero.BossPower != 50 {
		t.Errorf("expected BossPower 50, got %d", hero.BossPower)
	}

	if !hero.ArenaFight {
		t.Errorf("expected ArenaFight true")
	}

	if hero.SoulsPercent != "25" || hero.RelicsPercent != "3" {
		t.Errorf("expected percent strings preserved, got souls=%q relics=%q",
			hero.SoulsPercent, hero.RelicsPercent)
	}

	if len(hero.Activatables) != 2 {
		t.Errorf("expected 2 activatables, got %d", len(hero.Activatables))
	}

	if hero.Pet == nil || hero.Pet.Level != 0 || hero.Pet.Name != "Шушпанчик" {
		t.Errorf("expected pet with empty-string level decoded as 0, got %+v", hero.Pet)
	}
}

// Regression: HTTP transport errors must NOT echo the URL because the
// Godville API requires the userkey to live in the URL path. A failed
// fetch would otherwise leak the userkey through any log that captures
// the wrapped error string.
func TestClient_GetHero_TransportErrorDoesNotLeakUserkey(t *testing.T) {
	// Close the server immediately so the next request fails with a
	// transport error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Close()

	client := godville.NewClient(server.URL)

	const canary = "userkey-leak-canary-7f3a"

	_, err := client.GetHero(context.Background(), "TestGod", canary)
	if err == nil {
		t.Fatal("expected transport error after server.Close")
	}

	if strings.Contains(err.Error(), canary) {
		t.Errorf("transport error leaked userkey: %v", err)
	}
}

// Regression: TLS handshake errors are a different *url.Error subclass than
// connection-refused. Verify the URL scrubber strips the userkey from these
// too — otherwise misconfigured HTTPS endpoints would leak credentials.
// Regression: error bodies from the Russian Godville API contain Cyrillic
// (each codepoint is 2 bytes in UTF-8). A naive byte-slice truncate
// produced invalid UTF-8 when the cutoff landed inside a multi-byte
// sequence; that invalid string then flowed into MCP responses and logs.
func TestClient_GetHero_TruncatedErrorBodyIsValidUTF8(t *testing.T) {
	// Build a body whose 201st byte lands inside a multi-byte rune. 50
	// Cyrillic letters at 2 bytes each = 100 bytes, then 100 ASCII bytes,
	// then more Cyrillic — the 200-byte cut is well inside the trailing
	// Cyrillic run.
	prefix := strings.Repeat("я", 50)         // 100 bytes
	middle := strings.Repeat("x", 100)        // 100 bytes (total 200 so far)
	tail := strings.Repeat("ё", 50)           // another 100 bytes
	body := "<html>" + prefix + middle + tail // > 200 bytes total

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	_, err := client.GetHero(context.Background(), "TestGod", "")
	if err == nil {
		t.Fatal("expected API error from 502")
	}

	if !utf8.ValidString(err.Error()) {
		t.Errorf("expected error message to be valid UTF-8, got: %q", err.Error())
	}
}

func TestClient_GetHero_TLSErrorDoesNotLeakUserkey(t *testing.T) {
	// httptest.NewTLSServer issues a self-signed cert; our default client
	// rejects it, producing a TLS-class *url.Error.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)

	const canary = "userkey-tls-canary-9c1e"

	_, err := client.GetHero(context.Background(), "TestGod", canary)
	if err == nil {
		t.Fatal("expected TLS handshake error against self-signed httptest TLS server")
	}

	if strings.Contains(err.Error(), canary) {
		t.Errorf("TLS-class transport error leaked userkey: %v", err)
	}
}

// Regression: decodeIntOrEmptyString must accept all three documented
// forms — bare int, quoted int (Godville returns some numeric counters
// as strings), and empty string ("pet just lost a level"). Earlier the
// quoted-int and bare-int paths had no direct coverage; only the
// empty-string path was hit via the realistic payload test.
func TestPet_LevelDecodeForms(t *testing.T) {
	type pet struct {
		Pet godville.Pet `json:"pet"`
	}

	cases := []struct {
		name    string
		body    string
		wantLvl int
		wantErr bool
	}{
		{"bare int", `{"pet":{"pet_level":7}}`, 7, false},
		{"quoted int", `{"pet":{"pet_level":"42"}}`, 42, false},
		{"empty string", `{"pet":{"pet_level":""}}`, 0, false},
		{"missing field", `{"pet":{}}`, 0, false},
		{"non-numeric quoted", `{"pet":{"pet_level":"abc"}}`, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got pet
			err := json.Unmarshal([]byte(tc.body), &got)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tc.body)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Pet.Level != tc.wantLvl {
				t.Errorf("expected level %d, got %d", tc.wantLvl, got.Pet.Level)
			}
		})
	}
}

func TestClient_GetHero_TrimsBaseTrailingSlash(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(capturedHandler(&capturedPath, `{}`))
	defer server.Close()

	client := godville.NewClient(server.URL + "/")

	_, err := client.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("GetHero failed: %v", err)
	}

	// No double slash in path even when base had a trailing slash.
	if strings.Contains(capturedPath, "//") {
		t.Errorf("expected no double slash in path, got %s", capturedPath)
	}
}
