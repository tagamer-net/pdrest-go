// Package pdrest provides a typed HTTP client for the PalDefender REST API.
//
// Supports version, guilds, players, pals, items, techs, progression, bans,
// kicks, broadcasts, alerts and item/pal grants via bearer-token
// authentication. All request methods accept a context.Context.
package pdrest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultTimeout = 30 * time.Second
	apiPrefix      = "/v1/pdapi"
	// defaultPort is used by URL normalization when the base URL has no port.
	defaultPort = "17993"
	// maxResponseBody caps the size of success response bodies read by the client.
	maxResponseBody = 10 << 20
	// maxErrorBodyBytes caps the size of error response bodies read by the client.
	maxErrorBodyBytes = 1 << 10
	// drainLimitBytes bounds how much of an oversized body is drained before
	// the connection is released so it can be reused.
	drainLimitBytes = 64 << 10

	userAgent = "pdrest-go"
)

// Option is a configuration function applied to the Client.
type Option func(*Client)

// RESTClient defines the contract with the PalDefender REST API.
type RESTClient interface {
	Close() error
	GetVersion(ctx context.Context) (*VersionInfo, error)
	GetGuilds(ctx context.Context) (*GuildsResponse, error)
	GetGuild(ctx context.Context, guildID string) (*GuildDetail, error)
	GetPlayers(ctx context.Context) (*PlayersResponse, error)
	GetPlayer(ctx context.Context, playerIdentifier string) (*PlayerInfo, error)
	GetPals(ctx context.Context, playerIdentifier string) (*PlayerPalsResponse, error)
	GetItems(ctx context.Context, playerIdentifier string) (*PlayerItemsResponse, error)
	GetTechs(ctx context.Context, playerIdentifier string) (*PlayerTechsResponse, error)
	GetProgression(ctx context.Context, playerIdentifier string) (*PlayerProgressionResponse, error)
	GiveItems(ctx context.Context, playerIdentifier string, items ...ItemInput) (*GrantResult, error)
	GiveRecipeMaterials(ctx context.Context, playerIdentifier string, product any, quantity int) (*GrantResult, error)
	GivePals(ctx context.Context, playerIdentifier string, pals ...PalInput) (*GrantResult, error)
	GivePalTemplates(ctx context.Context, playerIdentifier string, templates ...string) (*GrantResult, error)
	GivePalEggs(ctx context.Context, playerIdentifier string, palEggs ...PalEggInput) (*GrantResult, error)
	GiveProgression(ctx context.Context, playerIdentifier string, request *GiveProgressionRequest, exp, technologyPoints, ancientTechnologyPoints *int, relics map[string]int) (*GrantResult, error)
	LearnTech(ctx context.Context, playerIdentifier string, technology ...TechnologyInput) (*LearnTechResponse, error)
	ForgetTech(ctx context.Context, playerIdentifier string, technology ...TechnologyInput) (*ForgetTechResponse, error)
	DeleteBase(ctx context.Context, baseCampIdentifier string) (*DeleteBaseResponse, error)
	Ban(ctx context.Context, playerIdentifier, reason string, ip bool) (*BanResponse, error)
	Unban(ctx context.Context, userID, reason string) (*UnbanResponse, error)
	BanIP(ctx context.Context, ip string, request *BanIPRequest) (*BanIPResponse, error)
	UnbanIP(ctx context.Context, ip string, request *UnbanIPRequest) (*UnbanIPResponse, error)
	Kick(ctx context.Context, playerIdentifier, reason string) (*KickResponse, error)
	Broadcast(ctx context.Context, message, sender string) (*BroadcastResponse, error)
	Alert(ctx context.Context, message string) (*AlertResponse, error)
	ReloadConfig(ctx context.Context) (*ReloadConfigResponse, error)
	SendPlayerMessage(ctx context.Context, request *SendPlayerMessageRequest) (*SendPlayerMessageResponse, error)
	GetBanlist(ctx context.Context, filters map[string]string) (*BanlistResponse, error)
}

// Compile-time assertion that Client implements RESTClient.
var _ RESTClient = (*Client)(nil)

// Client is the typed HTTP client for the PalDefender REST API.
type Client struct {
	baseURL        string
	bearerToken    string
	displayAddress string
	timeout        time.Duration
	timeoutSet     bool
	httpClient     *http.Client
	ownClient      bool
	recipeResolver RecipeResolver
}

// WithTimeout sets the HTTP timeout of the client.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
		c.timeoutSet = true
		if c.httpClient != nil {
			c.httpClient.Timeout = timeout
		}
	}
}

// WithDisplayAddress sets the address displayed in requests.
func WithDisplayAddress(displayAddress string) Option {
	return func(c *Client) {
		c.displayAddress = displayAddress
	}
}

// WithHTTPClient injects a custom http.Client (used in tests). The injected
// client's Timeout is preserved; a zero Timeout disables the default request
// timeout unless WithTimeout is also set. Close becomes a no-op because the
// injected client is owned by the caller. Keep a Timeout set, or cancel the
// request contexts, unless the caller manages timeouts itself.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithRecipeResolver injects a recipe resolver for GiveRecipeMaterials.
func WithRecipeResolver(resolver RecipeResolver) Option {
	return func(c *Client) {
		c.recipeResolver = resolver
	}
}

// NewClient creates a PalDefender client with a normalized base URL and bearer token.
//
// The base URL host must be an IP address or a valid DNS hostname with
// RFC 1123 style labels (ASCII letters, digits, '-'); internal hostnames
// with other characters, such as underscores, are rejected. The internal
// HTTP client never follows redirects and ignores environment proxies;
// inject a client via WithHTTPClient to customize that behavior.
func NewClient(baseURL, bearerToken string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if bearerToken == "" {
		return nil, errors.New("bearer token is required")
	}

	normalized, err := normalizeBaseURL(baseURL, defaultPort)
	if err != nil {
		return nil, err
	}

	client := &Client{
		baseURL:     normalized,
		bearerToken: bearerToken,
		timeout:     defaultTimeout,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client.httpClient = &http.Client{
			Transport: transport,
			Timeout:   client.timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		client.ownClient = true
	} else if client.timeoutSet {
		client.httpClient.Timeout = client.timeout
	}

	return client, nil
}

// Close releases the idle connections of the underlying HTTP client. It is a
// no-op when a custom client was injected via WithHTTPClient.
func (c *Client) Close() error {
	if c.ownClient {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

func (c *Client) buildURL(endpoint string) string {
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return c.baseURL + apiPrefix + endpoint
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if c.displayAddress != "" {
		req.Header.Set("DisplayAddress", c.displayAddress)
	}
}

func (c *Client) request(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	c.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimitBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return nil, errorFromResponse(method, path, bodyBytes, resp.StatusCode)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(bodyBytes) > maxResponseBody {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimitBytes))
		return nil, fmt.Errorf("response body exceeds maximum size of %d bytes (status %d)", maxResponseBody, resp.StatusCode)
	}

	return bodyBytes, nil
}

// errorFromResponse builds an APIError from a non-2xx response body.
func errorFromResponse(method, path string, bodyBytes []byte, statusCode int) error {
	var payload any
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			payload = strings.TrimSpace(string(bodyBytes))
		}
	}
	apiErr := &APIError{
		StatusCode:   statusCode,
		Method:       method,
		Path:         path,
		ResponseBody: payload,
	}
	if len(bodyBytes) > 0 {
		var envelope ErrorEnvelope
		if err := json.Unmarshal(bodyBytes, &envelope); err == nil && envelope.Error.Code != "" {
			apiErr.Envelope = &envelope
		}
	}
	return apiErr
}

func (c *Client) requestInto(ctx context.Context, method, path string, body any, out any) error {
	bodyBytes, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	if len(bodyBytes) == 0 || bytes.Equal(bytes.TrimSpace(bodyBytes), []byte("null")) {
		return fmt.Errorf("empty or null response body for %s %s", method, path)
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("failed to decode response into model: %w (body: %s)", err, responseSnippet(bodyBytes))
	}
	return nil
}

// responseSnippet returns a bounded excerpt of the response body for error messages.
func responseSnippet(body []byte) string {
	const maxLen = 1024
	text := strings.TrimSpace(string(body))
	if len(text) <= maxLen {
		return text
	}
	cut := text[:maxLen]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + "..."
}

// GetVersion returns the PalDefender version info.
func (c *Client) GetVersion(ctx context.Context) (*VersionInfo, error) {
	var result VersionResponse
	if err := c.requestInto(ctx, http.MethodGet, "/version", nil, &result); err != nil {
		return nil, err
	}
	return &result.Version, nil
}

// GetGuilds lists all guilds.
func (c *Client) GetGuilds(ctx context.Context) (*GuildsResponse, error) {
	var result GuildsResponse
	if err := c.requestInto(ctx, http.MethodGet, "/guilds", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetGuild returns the guild identified by guildID.
func (c *Client) GetGuild(ctx context.Context, guildID string) (*GuildDetail, error) {
	part, err := c.pathPart("guild id", guildID)
	if err != nil {
		return nil, err
	}
	var result GuildResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/guild/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result.Guild, nil
}

// GetPlayers returns the connected players.
func (c *Client) GetPlayers(ctx context.Context) (*PlayersResponse, error) {
	var result PlayersResponse
	if err := c.requestInto(ctx, http.MethodGet, "/players", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPlayer returns the player identified by playerIdentifier.
func (c *Client) GetPlayer(ctx context.Context, playerIdentifier string) (*PlayerInfo, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result PlayerResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/player/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result.Player, nil
}

// GetPals returns the pals of the player identified by playerIdentifier.
func (c *Client) GetPals(ctx context.Context, playerIdentifier string) (*PlayerPalsResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result PlayerPalsResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/pals/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetItems returns the items of the player identified by playerIdentifier.
func (c *Client) GetItems(ctx context.Context, playerIdentifier string) (*PlayerItemsResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result PlayerItemsResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/items/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTechs returns the techs of the player identified by playerIdentifier.
func (c *Client) GetTechs(ctx context.Context, playerIdentifier string) (*PlayerTechsResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result PlayerTechsResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/techs/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProgression returns the progression of the player identified by playerIdentifier.
func (c *Client) GetProgression(ctx context.Context, playerIdentifier string) (*PlayerProgressionResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result PlayerProgressionResponse
	if err := c.requestInto(ctx, http.MethodGet, fmt.Sprintf("/progression/%s", part), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GiveItems grants items to the player identified by playerIdentifier.
func (c *Client) GiveItems(ctx context.Context, playerIdentifier string, items ...ItemInput) (*GrantResult, error) {
	normalized, err := normalizeItemInputs(items)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	return c.doPost(ctx, fmt.Sprintf("/give/items/%s", part), map[string]any{"Items": normalized})
}

// GiveRecipeMaterials grants the materials required to craft product, scaled by quantity.
func (c *Client) GiveRecipeMaterials(ctx context.Context, playerIdentifier string, product any, quantity int) (*GrantResult, error) {
	if c.recipeResolver == nil {
		return nil, errors.New("recipe resolver is not configured")
	}
	productID, err := normalizeProductInput(product)
	if err != nil {
		return nil, err
	}
	if quantity <= 0 {
		return nil, errors.New("quantity must be a positive integer")
	}
	materials, err := c.recipeResolver(productID)
	if err != nil {
		return nil, err
	}
	itemIDs := make([]string, 0, len(materials))
	for itemID := range materials {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Strings(itemIDs)
	items := make([]ItemInput, 0, len(materials))
	for _, itemID := range itemIDs {
		count := materials[itemID]
		if count <= 0 {
			return nil, fmt.Errorf("recipe material count for %s must be positive", itemID)
		}
		if count > math.MaxInt/quantity {
			return nil, fmt.Errorf("recipe material count for %s exceeds the maximum quantity", itemID)
		}
		items = append(items, GiveItem{ItemID: itemID, Count: count * quantity})
	}
	return c.GiveItems(ctx, playerIdentifier, items...)
}

// GivePals grants pals to the player identified by playerIdentifier.
func (c *Client) GivePals(ctx context.Context, playerIdentifier string, pals ...PalInput) (*GrantResult, error) {
	normalized, err := normalizePalInputs(pals)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	return c.doPost(ctx, fmt.Sprintf("/give/pals/%s", part), map[string]any{"Pals": normalized})
}

// GivePalTemplates grants pal templates to the player identified by playerIdentifier.
func (c *Client) GivePalTemplates(ctx context.Context, playerIdentifier string, templates ...string) (*GrantResult, error) {
	normalized, err := normalizeStringInputs(templates, "templates")
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	return c.doPost(ctx, fmt.Sprintf("/give/paltemplate/%s", part), map[string]any{"PalTemplates": normalized})
}

// GivePalEggs grants pal eggs to the player identified by playerIdentifier.
//
// In tuple inputs a trailing ".json" value selects the pal template; use a
// GivePalEgg or map input for template names without the ".json" suffix.
// A single slice of scalar values is treated as one (egg_id, pal_id, level)
// tuple; pass GivePalEgg objects or maps to grant multiple eggs.
func (c *Client) GivePalEggs(ctx context.Context, playerIdentifier string, palEggs ...PalEggInput) (*GrantResult, error) {
	normalized, err := normalizePalEggInputs(palEggs)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	return c.doPost(ctx, fmt.Sprintf("/give/paleggs/%s", part), map[string]any{"PalEggs": normalized})
}

// GiveProgression grants progression (exp, relics, and tech points) to a player.
func (c *Client) GiveProgression(ctx context.Context, playerIdentifier string, request *GiveProgressionRequest, exp, technologyPoints, ancientTechnologyPoints *int, relics map[string]int) (*GrantResult, error) {
	payload, err := normalizeProgressionRequest(request, exp, technologyPoints, ancientTechnologyPoints, relics)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	return c.doPost(ctx, fmt.Sprintf("/give/progression/%s", part), payload)
}

// LearnTech unlocks technologies for the player identified by playerIdentifier.
func (c *Client) LearnTech(ctx context.Context, playerIdentifier string, technology ...TechnologyInput) (*LearnTechResponse, error) {
	payload, err := normalizeTechnologyInputs(technology)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result LearnTechResponse
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/learntech/%s", part), map[string]any{"Technology": payload}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ForgetTech removes technologies from the player identified by playerIdentifier.
func (c *Client) ForgetTech(ctx context.Context, playerIdentifier string, technology ...TechnologyInput) (*ForgetTechResponse, error) {
	payload, err := normalizeTechnologyInputs(technology)
	if err != nil {
		return nil, err
	}
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result ForgetTechResponse
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/forgettech/%s", part), map[string]any{"Technology": payload}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteBase deletes the base camp identified by baseCampIdentifier.
func (c *Client) DeleteBase(ctx context.Context, baseCampIdentifier string) (*DeleteBaseResponse, error) {
	if !baseCampIDPattern.MatchString(baseCampIdentifier) {
		return nil, errors.New("base camp identifier must be a GUID")
	}
	var result DeleteBaseResponse
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/deletebase/%s", baseCampIdentifier), map[string]any{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ban bans the player or IP identified by playerIdentifier; ip selects between the two.
func (c *Client) Ban(ctx context.Context, playerIdentifier, reason string, ip bool) (*BanResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result BanResponse
	payload := BanRequest{Reason: reason, IP: ip}
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/ban/%s", part), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Unban unbans the user identified by userID.
func (c *Client) Unban(ctx context.Context, userID, reason string) (*UnbanResponse, error) {
	part, err := c.pathPart("user id", userID)
	if err != nil {
		return nil, err
	}
	var result UnbanResponse
	payload := UnbanRequest{Reason: reason}
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/unban/%s", part), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BanIP bans the given IP address.
func (c *Client) BanIP(ctx context.Context, ip string, request *BanIPRequest) (*BanIPResponse, error) {
	var payload any
	if request != nil {
		payload = request
	} else {
		payload = map[string]any{}
	}
	part, err := c.pathPart("ip", ip)
	if err != nil {
		return nil, err
	}
	var result BanIPResponse
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/banip/%s", part), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UnbanIP unbans the given IP address.
func (c *Client) UnbanIP(ctx context.Context, ip string, request *UnbanIPRequest) (*UnbanIPResponse, error) {
	var payload any
	if request != nil {
		payload = request
	} else {
		payload = map[string]any{}
	}
	part, err := c.pathPart("ip", ip)
	if err != nil {
		return nil, err
	}
	var result UnbanIPResponse
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/unbanip/%s", part), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Kick kicks the player identified by playerIdentifier with a reason.
func (c *Client) Kick(ctx context.Context, playerIdentifier, reason string) (*KickResponse, error) {
	part, err := c.pathPart("player identifier", playerIdentifier)
	if err != nil {
		return nil, err
	}
	var result KickResponse
	payload := KickRequest{Reason: reason}
	if err := c.requestInto(ctx, http.MethodPost, fmt.Sprintf("/kick/%s", part), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Broadcast sends a message to the server chat with the given sender.
func (c *Client) Broadcast(ctx context.Context, message, sender string) (*BroadcastResponse, error) {
	var result BroadcastResponse
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("message is required")
	}
	payload := BroadcastRequest{Message: message, Sender: sender}
	if err := c.requestInto(ctx, http.MethodPost, "/Broadcast", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Alert sends a PalDefender alert with the given message.
func (c *Client) Alert(ctx context.Context, message string) (*AlertResponse, error) {
	var result AlertResponse
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("message is required")
	}
	payload := AlertRequest{Message: message}
	if err := c.requestInto(ctx, http.MethodPost, "/Alert", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReloadConfig reloads the PalDefender configuration.
func (c *Client) ReloadConfig(ctx context.Context) (*ReloadConfigResponse, error) {
	var result ReloadConfigResponse
	if err := c.requestInto(ctx, http.MethodPost, "/ReloadConfig", map[string]any{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendPlayerMessage sends a direct message to a player.
func (c *Client) SendPlayerMessage(ctx context.Context, request *SendPlayerMessageRequest) (*SendPlayerMessageResponse, error) {
	var result SendPlayerMessageResponse
	if request == nil {
		return nil, errors.New("request is required")
	}
	if strings.TrimSpace(request.SendType) == "" {
		return nil, errors.New("sendType is required")
	}
	if strings.TrimSpace(request.Message) == "" {
		return nil, errors.New("message is required")
	}
	if (request.UserID != "") == (len(request.UserIDs) > 0) {
		return nil, errors.New("exactly one of UserID or UserIDs must be provided")
	}
	for _, id := range request.UserIDs {
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("userIDs must not contain empty entries")
		}
	}
	if request.UserID != "" && strings.TrimSpace(request.UserID) == "" {
		return nil, errors.New("userID must not be empty")
	}
	if err := c.requestInto(ctx, http.MethodPost, "/SendPlayerMessage", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// banlistFilters are the query filter names supported by the ban list endpoint.
var banlistFilters = map[string]struct{}{
	"active":     {},
	"entryType":  {},
	"userId":     {},
	"ip":         {},
	"userIP":     {},
	"issuerType": {},
	"issuerName": {},
	"issuerIP":   {},
	"reason":     {},
	"q":          {},
}

// baseCampIDPattern matches the GUID format required for base camp identifiers.
var baseCampIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// GetBanlist returns the ban list, optionally filtered.
func (c *Client) GetBanlist(ctx context.Context, filters map[string]string) (*BanlistResponse, error) {
	path := "/banlist"
	if len(filters) > 0 {
		values := url.Values{}
		for key, value := range filters {
			if _, ok := banlistFilters[key]; !ok {
				return nil, fmt.Errorf("unsupported banlist filter %q", key)
			}
			values.Set(key, value)
		}
		path = fmt.Sprintf("%s?%s", path, values.Encode())
	}
	var result BanlistResponse
	if err := c.requestInto(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) doPost(ctx context.Context, path string, body any) (*GrantResult, error) {
	var result GrantResult
	if err := c.requestInto(ctx, http.MethodPost, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) pathPart(label, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must not be empty", label)
	}
	return url.PathEscape(value), nil
}

// normalizeBaseURL ensures a valid base URL with an http/https scheme and the
// scheme default port when missing (17993 for http, 443 for https). Supports
// IPv6 hosts (with brackets); zone-scoped IPv6 addresses keep their
// percent-escaped zone delimiter in the result. Paths, query strings, fragments
// and userinfo are rejected because the client always targets the /v1/pdapi
// endpoints directly. Hosts must be valid IP addresses or DNS hostnames.
func normalizeBaseURL(raw, defaultPort string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("base URL is required")
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", redactUserinfo(raw), err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q in base URL", u.Scheme)
	}
	if u.User != nil {
		return "", fmt.Errorf("base URL must not contain userinfo: %q", redactUserinfo(raw))
	}

	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("base URL must not contain a path: %q", raw)
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("base URL must not contain a query string: %q", raw)
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("base URL must not contain a fragment: %q", raw)
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("base URL missing host: %q", raw)
	}
	if !validHostname(host) {
		return "", fmt.Errorf("invalid hostname %q in base URL", host)
	}

	port := u.Port()
	if port == "" {
		port = defaultPortForScheme(u.Scheme, defaultPort)
	} else if port, err = validatePort(port); err != nil {
		return "", fmt.Errorf("%w in base URL", err)
	}

	// url.Parse unescapes %25 in zone-scoped IPv6 hosts; the rebuilt URL must
	// keep the percent-encoding or the next parse of a request URL fails with
	// an invalid escape error.
	host = strings.ReplaceAll(host, "%", "%25")
	return fmt.Sprintf("%s://%s", u.Scheme, net.JoinHostPort(host, port)), nil
}

// defaultPortForScheme returns the port to use when the URL has no port:
// 443 for https, otherwise the given fallback (the REST API default port).
func defaultPortForScheme(scheme, fallback string) string {
	if scheme == "https" {
		return "443"
	}
	return fallback
}

// validHostname reports whether host is a valid IP address or DNS hostname.
// A single trailing dot (FQDN form) is accepted.
func validHostname(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	if len(host) > 0 && host[len(host)-1] == '.' {
		host = host[:len(host)-1]
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	if !validHostnameLabels(host) {
		return false
	}
	return host[0] != '-' && host[len(host)-1] != '-'
}

// validHostnameLabels checks DNS label structure: non-empty labels of at most
// 63 characters containing only ASCII letters, digits or '-', with '-' only
// allowed inside a label.
func validHostnameLabels(host string) bool {
	labelLen := 0
	for i := 0; i < len(host); i++ {
		r := host[i]
		if r == '.' {
			if labelLen == 0 || host[i-1] == '-' {
				return false
			}
			labelLen = 0
			continue
		}
		if !validLabelChar(r, labelLen) {
			return false
		}
		labelLen++
		if labelLen > 63 {
			return false
		}
	}
	return true
}

// validLabelChar reports whether r may appear at position labelLen of a DNS
// label. A '-' is only allowed when the label is not empty.
func validLabelChar(r byte, labelLen int) bool {
	if r == '-' && labelLen == 0 {
		return false
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
}

// redactUserinfo masks credentials in raw so error messages referencing the
// URL do not leak the password into logs.
func redactUserinfo(raw string) string {
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return raw
	}
	at := strings.LastIndexByte(raw[schemeEnd+3:], '@')
	if at < 0 {
		return raw
	}
	return raw[:schemeEnd+3] + "xxxxx" + raw[schemeEnd+3+at:]
}

// validatePort checks that port is a number within the valid TCP range and
// returns it in canonical form without leading zeros. Only plain digit
// strings are accepted (RFC 3986 ports contain no sign).
func validatePort(port string) (string, error) {
	if port == "" || port[0] < '0' || port[0] > '9' {
		return "", fmt.Errorf("invalid port %q", port)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: %w", port, err)
	}
	if n < 1 || n > 65535 {
		return "", fmt.Errorf("port %q out of range (1-65535)", port)
	}
	return strconv.Itoa(n), nil
}
