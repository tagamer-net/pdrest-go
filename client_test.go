package pdrest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var testCtx = context.Background()

func newTestClient(t *testing.T, serverURL string) *Client {
	client, err := NewClient(serverURL, "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}

func TestNewClient_NormalizesBaseURL(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("expected client not nil")
	}
}

func TestNewClient_RequiresBearerToken(t *testing.T) {
	_, err := NewClient("127.0.0.1", "")
	if err == nil {
		t.Fatal("expected error when bearer token is empty")
	}
}

func TestNewClient_TimeoutAppliedRegardlessOfOptionOrder(t *testing.T) {
	for _, opts := range [][]Option{
		{WithTimeout(5 * time.Second), WithHTTPClient(&http.Client{})},
		{WithHTTPClient(&http.Client{}), WithTimeout(5 * time.Second)},
	} {
		client, err := NewClient("127.0.0.1", "token123", opts...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.httpClient.Timeout != 5*time.Second {
			t.Fatalf("expected timeout %v, got %v", 5*time.Second, client.httpClient.Timeout)
		}
	}
}

func TestNewClient_KeepsInjectedClientTimeout(t *testing.T) {
	injected := &http.Client{Timeout: 7 * time.Second}
	client, err := NewClient("127.0.0.1", "token123", WithHTTPClient(injected))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.httpClient != injected || client.httpClient.Timeout != 7*time.Second {
		t.Fatalf("unexpected http client: %+v", client.httpClient)
	}
}

func TestNewClient_ExplicitDefaultTimeoutAppliedToInjectedClient(t *testing.T) {
	injected := &http.Client{}
	client, err := NewClient("127.0.0.1", "token123", WithHTTPClient(injected), WithTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.httpClient != injected || client.httpClient.Timeout != 30*time.Second {
		t.Fatalf("unexpected http client: %+v", client.httpClient)
	}
}

func TestNewClient_RejectsNonPositiveTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -1 * time.Second} {
		if _, err := NewClient("127.0.0.1", "token123", WithTimeout(timeout)); err == nil {
			t.Fatalf("expected error for timeout %v", timeout)
		}
	}
}

func TestNewClient_KeepsInjectedZeroTimeoutWithoutWithTimeout(t *testing.T) {
	injected := &http.Client{}
	client, err := NewClient("127.0.0.1", "token123", WithHTTPClient(injected))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.httpClient != injected || client.httpClient.Timeout != 0 {
		t.Fatalf("unexpected http client: %+v", client.httpClient)
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		StatusCode:   401,
		Method:       "GET",
		Path:         "/v1/pdapi/version",
		ResponseBody: "Unauthorized.",
	}

	got := err.Error()
	if got == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestClient_SendPlayerMessage_Validation(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{Message: "hi", UserID: "a"}); err == nil {
		t.Fatal("expected error when SendType is missing")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", UserID: "a"}); err == nil {
		t.Fatal("expected error when Message is missing")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi"}); err == nil {
		t.Fatal("expected error when neither UserID nor UserIDs is set")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi", UserID: "a", UserIDs: []string{"b"}}); err == nil {
		t.Fatal("expected error when both UserID and UserIDs are set")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi", UserID: "   "}); err == nil {
		t.Fatal("expected error when UserID is whitespace-only")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi", UserIDs: []string{"a", " "}}); err == nil {
		t.Fatal("expected error when UserIDs contains an empty entry")
	}
}

func TestClient_BroadcastAndAlert_RequireMessage(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	for _, message := range []string{"", "   "} {
		if _, err := client.Broadcast(testCtx, message, "sender"); err == nil {
			t.Fatalf("expected error for Broadcast with message %q", message)
		}
		if _, err := client.Alert(testCtx, message); err == nil {
			t.Fatalf("expected error for Alert with message %q", message)
		}
	}
}

func TestResponseSnippet_UTF8SafeTruncation(t *testing.T) {
	long := strings.Repeat("€", 342)
	snippet := responseSnippet([]byte(long))
	if !utf8.ValidString(snippet) {
		t.Fatal("snippet must be valid UTF-8")
	}
	if len(snippet) != 1023+3 {
		t.Fatalf("unexpected snippet length: %d", len(snippet))
	}
}

func TestClient_ResponseBodyTooLarge(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strings.Repeat("a", maxResponseBody+1))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	_, err := client.GetVersion(testCtx)
	if err == nil {
		t.Fatal("expected error for oversized response body")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGuildStorage_UnknownNonObjectKeyIgnored(t *testing.T) {
	var storage GuildStorage
	data := []byte(`{"container_id": "cont-1", "current": 1, "max": 100, "available": true, "0": {"item_id": "Money", "count": 5}}`)
	if err := json.Unmarshal(data, &storage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.ContainerID != "cont-1" || storage.Current != 1 || storage.Max != 100 {
		t.Fatalf("unexpected storage: %+v", storage)
	}
	if len(storage.Slots) != 1 {
		t.Fatalf("unexpected slots: %+v", storage.Slots)
	}
	if slot := storage.Slots["0"]; slot.ItemID != "Money" || slot.Count != 5 {
		t.Fatalf("unexpected slot: %+v", storage.Slots)
	}
}

func TestGuildStorage_UnknownObjectKeyIgnored(t *testing.T) {
	var storage GuildStorage
	data := []byte(`{"container_id": "cont-1", "notes": {"text": "x"}, "0": {"item_id": "Money", "count": 5}}`)
	if err := json.Unmarshal(data, &storage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(storage.Slots) != 1 {
		t.Fatalf("unexpected slots: %+v", storage.Slots)
	}
	if slot := storage.Slots["0"]; slot.ItemID != "Money" || slot.Count != 5 {
		t.Fatalf("unexpected slot: %+v", storage.Slots)
	}
}

func TestClient_EmptyPathParamsRejected(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	exp := 100
	tests := []struct {
		name string
		call func() error
	}{
		{"GetGuild", func() error { _, err := client.GetGuild(testCtx, ""); return err }},
		{"GetPlayer", func() error { _, err := client.GetPlayer(testCtx, ""); return err }},
		{"GetPals", func() error { _, err := client.GetPals(testCtx, ""); return err }},
		{"GetItems", func() error { _, err := client.GetItems(testCtx, ""); return err }},
		{"GetTechs", func() error { _, err := client.GetTechs(testCtx, ""); return err }},
		{"GetProgression", func() error { _, err := client.GetProgression(testCtx, ""); return err }},
		{"GiveItems", func() error { _, err := client.GiveItems(testCtx, "", "Money"); return err }},
		{"GivePals", func() error { _, err := client.GivePals(testCtx, "", "Pengullet"); return err }},
		{"GivePalTemplates", func() error { _, err := client.GivePalTemplates(testCtx, "", "Lamball.json"); return err }},
		{"GivePalEggs", func() error {
			_, err := client.GivePalEggs(testCtx, "", GivePalEgg{EggID: "PalEgg_Fire_01", PalID: "Foxparks"})
			return err
		}},
		{"GiveProgression", func() error { _, err := client.GiveProgression(testCtx, "", nil, &exp, nil, nil, nil); return err }},
		{"LearnTech", func() error { _, err := client.LearnTech(testCtx, "", "Technology_1"); return err }},
		{"ForgetTech", func() error { _, err := client.ForgetTech(testCtx, "", "Technology_1"); return err }},
		{"DeleteBase", func() error { _, err := client.DeleteBase(testCtx, ""); return err }},
		{"Ban", func() error { _, err := client.Ban(testCtx, "", "reason", false); return err }},
		{"Unban", func() error { _, err := client.Unban(testCtx, "", "reason"); return err }},
		{"BanIP", func() error { _, err := client.BanIP(testCtx, "", nil); return err }},
		{"UnbanIP", func() error { _, err := client.UnbanIP(testCtx, "", nil); return err }},
		{"Kick", func() error { _, err := client.Kick(testCtx, "", "reason"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected error for empty identifier")
			}
		})
	}
}

func TestClient_WhitespacePathParts(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	exp := 100
	tests := []struct {
		name string
		call func() error
	}{
		{"GetGuild", func() error { _, err := client.GetGuild(testCtx, "   "); return err }},
		{"GetPlayer", func() error { _, err := client.GetPlayer(testCtx, "   "); return err }},
		{"GetPals", func() error { _, err := client.GetPals(testCtx, "   "); return err }},
		{"GetItems", func() error { _, err := client.GetItems(testCtx, "   "); return err }},
		{"GetTechs", func() error { _, err := client.GetTechs(testCtx, "   "); return err }},
		{"GetProgression", func() error { _, err := client.GetProgression(testCtx, "   "); return err }},
		{"GiveItems", func() error { _, err := client.GiveItems(testCtx, "   ", "Money"); return err }},
		{"GivePals", func() error { _, err := client.GivePals(testCtx, "   ", "Pengullet"); return err }},
		{"GivePalTemplates", func() error { _, err := client.GivePalTemplates(testCtx, "   ", "Lamball.json"); return err }},
		{"GivePalEggs", func() error {
			_, err := client.GivePalEggs(testCtx, "   ", GivePalEgg{EggID: "PalEgg_Fire_01", PalID: "Foxparks"})
			return err
		}},
		{"GiveProgression", func() error { _, err := client.GiveProgression(testCtx, "   ", nil, &exp, nil, nil, nil); return err }},
		{"LearnTech", func() error { _, err := client.LearnTech(testCtx, "   ", "Technology_1"); return err }},
		{"ForgetTech", func() error { _, err := client.ForgetTech(testCtx, "   ", "Technology_1"); return err }},
		{"Ban", func() error { _, err := client.Ban(testCtx, "   ", "reason", false); return err }},
		{"Unban", func() error { _, err := client.Unban(testCtx, "   ", "reason"); return err }},
		{"BanIP", func() error { _, err := client.BanIP(testCtx, "   ", nil); return err }},
		{"UnbanIP", func() error { _, err := client.UnbanIP(testCtx, "   ", nil); return err }},
		{"Kick", func() error { _, err := client.Kick(testCtx, "   ", "reason"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected error for whitespace-only identifier")
			}
		})
	}
}

func TestClient_BanAndBanlist(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/ban/test_player", func(w http.ResponseWriter, r *http.Request) {
		assertBanRequest(t, r)
		writeJSON(t, w, BanResponse{Success: true, UserId: "test_player", IP: true, BannedIP: "1.2.3.4", Kicked: 1})
	})
	handler.HandleFunc("/v1/pdapi/banlist", func(w http.ResponseWriter, r *http.Request) {
		assertBanlistRequest(t, r)
		writeJSON(t, w, BanlistResponse{Banlist: BanlistData{
			Version:       1,
			BannedMessage: "Banned",
			UserEntries: []BanlistUserEntry{{
				UserId: "test_player",
				Active: true,
				BannedBy: BanlistIssuer{
					Type:      "rest",
					NameValue: "token123",
					Reason:    "test reason",
					Timestamp: BanlistTimestamp{UTC: 1700000000, Year: 2023},
				},
			}},
			IPEntries: []BanlistIPEntry{{IP: "1.2.3.4", Active: true}},
		}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	banResult, err := client.Ban(testCtx, "test_player", "test reason", true)
	if err != nil {
		t.Fatalf("Ban failed: %v", err)
	}
	if !banResult.Success || banResult.UserId != "test_player" || banResult.Kicked != 1 {
		t.Fatalf("unexpected ban response: %+v", banResult)
	}

	banlistResult, err := client.GetBanlist(testCtx, map[string]string{"active": "true"})
	if err != nil {
		t.Fatalf("GetBanlist failed: %v", err)
	}
	if banlistResult.Banlist.Version != 1 {
		t.Fatalf("unexpected banlist response: %+v", banlistResult)
	}
	if len(banlistResult.Banlist.UserEntries) != 1 || banlistResult.Banlist.UserEntries[0].UserId != "test_player" {
		t.Fatalf("unexpected banlist entries: %+v", banlistResult)
	}
	if len(banlistResult.Banlist.IPEntries) != 1 || banlistResult.Banlist.IPEntries[0].IP != "1.2.3.4" {
		t.Fatalf("unexpected banlist IP entries: %+v", banlistResult)
	}
}

func assertBanRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method %s", r.Method)
	}
	if r.Header.Get("Authorization") != "Bearer token123" {
		t.Fatal("missing bearer token header")
	}
	var payload BanRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode request: %v", err)
	}
	if payload.Reason != "test reason" || !payload.IP {
		t.Fatal("unexpected ban payload")
	}
}

func assertBanlistRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodGet {
		t.Fatalf("unexpected method %s", r.Method)
	}
	if r.URL.Query().Get("active") != "true" {
		t.Fatal("missing or incorrect active query")
	}
}

func TestClient_GetBanlist_RejectsUnknownFilters(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.GetBanlist(testCtx, map[string]string{"userID": "steam_1"}); err == nil {
		t.Fatal("expected error for unknown filter key")
	}
	if _, err := client.GetBanlist(testCtx, map[string]string{"active": "true", "ip_address": "1.2.3.4"}); err == nil {
		t.Fatal("expected error for unknown filter key mixed with documented ones")
	}
}

func TestClient_GetBanlist_RejectsInvalidActiveValue(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	for _, value := range []string{"maybe", "0", "False", ""} {
		if _, err := client.GetBanlist(testCtx, map[string]string{"active": value}); err == nil {
			t.Fatalf("expected error for invalid active value %q", value)
		}
	}
}

func TestClient_SendPlayerMessage(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/SendPlayerMessage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload SendPlayerMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.SendType != "PlayerChat" || payload.Message != "hello" {
			t.Fatalf("unexpected send message payload: %+v", payload)
		}
		writeJSON(t, w, SendPlayerMessageResponse{Success: true, SentCount: 1})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hello", UserID: "user123"})
	if err != nil {
		t.Fatalf("SendPlayerMessage failed: %v", err)
	}
	if !result.Success || result.SentCount != 1 {
		t.Fatalf("unexpected send player message response: %+v", result)
	}

	result, err = client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hello", UserIDs: []string{"user123"}})
	if err != nil {
		t.Fatalf("SendPlayerMessage with UserIDs failed: %v", err)
	}
	if !result.Success || result.SentCount != 1 {
		t.Fatalf("unexpected send player message response: %+v", result)
	}
}

func TestClient_GetVersion_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		writeJSON(t, w, map[string]any{
			"Version": map[string]any{
				"Major": 1, "Minor": 2, "Patch": 3, "Build": 4,
				"Version": "1.2.3", "VersionLong": "1.2.3.4", "Beta": false,
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetVersion(testCtx)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if result.Major != 1 || result.Minor != 2 || result.Patch != 3 || result.Build != 4 {
		t.Fatalf("unexpected version: %+v", result)
	}
	if result.Version != "1.2.3" || result.VersionLong != "1.2.3.4" || result.Beta {
		t.Fatalf("unexpected version strings: %+v", result)
	}
}

func TestClient_GetPlayer_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/player/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Player": map[string]any{
				"Name": "Alice", "IP": "10.0.0.1",
				"PlayerUID": "uid-1", "UserId": "steam_1",
				"GuildName": "Guild", "GuildUUID": "guild-1",
				"Status":        "online",
				"WorldLocation": map[string]any{"x": 1, "y": 2, "z": 3},
				"MapLocation":   map[string]any{"x": 4, "y": 5, "z": 6},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetPlayer(testCtx, "player123")
	if err != nil {
		t.Fatalf("GetPlayer failed: %v", err)
	}
	if result.Name != "Alice" || result.UserId != "steam_1" || result.Status != "online" {
		t.Fatalf("unexpected player: %+v", result)
	}
	if result.WorldLocation.X != 1 || result.WorldLocation.Z != 3 || result.MapLocation.Y != 5 {
		t.Fatalf("unexpected locations: %+v", result)
	}
}

func TestClient_GetPlayers_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/players", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta":    map[string]any{"PlayerCount": 2, "OnlineCount": 1},
			"Players": []any{map[string]any{"Name": "Alice", "Status": "online"}, map[string]any{"Name": "Bob"}},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetPlayers(testCtx)
	if err != nil {
		t.Fatalf("GetPlayers failed: %v", err)
	}
	if result.Meta.PlayerCount != 2 || result.Meta.OnlineCount != 1 {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
	if len(result.Players) != 2 || result.Players[0].Status != "online" {
		t.Fatalf("unexpected players: %+v", result.Players)
	}
}

func TestClient_GetGuilds_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/guilds", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta": map[string]any{"GuildCount": 1},
			"Guilds": map[string]any{
				"guild-1": map[string]any{
					"name": "Guild", "Level": 15,
					"admin":      map[string]any{"id": "uid-1", "name": "Alice"},
					"camp_count": 2, "member_count": 3,
					"camps": []any{map[string]any{
						"id":        "camp-1",
						"world_pos": map[string]any{"x": 1, "y": 2, "z": 3},
						"map_pos":   map[string]any{"x": 4, "y": 5, "z": 6},
					}},
					"members": []any{"uid-1", "uid-2"},
				},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetGuilds(testCtx)
	if err != nil {
		t.Fatalf("GetGuilds failed: %v", err)
	}
	if result.Meta.GuildCount != 1 {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
	guild, ok := result.Guilds["guild-1"]
	if !ok || guild.Name != "Guild" || guild.Admin.ID != "uid-1" || guild.Level != 15 {
		t.Fatalf("unexpected guild summary: %+v", result.Guilds)
	}
	if len(guild.Camps) != 1 || guild.Camps[0].WorldPos != (Coordinates{X: 1, Y: 2, Z: 3}) || guild.Camps[0].MapPos.Y != 5 {
		t.Fatalf("unexpected camp summary: %+v", guild.Camps)
	}
}

func TestClient_GetGuild_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/guild/guild-1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Guild": map[string]any{
				"name": "Guild", "Level": 15,
				"admin":        map[string]any{"id": "uid-1", "name": "Alice"},
				"member_count": 3,
				"members": []any{map[string]any{
					"player_uid": "uid-1", "player_name": "Alice", "status": "online",
				}},
				"camp_count": 1,
				"camps": []any{map[string]any{
					"id":        "camp-1",
					"level":     3,
					"state":     "working",
					"world_pos": map[string]any{"x": 1, "y": 2, "z": 3},
					"map_pos":   map[string]any{"x": 4, "y": 5, "z": 6},
					"pals": map[string]any{
						"worker-1": map[string]any{
							"nickname": "Fluffy", "pal_id": "Foxparks", "npc_id": "npc-1",
							"skin_id": "", "gender": "Male", "level": 12, "shiny": true,
							"phisical_health": "Healthy", "worker_sick": "None", "san": 80.5,
							"imported": false, "friendship": 20,
							"active_skills": []any{"Skill_1"}, "learnt_skills": []any{"Skill_2"},
							"passives": []any{"passive_1"},
						},
					},
					"buildings": "payload",
				}},
				"items": map[string]any{
					"container_id": "cont-1", "current": 2, "max": 100,
					"0": map[string]any{"item_id": "Money", "count": 500},
				},
				"expeditions": map[string]any{
					"finished": 2, "missions": map[string]any{"m1": true},
				},
				"laboratory": map[string]any{
					"current_research": "research_1",
					"researches": map[string]any{
						"research_1": map[string]any{
							"work_amount": 50, "required_work_amount": 100, "percentage": 0.5,
						},
					},
				},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetGuild(testCtx, "guild-1")
	if err != nil {
		t.Fatalf("GetGuild failed: %v", err)
	}
	if result.Name != "Guild" || result.Admin.ID != "uid-1" {
		t.Fatalf("unexpected guild: %+v", result)
	}
	if len(result.Members) != 1 || result.Members[0].PlayerUID != "uid-1" {
		t.Fatalf("unexpected members: %+v", result.Members)
	}
	if result.Items.ContainerID != "cont-1" || result.Items.Current != 2 {
		t.Fatalf("unexpected guild storage: %+v", result.Items)
	}
	slot, ok := result.Items.Slots["0"]
	if !ok || slot.ItemID != "Money" || slot.Count != 500 {
		t.Fatalf("unexpected guild storage slots: %+v", result.Items.Slots)
	}
	if len(result.Camps) != 1 {
		t.Fatalf("unexpected camps: %+v", result.Camps)
	}
	verifyGuildCampDetail(t, result.Camps[0])
	verifyGuildExtras(t, result)
}

func verifyGuildCampDetail(t *testing.T, camp GuildCampDetail) {
	t.Helper()
	if camp.WorldPos.X != 1 || camp.WorldPos.Z != 3 || camp.MapPos.Y != 5 || camp.Buildings != "payload" {
		t.Fatalf("unexpected camp: %+v", camp)
	}
	worker, ok := camp.Pals["worker-1"]
	if !ok || worker.PalID != "Foxparks" || worker.Level != 12 || !worker.Shiny {
		t.Fatalf("unexpected camp pals: %+v", camp.Pals)
	}
	if worker.PhysicalHealth != "Healthy" || worker.San != 80.5 || worker.Friendship != 20 {
		t.Fatalf("unexpected camp pal: %+v", worker)
	}
	if len(worker.Passives) != 1 || worker.Passives[0] != "passive_1" {
		t.Fatalf("unexpected camp pal: %+v", worker)
	}
}

func verifyGuildExtras(t *testing.T, result *GuildDetail) {
	t.Helper()
	if result.Expeditions.Finished != 2 || !result.Expeditions.Missions["m1"] {
		t.Fatalf("unexpected expeditions: %+v", result.Expeditions)
	}
	if result.Laboratory.CurrentResearch != "research_1" ||
		result.Laboratory.Researches["research_1"].Percentage != 0.5 {
		t.Fatalf("unexpected laboratory: %+v", result.Laboratory)
	}
}

func TestClient_GetPals_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/pals/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta": map[string]any{
				"PlayerUID": "uid-1", "Player": "player123",
				"TeamCount": 1, "PalboxCount": 0, "BaseCampCount": 1,
			},
			"Pals": map[string]any{
				"Team": map[string]any{
					"instance-1": map[string]any{
						"PalID": "Pengullet", "Level": 10, "Shiny": true,
						"Passives": []any{"passive_1"}, "team_slot_index": 0,
						"PalSouls":               map[string]any{"Health": 1, "Attack": 2, "Defense": 0, "CraftSpeed": 0},
						"IVs":                    map[string]any{"Health": 0.5, "AttackMelee": 0.4, "AttackShot": 0.3, "Defense": 0.2},
						"ExtraWorkSuitabilities": map[string]any{"Transporting": 3},
					},
				},
				"Palbox": map[string]any{},
				"BaseCamps": []any{map[string]any{
					"id":        "camp-1",
					"level":     3,
					"state":     "working",
					"world_pos": map[string]any{"x": 1, "y": 2, "z": 3},
					"map_pos":   map[string]any{"x": 4, "y": 5, "z": 6},
					"pals": map[string]any{
						"worker-1": map[string]any{"PalID": "Foxparks", "Level": 12, "base_camp_slot_index": 0},
					},
				}},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetPals(testCtx, "player123")
	if err != nil {
		t.Fatalf("GetPals failed: %v", err)
	}
	if result.Meta.TeamCount != 1 || result.Meta.BaseCampCount != 1 {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
	pal, ok := result.Pals.Team["instance-1"]
	if !ok || pal.PalID != "Pengullet" || pal.Level != 10 || !pal.Shiny {
		t.Fatalf("unexpected team pal: %+v", result.Pals.Team)
	}
	if len(pal.Passives) != 1 || pal.Passives[0] != "passive_1" {
		t.Fatalf("unexpected passives: %+v", pal.Passives)
	}
	if pal.PalSouls.Health != 1 || pal.PalSouls.Attack != 2 || pal.IVs.Health != 0.5 ||
		pal.IVs.AttackShot != 0.3 || pal.ExtraWorkSuitabilities["Transporting"] != 3 {
		t.Fatalf("unexpected pal internals: %+v", pal)
	}
	if len(result.Pals.BaseCamps) != 1 {
		t.Fatalf("unexpected base camps: %+v", result.Pals.BaseCamps)
	}
	camp := result.Pals.BaseCamps[0]
	if camp.WorldPos.Z != 3 || camp.MapPos.X != 4 {
		t.Fatalf("unexpected camp positions: %+v", camp)
	}
	worker, ok := camp.Pals["worker-1"]
	if !ok || worker.PalID != "Foxparks" || worker.BaseCampSlotIndex != 0 {
		t.Fatalf("unexpected camp pals: %+v", camp.Pals)
	}
}

func TestClient_GetItems_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/items/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta": map[string]any{"PlayerUID": "uid-1", "Player": "player123"},
			"Inventory": map[string]any{
				"Items": map[string]any{
					"Available": true, "ContainerID": "cont-1",
					"UsedSlots": 1, "MaxSlots": 300, "FreeSlots": 299,
					"Slots": map[string]any{
						"0": map[string]any{"ItemID": "Money", "Count": 100},
					},
				},
				"KeyItems": map[string]any{"Available": false},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetItems(testCtx, "player123")
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if !result.Inventory.Items.Available || result.Inventory.Items.ContainerID != "cont-1" {
		t.Fatalf("unexpected items container: %+v", result.Inventory.Items)
	}
	slot, ok := result.Inventory.Items.Slots["0"]
	if !ok || slot.ItemID != "Money" || slot.Count != 100 {
		t.Fatalf("unexpected item slots: %+v", result.Inventory.Items.Slots)
	}
}

func TestClient_GetTechs_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/techs/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta": map[string]any{
				"PlayerUID": "uid-1", "Player": "player123",
				"UnlockedCount": 2, "LockedCount": 10, "TotalCount": 12,
			},
			"Techs": map[string]any{"Unlocked": []any{"Technology_ElecBaton", "Technology_GrapplingGun"}},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetTechs(testCtx, "player123")
	if err != nil {
		t.Fatalf("GetTechs failed: %v", err)
	}
	if result.Meta.UnlockedCount != 2 || result.Meta.TotalCount != 12 {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
	if len(result.Techs.Unlocked) != 2 || result.Techs.Unlocked[0] != "Technology_ElecBaton" {
		t.Fatalf("unexpected unlocked techs: %+v", result.Techs.Unlocked)
	}
}

func TestClient_GetProgression_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/progression/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"Meta": map[string]any{"PlayerUID": "uid-1", "Player": "player123"},
			"Progression": map[string]any{
				"Player": map[string]any{"level": 42, "exp": 25000, "unusedStatusPoints": 3},
				"Currencies": map[string]any{
					"relics":           map[string]any{"CapturePower": 5},
					"technologyPoints": 10, "ancientTechnologyPoints": 2,
				},
				"Bosses": map[string]any{
					"towerBossDefeatCounts": map[string]any{"TowerBoss_1": 1},
					"normalBossDefeatFlags": map[string]any{"Boss_1": true},
					"raidBossDefeatCounts":  map[string]any{"RaidBoss_1": 2},
					"totalBossDefeatCount":  4, "predatorDefeatCount": 1,
				},
				"Captures": map[string]any{
					"tribeCaptureCount":     10,
					"palCaptureCounts":      map[string]any{"Pengullet": 3},
					"palCaptureBonusCounts": map[string]any{"Pengullet": 1},
					"palButcherCounts":      map[string]any{"Pengullet": 2},
				},
				"Activities": map[string]any{
					"craftItemCounts":         map[string]any{"Money": 5},
					"normalDungeonClearCount": 3,
					"palRankUpCounts":         map[string]any{"Pengullet": 4},
					"arenaSoloClearCounts":    map[string]any{"Arena_1": 1},
					"npcTalkCounts":           map[string]any{"NPC_1": 7},
					"fishingCounts":           map[string]any{"Fish_1": 9},
					"firstFishingComplete":    true,
				},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GetProgression(testCtx, "player123")
	if err != nil {
		t.Fatalf("GetProgression failed: %v", err)
	}
	if result.Progression.Player.Level != 42 || result.Progression.Player.Exp != 25000 {
		t.Fatalf("unexpected player progression: %+v", result.Progression.Player)
	}
	if result.Progression.Currencies.TechnologyPoints != 10 || result.Progression.Currencies.Relics["CapturePower"] != 5 {
		t.Fatalf("unexpected currencies: %+v", result.Progression.Currencies)
	}
	if result.Progression.Bosses.TotalBossDefeatCount != 4 ||
		result.Progression.Bosses.TowerBossDefeatCounts["TowerBoss_1"] != 1 ||
		!result.Progression.Bosses.NormalBossDefeatFlags["Boss_1"] ||
		result.Progression.Bosses.RaidBossDefeatCounts["RaidBoss_1"] != 2 {
		t.Fatalf("unexpected bosses: %+v", result.Progression.Bosses)
	}
	if result.Progression.Captures.TribeCaptureCount != 10 ||
		result.Progression.Captures.PalCaptureCounts["Pengullet"] != 3 ||
		result.Progression.Captures.PalCaptureBonusCounts["Pengullet"] != 1 ||
		result.Progression.Captures.PalButcherCounts["Pengullet"] != 2 {
		t.Fatalf("unexpected captures: %+v", result.Progression.Captures)
	}
	if result.Progression.Activities.CraftItemCounts["Money"] != 5 ||
		result.Progression.Activities.PalRankUpCounts["Pengullet"] != 4 ||
		result.Progression.Activities.ArenaSoloClearCounts["Arena_1"] != 1 ||
		result.Progression.Activities.NPCTalkCounts["NPC_1"] != 7 ||
		result.Progression.Activities.FishingCounts["Fish_1"] != 9 {
		t.Fatalf("unexpected activities: %+v", result.Progression.Activities)
	}
	if !result.Progression.Activities.FirstFishingComplete {
		t.Fatalf("unexpected activities: %+v", result.Progression.Activities)
	}
}

func TestClient_LearnTech_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/learntech/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		writeJSON(t, w, map[string]any{
			"UnlockedCount": 1,
			"Unlocked":      []any{"Technology_ElecBaton"},
			"Skipped":       []any{"Technology_GrapplingGun"},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.LearnTech(testCtx, "player123", "Technology_ElecBaton")
	if err != nil {
		t.Fatalf("LearnTech failed: %v", err)
	}
	if result.UnlockedCount != 1 || len(result.Unlocked) != 1 || result.Unlocked[0] != "Technology_ElecBaton" {
		t.Fatalf("unexpected learn response: %+v", result)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "Technology_GrapplingGun" {
		t.Fatalf("unexpected skipped techs: %+v", result.Skipped)
	}
}

func TestClient_ForgetTech_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/forgettech/player123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"ForgottenCount": 1,
			"Forgotten":      "All",
			"Skipped":        []any{},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.ForgetTech(testCtx, "player123", "All")
	if err != nil {
		t.Fatalf("ForgetTech failed: %v", err)
	}
	if result.ForgottenCount != 1 || !result.Forgotten.All || len(result.Forgotten.IDs) != 0 {
		t.Fatalf("unexpected forget response: %+v", result)
	}
}

func TestForgottenTechs_UnmarshalJSON(t *testing.T) {
	var response ForgetTechResponse

	if err := json.Unmarshal([]byte(`{"Forgotten": ["Technology_A", "Technology_B"]}`), &response); err != nil {
		t.Fatalf("failed to decode array forgotten: %v", err)
	}
	if response.Forgotten.All || len(response.Forgotten.IDs) != 2 || response.Forgotten.IDs[0] != "Technology_A" {
		t.Fatalf("unexpected array forgotten: %+v", response.Forgotten)
	}

	if err := json.Unmarshal([]byte(`{"Forgotten": "Technology_A"}`), &response); err != nil {
		t.Fatalf("failed to decode single forgotten: %v", err)
	}
	if response.Forgotten.All || len(response.Forgotten.IDs) != 1 || response.Forgotten.IDs[0] != "Technology_A" {
		t.Fatalf("unexpected single forgotten: %+v", response.Forgotten)
	}

	if err := json.Unmarshal([]byte(`{"Forgotten": "all"}`), &response); err != nil {
		t.Fatalf("failed to decode lowercase all: %v", err)
	}
	if !response.Forgotten.All {
		t.Fatalf("unexpected lowercase all forgotten: %+v", response.Forgotten)
	}

	if err := json.Unmarshal([]byte(`{"Forgotten": 123}`), &response); err == nil {
		t.Fatal("expected error for non-string non-array forgotten")
	}

	if err := json.Unmarshal([]byte(`{"Forgotten": null}`), &response); err != nil {
		t.Fatalf("failed to decode null forgotten: %v", err)
	}
	if response.Forgotten.All || len(response.Forgotten.IDs) != 0 {
		t.Fatalf("unexpected null forgotten: %+v", response.Forgotten)
	}

	if err := json.Unmarshal([]byte(`{"Forgotten": ""}`), &response); err != nil {
		t.Fatalf("failed to decode empty forgotten: %v", err)
	}
	if response.Forgotten.All || len(response.Forgotten.IDs) != 0 {
		t.Fatalf("unexpected empty forgotten: %+v", response.Forgotten)
	}
}

func TestClient_DeleteBase_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/deletebase/13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request: %v", err)
		}
		if string(body) != "{}" {
			t.Fatalf("unexpected delete base payload: %s", body)
		}
		writeJSON(t, w, map[string]any{
			"BaseCamp": map[string]any{"Id": "13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5", "Summary": "Camp 1"},
			"Deleted": map[string]any{
				"BaseCampPals": 10, "ItemCount": 500, "PalBox": true,
			},
			"Archive": "archives/2023-01-01.json",
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.DeleteBase(testCtx, "13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5")
	if err != nil {
		t.Fatalf("DeleteBase failed: %v", err)
	}
	if result.BaseCamp.ID != "13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5" || result.Deleted.ItemCount != 500 || !result.Deleted.PalBox {
		t.Fatalf("unexpected delete response: %+v", result)
	}
	if result.Archive != "archives/2023-01-01.json" {
		t.Fatalf("unexpected archive: %q", result.Archive)
	}
}

func TestClient_DeleteBase_RejectsNonGUID(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	for _, identifier := range []string{"camp-1", "13b9e8d7", "13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5-extra"} {
		if _, err := client.DeleteBase(testCtx, identifier); err == nil {
			t.Fatalf("expected error for non-GUID identifier %q", identifier)
		}
	}
}

func TestClient_GiveProgression_DocumentedShape(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/progression/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload["EXP"] != float64(25000) {
			t.Fatalf("unexpected progression payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{
			"Granted": map[string]any{"EXP": 25000},
			"Totals":  map[string]any{"TechnologyPoints": 10},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	exp := 25000
	result, err := client.GiveProgression(testCtx, "player123", nil, &exp, nil, nil, nil)
	if err != nil {
		t.Fatalf("GiveProgression failed: %v", err)
	}
	if result.Granted.EXP != 25000 || result.Totals.TechnologyPoints != 10 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GiveProgression_RequestStruct(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/progression/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload["EXP"] != float64(100) {
			t.Fatalf("unexpected progression payload: %+v", payload)
		}
		if _, ok := payload["Relics"]; ok {
			t.Fatalf("unexpected relics in payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{"EXP": 100}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GiveProgression(testCtx, "player123", &GiveProgressionRequest{EXP: 100}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("GiveProgression failed: %v", err)
	}
	if result.Granted.EXP != 100 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GiveProgression_Relics(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/progression/player123", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		relics, ok := payload["Relics"].(map[string]any)
		if !ok || relics["CapturePower"] != float64(5) {
			t.Fatalf("unexpected relics payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{
			"Granted": map[string]any{"Relics": map[string]any{"CapturePower": 5}},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GiveProgression(testCtx, "player123", nil, nil, nil, nil, map[string]int{"CapturePower": 5})
	if err != nil {
		t.Fatalf("GiveProgression failed: %v", err)
	}
	if result.Granted.Relics["CapturePower"] != 5 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GiveItems(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/items/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload struct {
			Items []GiveItem `json:"Items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Items) != 1 || payload.Items[0].ItemID != "Money" || payload.Items[0].Count != 500 {
			t.Fatalf("unexpected items payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{"Items": 500}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GiveItems(testCtx, "player123", []any{"Money", 500})
	if err != nil {
		t.Fatalf("GiveItems failed: %v", err)
	}
	if result.Granted.Items != 500 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GivePals(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/pals/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload struct {
			Pals []GivePal `json:"Pals"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Pals) != 1 || payload.Pals[0].PalID != "Pengullet" || payload.Pals[0].Level != 10 {
			t.Fatalf("unexpected pals payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{"Pals": 1}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GivePals(testCtx, "player123", []any{"Pengullet", 10})
	if err != nil {
		t.Fatalf("GivePals failed: %v", err)
	}
	if result.Granted.Pals != 1 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GivePalTemplates(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/paltemplate/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload struct {
			PalTemplates []string `json:"PalTemplates"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.PalTemplates) != 1 || payload.PalTemplates[0] != "Lamball.json" {
			t.Fatalf("unexpected templates payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{"PalTemplates": 1}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GivePalTemplates(testCtx, "player123", "Lamball.json")
	if err != nil {
		t.Fatalf("GivePalTemplates failed: %v", err)
	}
	if result.Granted.PalTemplates != 1 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GivePalEggs(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/paleggs/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload struct {
			PalEggs []GivePalEgg `json:"PalEggs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.PalEggs) != 1 || payload.PalEggs[0].EggID != "PalEgg_Fire_01" || payload.PalEggs[0].PalID != "Foxparks" || payload.PalEggs[0].Level != 12 {
			t.Fatalf("unexpected pal eggs payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{"PalEggs": 1}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.GivePalEggs(testCtx, "player123", GivePalEgg{EggID: "PalEgg_Fire_01", PalID: "Foxparks", Level: 12})
	if err != nil {
		t.Fatalf("GivePalEggs failed: %v", err)
	}
	if result.Granted.PalEggs != 1 {
		t.Fatalf("unexpected grant result: %+v", result)
	}
}

func TestClient_GiveRecipeMaterials_Validation(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "PalSphere", 1); err == nil {
		t.Fatal("expected error without recipe resolver")
	}

	client, err = NewClient("127.0.0.1", "token123", WithRecipeResolver(func(string) (map[string]int, error) {
		return map[string]int{"Stone": 1}, nil
	}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "", 1); err == nil {
		t.Fatal("expected error for empty product")
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player123", 42, 1); err == nil {
		t.Fatal("expected error for non-string product")
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "PalSphere", 0); err == nil {
		t.Fatal("expected error for non-positive quantity")
	}
}

func TestClient_GiveRecipeMaterials_RejectsOverflow(t *testing.T) {
	client, err := NewClient("127.0.0.1", "token123", WithRecipeResolver(func(string) (map[string]int, error) {
		return map[string]int{"Stone": math.MaxInt}, nil
	}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "PalSphere", 2); err == nil {
		t.Fatal("expected error when scaled count overflows int")
	}
}

func TestClient_GiveRecipeMaterials_HTTP(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/items/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload struct {
			Items []GiveItem `json:"Items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		counts := map[string]int{}
		for _, item := range payload.Items {
			counts[item.ItemID] = item.Count
		}
		if counts["Stone"] != 6 || counts["PaldiumFragment"] != 10 {
			t.Fatalf("unexpected recipe materials payload: %+v", payload)
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL, "token123", WithRecipeResolver(func(product string) (map[string]int, error) {
		if product != "PalSphere" {
			t.Fatalf("unexpected product: %q", product)
		}
		return map[string]int{"Stone": 3, "PaldiumFragment": 5}, nil
	}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "PalSphere", 2); err != nil {
		t.Fatalf("GiveRecipeMaterials failed: %v", err)
	}
}

func TestClient_GiveRecipeMaterials_DeterministicOrder(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/give/items/player123", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Items []GiveItem `json:"Items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		want := []string{"PaldiumFragment", "Stone"}
		if len(payload.Items) != len(want) {
			t.Fatalf("unexpected materials: %+v", payload.Items)
		}
		for i, item := range payload.Items {
			if item.ItemID != want[i] {
				t.Fatalf("unexpected material order: %+v", payload.Items)
			}
		}
		writeJSON(t, w, map[string]any{"Granted": map[string]any{}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL, "token123", WithRecipeResolver(func(string) (map[string]int, error) {
		return map[string]int{"Stone": 3, "PaldiumFragment": 5}, nil
	}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if _, err := client.GiveRecipeMaterials(testCtx, "player123", "PalSphere", 1); err != nil {
		t.Fatalf("GiveRecipeMaterials failed: %v", err)
	}
}

func TestClient_BanIPAndUnbanIP(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/banip/1.2.3.4", func(w http.ResponseWriter, r *http.Request) {
		assertBanIPRequest(t, r)
		writeJSON(t, w, BanIPResponse{Success: true, IP: "1.2.3.4", UserId: "user123", Kicked: 1})
	})
	handler.HandleFunc("/v1/pdapi/unbanip/1.2.3.4", func(w http.ResponseWriter, r *http.Request) {
		assertUnbanIPRequest(t, r)
		writeJSON(t, w, UnbanIPResponse{Success: true, IP: "1.2.3.4"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	banResult, err := client.BanIP(testCtx, "1.2.3.4", &BanIPRequest{Reason: "test reason", UserId: "user123"})
	if err != nil {
		t.Fatalf("BanIP failed: %v", err)
	}
	if !banResult.Success || banResult.IP != "1.2.3.4" || banResult.Kicked != 1 {
		t.Fatalf("unexpected banip response: %+v", banResult)
	}

	unbanResult, err := client.UnbanIP(testCtx, "1.2.3.4", &UnbanIPRequest{Reason: "test reason"})
	if err != nil {
		t.Fatalf("UnbanIP failed: %v", err)
	}
	if !unbanResult.Success || unbanResult.IP != "1.2.3.4" {
		t.Fatalf("unexpected unbanip response: %+v", unbanResult)
	}
}

func assertBanIPRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method %s", r.Method)
	}
	var payload BanIPRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode request: %v", err)
	}
	if payload.Reason != "test reason" || payload.UserId != "user123" {
		t.Fatalf("unexpected banip payload: %+v", payload)
	}
}

func TestClient_BanIPAndUnbanIP_NilRequestSendsEmptyObject(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/banip/1.2.3.4", func(w http.ResponseWriter, r *http.Request) {
		assertEmptyJSONBody(t, r)
		writeJSON(t, w, BanIPResponse{Success: true, IP: "1.2.3.4"})
	})
	handler.HandleFunc("/v1/pdapi/unbanip/1.2.3.4", func(w http.ResponseWriter, r *http.Request) {
		assertEmptyJSONBody(t, r)
		writeJSON(t, w, UnbanIPResponse{Success: true, IP: "1.2.3.4"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	if _, err := client.BanIP(testCtx, "1.2.3.4", nil); err != nil {
		t.Fatalf("BanIP failed: %v", err)
	}
	if _, err := client.UnbanIP(testCtx, "1.2.3.4", nil); err != nil {
		t.Fatalf("UnbanIP failed: %v", err)
	}
}

func assertEmptyJSONBody(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method %s", r.Method)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request: %v", err)
	}
	if string(body) != "{}" {
		t.Fatalf("unexpected empty payload: %s", body)
	}
}

func assertUnbanIPRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method %s", r.Method)
	}
	var payload UnbanIPRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode request: %v", err)
	}
	if payload.Reason != "test reason" {
		t.Fatalf("unexpected unbanip payload: %+v", payload)
	}
}

func TestClient_Unban(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/unban/user123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload UnbanRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Reason != "test reason" {
			t.Fatalf("unexpected unban payload: %+v", payload)
		}
		writeJSON(t, w, UnbanResponse{Success: true, UserId: "user123"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.Unban(testCtx, "user123", "test reason")
	if err != nil {
		t.Fatalf("Unban failed: %v", err)
	}
	if !result.Success || result.UserId != "user123" {
		t.Fatalf("unexpected unban response: %+v", result)
	}
}

func TestClient_Kick(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/kick/player123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload KickRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Reason != "test reason" {
			t.Fatalf("unexpected kick payload: %+v", payload)
		}
		writeJSON(t, w, KickResponse{Success: true, UserId: "player123"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.Kick(testCtx, "player123", "test reason")
	if err != nil {
		t.Fatalf("Kick failed: %v", err)
	}
	if !result.Success || result.UserId != "player123" {
		t.Fatalf("unexpected kick response: %+v", result)
	}
}

func TestClient_Broadcast(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/Broadcast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload BroadcastRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Message != "hello" {
			t.Fatalf("unexpected broadcast payload: %+v", payload)
		}
		writeJSON(t, w, BroadcastResponse{Success: true})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.Broadcast(testCtx, "hello", "admin")
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("unexpected broadcast response: %+v", result)
	}
}

func TestClient_Alert(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/Alert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var payload AlertRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Message != "Server restart in 5 minutes" {
			t.Fatalf("unexpected alert payload: %+v", payload)
		}
		writeJSON(t, w, AlertResponse{Success: true})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.Alert(testCtx, "Server restart in 5 minutes")
	if err != nil {
		t.Fatalf("Alert failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("unexpected alert response: %+v", result)
	}
}

func TestClient_ReloadConfig(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/ReloadConfig", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request: %v", err)
		}
		if string(body) != "{}" {
			t.Fatalf("unexpected reload config payload: %s", body)
		}
		writeJSON(t, w, ReloadConfigResponse{Success: true})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	result, err := client.ReloadConfig(testCtx)
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("unexpected reload config response: %+v", result)
	}
}

func TestClient_EmptySuccessBody(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	if _, err := client.GetVersion(testCtx); err == nil {
		t.Fatal("expected error for empty 2xx body")
	}
}

func TestClient_NonJSONSuccessBody(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "OK")
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	_, err := client.GetVersion(testCtx)
	if err == nil {
		t.Fatal("expected error for non-JSON 2xx body")
	}
	if !strings.Contains(err.Error(), "OK") {
		t.Fatalf("expected body context in error, got: %v", err)
	}
}

func TestClient_NullSuccessBody(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "null")
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	if _, err := client.GetVersion(testCtx); err == nil {
		t.Fatal("expected error for null 2xx body")
	}
}

func TestClient_RedirectStatusIsError(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	internalClient, err := NewClient(srv.URL, "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	_, err = internalClient.GetVersion(testCtx)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("expected *APIError with status 302 from the internal client, got: %v", err)
	}

	client, err := NewClient(srv.URL, "token123", WithHTTPClient(&http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.GetVersion(testCtx)
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("expected *APIError with status 302 from the injected client, got: %v", err)
	}
}

func TestClient_APIErrorEnvelope(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{
			"Error": map[string]any{
				"Code":    "INVALID_REQUEST",
				"Message": "bad request",
				"Details": map[string]any{},
			},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	_, err := client.GetVersion(testCtx)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Method != http.MethodGet {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
	if apiErr.Envelope == nil || apiErr.Envelope.Error.Code != "INVALID_REQUEST" || apiErr.Envelope.Error.Message != "bad request" {
		t.Fatalf("unexpected error envelope: %+v", apiErr.Envelope)
	}
	body, ok := apiErr.ResponseBody.(map[string]any)
	if !ok || body["Error"] == nil {
		t.Fatalf("unexpected error body: %+v", apiErr.ResponseBody)
	}
}

func TestNewClient_RejectsInvalidBaseURLs(t *testing.T) {
	if _, err := NewClient("", "token123"); err == nil {
		t.Fatal("expected error for empty base URL")
	}
	if _, err := NewClient("://bad", "token123"); err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestClient_WithDisplayAddressSendsHeader(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("DisplayAddress"); got != "admin-panel-1" {
			t.Fatalf("unexpected display address header: %q", got)
		}
		writeJSON(t, w, VersionResponse{Version: VersionInfo{Major: 1}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL, "token123", WithDisplayAddress("admin-panel-1"))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if got := client.buildURL("version"); got != srv.URL+"/v1/pdapi/version" {
		t.Fatalf("unexpected URL: %q", got)
	}

	if _, err := client.GetVersion(testCtx); err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
}

type errorRoundTripper struct{}

func (errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("transport failure")
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failure") }
func (errorReadCloser) Close() error             { return nil }

type failingReadRoundTripper struct{}

func (failingReadRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: errorReadCloser{}, Header: make(http.Header)}, nil
}

type failingErrorReadRoundTripper struct{}

func (failingErrorReadRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusInternalServerError, Body: errorReadCloser{}, Header: make(http.Header)}, nil
}

func TestClient_Request_TransportErrors(t *testing.T) {
	doErrClient, err := NewClient("http://127.0.0.1:1", "token123", WithHTTPClient(&http.Client{Transport: errorRoundTripper{}}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := doErrClient.GetVersion(testCtx); err == nil {
		t.Fatal("expected error when the transport fails")
	}

	readErrClient, err := NewClient("http://127.0.0.1:1", "token123", WithHTTPClient(&http.Client{Transport: failingReadRoundTripper{}}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := readErrClient.GetVersion(testCtx); err == nil {
		t.Fatal("expected error when the response body read fails")
	}

	errorReadErrClient, err := NewClient("http://127.0.0.1:1", "token123", WithHTTPClient(&http.Client{Transport: failingErrorReadRoundTripper{}}))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := errorReadErrClient.GetVersion(testCtx); err == nil {
		t.Fatal("expected error when the error response body read fails")
	} else {
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, apiErr.StatusCode)
		}
	}
}

func TestClient_Request_InvalidInputs(t *testing.T) {
	client := newTestClient(t, "http://127.0.0.1:1")

	if _, err := client.request(testCtx, http.MethodPost, "/give/items/p", make(chan int)); err == nil {
		t.Fatal("expected error when the request body cannot be marshaled")
	}
	if _, err := client.request(testCtx, http.MethodGet, "/bad%zz", nil); err == nil {
		t.Fatal("expected error when the request URL is invalid")
	}
}

func TestClient_NonJSONErrorBody(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	_, err := client.GetVersion(testCtx)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
	if body, ok := apiErr.ResponseBody.(string); !ok || body != "boom" {
		t.Fatalf("unexpected error body: %+v", apiErr.ResponseBody)
	}
	if apiErr.Envelope != nil {
		t.Fatalf("unexpected error envelope: %+v", apiErr.Envelope)
	}
}

func TestClient_NonEnvelopeJSONErrorBody(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/pdapi/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(t, w, map[string]any{"detail": "proxy error"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	_, err := client.GetVersion(testCtx)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Envelope != nil {
		t.Fatalf("unexpected error envelope: %+v", apiErr.Envelope)
	}
	body, ok := apiErr.ResponseBody.(map[string]any)
	if !ok || body["detail"] != "proxy error" {
		t.Fatalf("unexpected error body: %+v", apiErr.ResponseBody)
	}
}

func TestClient_EndpointErrorBranches(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client := newTestClient(t, srv.URL)

	tests := []struct {
		name string
		call func(*Client) error
	}{
		{"GetGuilds", func(c *Client) error { _, err := c.GetGuilds(testCtx); return err }},
		{"GetGuild", func(c *Client) error { _, err := c.GetGuild(testCtx, "guild1"); return err }},
		{"GetPlayers", func(c *Client) error { _, err := c.GetPlayers(testCtx); return err }},
		{"GetPlayer", func(c *Client) error { _, err := c.GetPlayer(testCtx, "player1"); return err }},
		{"GetPals", func(c *Client) error { _, err := c.GetPals(testCtx, "player1"); return err }},
		{"GetItems", func(c *Client) error { _, err := c.GetItems(testCtx, "player1"); return err }},
		{"GetTechs", func(c *Client) error { _, err := c.GetTechs(testCtx, "player1"); return err }},
		{"GetProgression", func(c *Client) error { _, err := c.GetProgression(testCtx, "player1"); return err }},
		{"GiveItems", func(c *Client) error { _, err := c.GiveItems(testCtx, "player1", "Money"); return err }},
		{"GivePals", func(c *Client) error { _, err := c.GivePals(testCtx, "player1", "Pengullet"); return err }},
		{"GivePalTemplates", func(c *Client) error { _, err := c.GivePalTemplates(testCtx, "player1", "Lamball.json"); return err }},
		{"GivePalEggs", func(c *Client) error {
			_, err := c.GivePalEggs(testCtx, "player1", GivePalEgg{EggID: "x", PalID: "Foxparks"})
			return err
		}},
		{"GiveProgression", func(c *Client) error {
			_, err := c.GiveProgression(testCtx, "player1", &GiveProgressionRequest{EXP: 10}, nil, nil, nil, nil)
			return err
		}},
		{"LearnTech", func(c *Client) error { _, err := c.LearnTech(testCtx, "player1", "Technology_1"); return err }},
		{"ForgetTech", func(c *Client) error { _, err := c.ForgetTech(testCtx, "player1", "Technology_1"); return err }},
		{"DeleteBase", func(c *Client) error {
			_, err := c.DeleteBase(testCtx, "13b9e8d7-4f2c-42a1-b79e-fc2a9186e4d5")
			return err
		}},
		{"Ban", func(c *Client) error { _, err := c.Ban(testCtx, "player1", "reason", false); return err }},
		{"Unban", func(c *Client) error { _, err := c.Unban(testCtx, "user1", "reason"); return err }},
		{"BanIP", func(c *Client) error { _, err := c.BanIP(testCtx, "1.2.3.4", nil); return err }},
		{"UnbanIP", func(c *Client) error { _, err := c.UnbanIP(testCtx, "1.2.3.4", nil); return err }},
		{"Kick", func(c *Client) error { _, err := c.Kick(testCtx, "player1", "reason"); return err }},
		{"Broadcast", func(c *Client) error { _, err := c.Broadcast(testCtx, "hello", "admin"); return err }},
		{"Alert", func(c *Client) error { _, err := c.Alert(testCtx, "hello"); return err }},
		{"ReloadConfig", func(c *Client) error { _, err := c.ReloadConfig(testCtx); return err }},
		{"SendPlayerMessage", func(c *Client) error {
			_, err := c.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi", UserID: "user1"})
			return err
		}},
		{"GetBanlist", func(c *Client) error {
			_, err := c.GetBanlist(testCtx, map[string]string{"active": "true"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var apiErr *APIError
			if err := tt.call(client); !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got: %v", err)
			}
		})
	}
}

func TestClient_EndpointNormalizeErrors(t *testing.T) {
	client := newTestClient(t, "http://127.0.0.1:1")

	tests := []struct {
		name string
		call func() error
	}{
		{"GiveItems", func() error { _, err := client.GiveItems(testCtx, "player1"); return err }},
		{"GivePals", func() error { _, err := client.GivePals(testCtx, "player1"); return err }},
		{"GivePalTemplates", func() error { _, err := client.GivePalTemplates(testCtx, "player1"); return err }},
		{"GivePalEggs", func() error { _, err := client.GivePalEggs(testCtx, "player1"); return err }},
		{"GiveProgression", func() error {
			_, err := client.GiveProgression(testCtx, "player1", nil, nil, nil, nil, nil)
			return err
		}},
		{"LearnTech", func() error { _, err := client.LearnTech(testCtx, "player1"); return err }},
		{"ForgetTech", func() error { _, err := client.ForgetTech(testCtx, "player1"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected error for invalid inputs")
			}
		})
	}
}

func TestClient_GiveRecipeMaterials_ResolverErrors(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1", "token123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.GiveRecipeMaterials(testCtx, "player1", "PalSphere", 1); err == nil {
		t.Fatal("expected error when the recipe resolver is not configured")
	}

	resolverErr := func(string) (map[string]int, error) { return nil, errors.New("resolve failure") }
	clientWithResolver, err := NewClient("http://127.0.0.1:1", "token123", WithRecipeResolver(resolverErr))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := clientWithResolver.GiveRecipeMaterials(testCtx, "player1", "PalSphere", 1); err == nil {
		t.Fatal("expected error when the recipe resolver fails")
	}

	badCounts := func(string) (map[string]int, error) { return map[string]int{"Stone": 0}, nil }
	clientWithBadCounts, err := NewClient("http://127.0.0.1:1", "token123", WithRecipeResolver(badCounts))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if _, err := clientWithBadCounts.GiveRecipeMaterials(testCtx, "player1", "PalSphere", 1); err == nil {
		t.Fatal("expected error when a material count is non-positive")
	}
}

func TestClient_SendPlayerMessage_NilRequestAndWhitespaceUserID(t *testing.T) {
	client := newTestClient(t, "http://127.0.0.1:1")

	if _, err := client.SendPlayerMessage(testCtx, nil); err == nil {
		t.Fatal("expected error for nil request")
	}
	if _, err := client.SendPlayerMessage(testCtx, &SendPlayerMessageRequest{SendType: "PlayerChat", Message: "hi", UserID: " ", UserIDs: []string{"b"}}); err == nil || err.Error() != "userID must not be empty" {
		t.Fatalf("expected whitespace-only UserID to be rejected with 'userID must not be empty', got: %v", err)
	}
}

func TestGuildStorage_UnmarshalJSON_Errors(t *testing.T) {
	var storage GuildStorage

	if err := json.Unmarshal([]byte(`[1,2]`), &storage); err == nil {
		t.Fatal("expected error for non-object JSON")
	}
	if err := json.Unmarshal([]byte(`{"container_id": 123}`), &storage); err == nil {
		t.Fatal("expected error for wrong-typed known key")
	}
	if err := json.Unmarshal([]byte(`{"0": {"item_id": 123}}`), &storage); err == nil {
		t.Fatal("expected error for wrong-typed slot fields")
	}
}

func TestGuildStorage_UnmarshalJSON_ResetsState(t *testing.T) {
	var storage GuildStorage
	if err := json.Unmarshal([]byte(`{"container_id": "cont-1", "0": {"item_id": "Money", "count": 5}}`), &storage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(storage.Slots) != 1 || storage.ContainerID != "cont-1" {
		t.Fatalf("unexpected first decode: %+v", storage)
	}
	if err := json.Unmarshal([]byte(`{"container_id": "cont-2", "current": 3, "max": 10}`), &storage); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.ContainerID != "cont-2" || storage.Current != 3 || storage.Max != 10 {
		t.Fatalf("unexpected fields after re-decode: %+v", storage)
	}
	if len(storage.Slots) != 0 {
		t.Fatalf("expected empty slots after re-decode, got: %+v", storage.Slots)
	}
}
