package agentqualification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

const (
	OpenCodeStructuredV33Transport          = "opencode_server_api_json_schema_v2"
	OpenCodeStructuredV33Version            = "1.18.18"
	OpenCodeStructuredV33HTTPTimeoutSeconds = 120
	openCodeV33RoutePolicy                  = "enforced_single_route_no_fallback"
)

type OpenCodeStructuredV33Capsule struct {
	BindingID, OpenCodeVersion, Transport, Method, PathTemplate string
	SessionID, RequestMessageID, Workspace                      string
	ServerArgs, EnvironmentTemplate                             []string
	InlineConfig, RequestBody                                   json.RawMessage
	InlineConfigSHA256, ResolvedConfigSHA256                    string
	RequestBodySHA256, PromptSHA256, SchemaSHA256               string
	RoutePolicy                                                 string
	RetryCount                                                  int
	HTTPTimeoutSeconds                                          int
	GeneratedTokenCeiling                                       int64
	ToolsDenied, CLIFormatJSONEnforcement                       bool
}

type openCodeStructuredV33Request struct {
	MessageID string `json:"messageID"`
	Model     struct {
		ProviderID string `json:"providerID"`
		ModelID    string `json:"modelID"`
	} `json:"model"`
	Agent   string                      `json:"agent"`
	Tools   map[string]bool             `json:"tools"`
	Format  openCodeStructuredV33Format `json:"format"`
	Variant string                      `json:"variant"`
	Parts   []openCodeStructuredV33Part `json:"parts"`
}
type openCodeStructuredV33Format struct {
	Type       string          `json:"type"`
	Schema     json.RawMessage `json:"schema"`
	RetryCount int             `json:"retryCount"`
}
type openCodeStructuredV33Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func BuildOpenCodeStructuredV33Capsule(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding) (OpenCodeStructuredV33Capsule, error) {
	s := hashRealisticNextBytes([]byte(b.BindingID))[:20]
	return BuildOpenCodeStructuredV33CapsuleForSession(d, b, "ses_"+s, "msg_"+s, "C:/sealed/r05", "")
}
func BuildOpenCodeStructuredV33CapsuleForSession(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, sessionID, messageID, workspace, resolvedHash string) (OpenCodeStructuredV33Capsule, error) {
	p, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return OpenCodeStructuredV33Capsule{}, err
	}
	return BuildOpenCodeStructuredV33CapsuleForPlan(d, b, p, sessionID, messageID, workspace, resolvedHash)
}

// BuildOpenCodeStructuredV33CapsuleForPlan accepts the exact per-turn plan
// emitted by the unchanged v32 controller. Loop turns may add only one
// controller_result suffix; the frozen schema, model, effort, and config cannot
// change.
func BuildOpenCodeStructuredV33CapsuleForPlan(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, p OpenCodeScreeningPlan, sessionID, messageID, workspace, resolvedHash string) (OpenCodeStructuredV33Capsule, error) {
	base, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return OpenCodeStructuredV33Capsule{}, err
	}
	validPrompt := p.Prompt == base.Prompt
	if b.Protocol.ProtocolID == "one-action-loop-v1" && strings.HasPrefix(p.Prompt, base.Prompt+"\n<controller_result>") && strings.HasSuffix(p.Prompt, "</controller_result>") {
		validPrompt = true
	}
	if !validPrompt || p.PromptSHA256 != hashRealisticNextBytes([]byte(p.Prompt)) || !bytes.Equal(p.Schema, base.Schema) || p.SchemaSHA256 != base.SchemaSHA256 || !bytes.Equal(p.InlineConfig, base.InlineConfig) || p.InlineConfigSHA256 != base.InlineConfigSHA256 || p.Model != base.Model || p.Effort != base.Effort || p.CandidateID != base.CandidateID || p.CaseID != base.CaseID || p.ProviderFallbacks == nil || *p.ProviderFallbacks {
		return OpenCodeStructuredV33Capsule{}, fmt.Errorf("v33 per-turn plan escaped frozen v32 boundary")
	}
	model := strings.TrimPrefix(p.Model, "openrouter/")
	if model == p.Model || model == "" || p.ProviderFallbacks == nil || *p.ProviderFallbacks {
		return OpenCodeStructuredV33Capsule{}, fmt.Errorf("v33 requires one exact OpenRouter route")
	}
	if !strings.HasPrefix(sessionID, "ses_") || !strings.HasPrefix(messageID, "msg_") || strings.TrimSpace(workspace) == "" {
		return OpenCodeStructuredV33Capsule{}, fmt.Errorf("v33 request identity invalid")
	}
	if resolvedHash == "" {
		resolvedHash = p.InlineConfigSHA256
	}
	var req openCodeStructuredV33Request
	req.MessageID = messageID
	req.Model.ProviderID = "openrouter"
	req.Model.ModelID = model
	req.Agent = "qualification"
	req.Tools = map[string]bool{"*": false}
	req.Format = openCodeStructuredV33Format{"json_schema", append(json.RawMessage(nil), p.Schema...), 0}
	req.Variant = p.Effort
	req.Parts = []openCodeStructuredV33Part{{"text", p.Prompt}}
	body, err := json.Marshal(req)
	if err != nil {
		return OpenCodeStructuredV33Capsule{}, err
	}
	c := OpenCodeStructuredV33Capsule{BindingID: b.BindingID, OpenCodeVersion: OpenCodeStructuredV33Version, Transport: OpenCodeStructuredV33Transport, Method: "POST", PathTemplate: "/session/{sessionID}/message?directory={url-escaped-workspace}", SessionID: sessionID, RequestMessageID: messageID, Workspace: workspace, ServerArgs: []string{"serve", "--pure", "--hostname", "127.0.0.1", "--port", "<ephemeral-port>"}, EnvironmentTemplate: []string{"OPENCODE_CONFIG_CONTENT=<exact-inline-config>", "OPENCODE_CONFIG_DIR=<ephemeral-empty-config-dir>", "OPENCODE_DISABLE_GLOBAL_CONFIG=true", "OPENCODE_DISABLE_AUTOUPDATE=true", "OPENCODE_DISABLE_PRUNE=true", fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", d.PerObservationGeneratedTokenCeiling)}, InlineConfig: append(json.RawMessage(nil), p.InlineConfig...), InlineConfigSHA256: p.InlineConfigSHA256, ResolvedConfigSHA256: resolvedHash, RequestBody: body, RequestBodySHA256: hashRealisticNextBytes(body), PromptSHA256: p.PromptSHA256, SchemaSHA256: p.SchemaSHA256, RoutePolicy: openCodeV33RoutePolicy, HTTPTimeoutSeconds: OpenCodeStructuredV33HTTPTimeoutSeconds, GeneratedTokenCeiling: d.PerObservationGeneratedTokenCeiling, ToolsDenied: true}
	if err := ValidateOpenCodeStructuredV33CapsuleForPlan(d, b, p, c); err != nil {
		return OpenCodeStructuredV33Capsule{}, err
	}
	return c, nil
}
func ValidateOpenCodeStructuredV33Capsule(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, c OpenCodeStructuredV33Capsule) error {
	p, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return err
	}
	return ValidateOpenCodeStructuredV33CapsuleForPlan(d, b, p, c)
}

func ValidateOpenCodeStructuredV33CapsuleForPlan(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, p OpenCodeScreeningPlan, c OpenCodeStructuredV33Capsule) error {
	base, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return err
	}
	validPrompt := p.Prompt == base.Prompt || (b.Protocol.ProtocolID == "one-action-loop-v1" && strings.HasPrefix(p.Prompt, base.Prompt+"\n<controller_result>") && strings.HasSuffix(p.Prompt, "</controller_result>"))
	if !validPrompt || p.PromptSHA256 != hashRealisticNextBytes([]byte(p.Prompt)) || !bytes.Equal(p.Schema, base.Schema) || !bytes.Equal(p.InlineConfig, base.InlineConfig) || p.Model != base.Model || p.Effort != base.Effort {
		return fmt.Errorf("v33 dynamic plan drift")
	}
	if c.BindingID != b.BindingID || c.OpenCodeVersion != OpenCodeStructuredV33Version || c.Transport != OpenCodeStructuredV33Transport || c.Method != "POST" || c.RoutePolicy != openCodeV33RoutePolicy {
		return fmt.Errorf("v33 transport/route drift")
	}
	if !strings.HasPrefix(c.SessionID, "ses_") || !strings.HasPrefix(c.RequestMessageID, "msg_") || c.Workspace == "" || c.ResolvedConfigSHA256 == "" {
		return fmt.Errorf("v33 request identity/config proof missing")
	}
	if !c.ToolsDenied || c.CLIFormatJSONEnforcement || c.RetryCount != 0 || c.HTTPTimeoutSeconds != OpenCodeStructuredV33HTTPTimeoutSeconds || c.GeneratedTokenCeiling < 0 || c.GeneratedTokenCeiling > d.PerObservationGeneratedTokenCeiling || !contains(c.EnvironmentTemplate, fmt.Sprintf("OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=%d", c.GeneratedTokenCeiling)) || contains(c.ServerArgs, "run") || contains(c.ServerArgs, "--format") || !reflect.DeepEqual(c.ServerArgs, []string{"serve", "--pure", "--hostname", "127.0.0.1", "--port", "<ephemeral-port>"}) {
		return fmt.Errorf("v33 server/no-retry drift")
	}
	if !bytes.Equal(c.InlineConfig, p.InlineConfig) || c.InlineConfigSHA256 != hashRealisticNextBytes(c.InlineConfig) || c.InlineConfigSHA256 != p.InlineConfigSHA256 {
		return fmt.Errorf("v33 config drift")
	}
	if c.RequestBodySHA256 != hashRealisticNextBytes(c.RequestBody) || c.PromptSHA256 != p.PromptSHA256 || c.SchemaSHA256 != p.SchemaSHA256 {
		return fmt.Errorf("v33 byte hash drift")
	}
	var r openCodeStructuredV33Request
	if err := decodeOpenCodeStructuredV33Exact(c.RequestBody, &r); err != nil {
		return err
	}
	if r.MessageID != c.RequestMessageID || r.Model.ProviderID != "openrouter" || r.Model.ModelID != strings.TrimPrefix(p.Model, "openrouter/") || r.Agent != "qualification" || r.Variant != p.Effort || len(r.Tools) != 1 || r.Tools["*"] || r.Format.Type != "json_schema" || r.Format.RetryCount != 0 || len(r.Parts) != 1 || r.Parts[0].Type != "text" || r.Parts[0].Text != p.Prompt || !bytes.Equal(r.Format.Schema, p.Schema) {
		return fmt.Errorf("v33 exact request drift")
	}
	return nil
}

type openCodeV33Tokens struct {
	Total     *int64 `json:"total"`
	Input     int64  `json:"input"`
	Output    int64  `json:"output"`
	Reasoning int64  `json:"reasoning"`
	Cache     struct {
		Read  int64 `json:"read"`
		Write int64 `json:"write"`
	} `json:"cache"`
}
type openCodeV33Info struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	Time      struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
	ParentID   string `json:"parentID"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Mode       string `json:"mode"`
	Agent      string `json:"agent"`
	Path       struct {
		Cwd  string `json:"cwd"`
		Root string `json:"root"`
	} `json:"path"`
	Cost       json.Number       `json:"cost"`
	Tokens     openCodeV33Tokens `json:"tokens"`
	Structured json.RawMessage   `json:"structured,omitempty"`
	Error      json.RawMessage   `json:"error,omitempty"`
	Summary    json.RawMessage   `json:"summary,omitempty"`
	Metadata   json.RawMessage   `json:"metadata,omitempty"`
	Variant    string            `json:"variant"`
	Finish     string            `json:"finish"`
}
type openCodeV33Envelope struct {
	Info  openCodeV33Info   `json:"info"`
	Parts []json.RawMessage `json:"parts"`
}
type OpenCodeStructuredV33Error struct {
	Name    string
	Message string
	Retries int64
}
type OpenCodeStructuredV33Receipt struct {
	Outcome                                                                                                                        string
	Structured                                                                                                                     json.RawMessage
	StructuredErr                                                                                                                  *OpenCodeStructuredV33Error
	ProviderID, ModelID, Variant, Finish                                                                                           string
	SessionID, RequestMessageID, AssistantMessageID                                                                                string
	HTTPStatus                                                                                                                     int
	ContentType                                                                                                                    string
	ProviderTurns, StepStarts, StepFinishes                                                                                        int
	Usage                                                                                                                          ProtocolAblationV32Usage
	CacheRead, CacheWrite                                                                                                          int64
	Parts                                                                                                                          []json.RawMessage
	RawResponse                                                                                                                    json.RawMessage
	RawSHA256, CapsuleSHA256, RequestBodySHA256, InlineConfigSHA256, ResolvedConfigSHA256, PromptSHA256, SchemaSHA256, RoutePolicy string
}

func ParseOpenCodeStructuredV33HTTPResponse(c OpenCodeStructuredV33Capsule, status int, contentType string, raw []byte) (OpenCodeStructuredV33Receipt, error) {
	var z OpenCodeStructuredV33Receipt
	if status != http.StatusOK {
		return z, fmt.Errorf("v33 HTTP status %d", status)
	}
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil || media != "application/json" {
		return z, fmt.Errorf("v33 non-JSON content type")
	}
	var e openCodeV33Envelope
	if err := decodeOpenCodeStructuredV33Exact(raw, &e); err != nil {
		return z, err
	}
	i := e.Info
	if !strings.HasPrefix(i.ID, "msg_") || i.SessionID != c.SessionID || i.Role != "assistant" || i.ParentID != c.RequestMessageID || i.ProviderID == "" || i.ModelID == "" || i.Mode == "" || i.Agent != "qualification" || i.Path.Cwd == "" || i.Path.Root == "" || i.Variant == "" || i.Finish == "" || i.Time.Created <= 0 || i.Time.Completed < i.Time.Created || i.Cost == "" {
		return z, fmt.Errorf("v33 incomplete/unrelated assistant envelope")
	}
	if err := validateOpenCodeV33TokenTotal(i.Tokens); err != nil {
		return z, fmt.Errorf("v33 negative usage")
	}
	cost, err := decimalUSDMicros(i.Cost.String())
	if err != nil {
		return z, err
	}
	parts := make([]json.RawMessage, len(e.Parts))
	ids := map[string]bool{}
	starts, finishes := 0, 0
	var stepCost int64
	var stepTokens openCodeV33Tokens
	for n, rp := range e.Parts {
		parts[n] = append(json.RawMessage(nil), rp...)
		var base struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionID"`
			MessageID string `json:"messageID"`
			Type      string `json:"type"`
		}
		if err := json.Unmarshal(rp, &base); err != nil || base.ID == "" || base.SessionID != c.SessionID || base.MessageID != i.ID || base.Type == "" || ids[base.ID] {
			return z, fmt.Errorf("v33 malformed/unrelated part")
		}
		ids[base.ID] = true
		switch base.Type {
		case "step-start":
			starts++
		case "step-finish":
			finishes++
			var p struct {
				ID        string            `json:"id"`
				SessionID string            `json:"sessionID"`
				MessageID string            `json:"messageID"`
				Type      string            `json:"type"`
				Reason    string            `json:"reason"`
				Snapshot  string            `json:"snapshot,omitempty"`
				Cost      json.Number       `json:"cost"`
				Tokens    openCodeV33Tokens `json:"tokens"`
			}
			if err := decodeOpenCodeStructuredV33Exact(rp, &p); err != nil || p.Reason == "" || p.Cost == "" {
				return z, fmt.Errorf("v33 malformed step finish")
			}
			if err := validateOpenCodeV33TokenTotal(p.Tokens); err != nil {
				return z, fmt.Errorf("v33 malformed step token total: %w", err)
			}
			stepCost, err = decimalUSDMicros(p.Cost.String())
			if err != nil {
				return z, err
			}
			stepTokens = p.Tokens
		case "retry", "tool", "subtask", "patch":
			return z, fmt.Errorf("v33 forbidden %s part", base.Type)
		case "text", "reasoning", "snapshot":
		default:
			return z, fmt.Errorf("v33 unknown part %s", base.Type)
		}
	}
	if starts != 1 || finishes != 1 || stepCost != cost || !reflect.DeepEqual(stepTokens, i.Tokens) {
		return z, fmt.Errorf("v33 lacks one reconciled provider step")
	}
	input, ok := safeAddV33(i.Tokens.Input, i.Tokens.Cache.Read, i.Tokens.Cache.Write)
	if !ok {
		return z, fmt.Errorf("v33 token overflow")
	}
	out := OpenCodeStructuredV33Receipt{ProviderID: i.ProviderID, ModelID: i.ModelID, Variant: i.Variant, Finish: i.Finish, SessionID: c.SessionID, RequestMessageID: c.RequestMessageID, AssistantMessageID: i.ID, HTTPStatus: status, ContentType: media, ProviderTurns: 1, StepStarts: starts, StepFinishes: finishes, Usage: ProtocolAblationV32Usage{ProviderTurns: 1, InputTokens: input, OutputTokens: i.Tokens.Output, ReasoningTokens: i.Tokens.Reasoning, CostMicros: cost}, CacheRead: i.Tokens.Cache.Read, CacheWrite: i.Tokens.Cache.Write, Parts: parts, RawResponse: append(json.RawMessage(nil), raw...), RawSHA256: hashRealisticNextBytes(raw), CapsuleSHA256: hashOpenCodeV33Capsule(c), RequestBodySHA256: c.RequestBodySHA256, InlineConfigSHA256: c.InlineConfigSHA256, ResolvedConfigSHA256: c.ResolvedConfigSHA256, PromptSHA256: c.PromptSHA256, SchemaSHA256: c.SchemaSHA256, RoutePolicy: c.RoutePolicy}
	s, er := bytes.TrimSpace(i.Structured), bytes.TrimSpace(i.Error)
	if len(s) > 0 && !bytes.Equal(s, []byte("null")) {
		if len(er) > 0 && !bytes.Equal(er, []byte("null")) {
			return z, fmt.Errorf("v33 ambiguous outcome")
		}
		var req openCodeStructuredV33Request
		if err := decodeOpenCodeStructuredV33Exact(c.RequestBody, &req); err != nil {
			return z, err
		}
		if err := validateOpenCodeV33JSONSchema(req.Format.Schema, s); err != nil {
			return z, fmt.Errorf("v33 schema violation: %w", err)
		}
		out.Outcome = "structured"
		out.Structured = append(json.RawMessage(nil), s...)
		return out, nil
	}
	if len(er) == 0 || bytes.Equal(er, []byte("null")) {
		return z, fmt.Errorf("v33 missing outcome")
	}
	var se struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
			Retries int64  `json:"retries"`
		} `json:"data"`
	}
	if err := decodeOpenCodeStructuredV33Exact(er, &se); err != nil || se.Name != "StructuredOutputError" || se.Data.Message == "" || se.Data.Retries != 0 {
		return z, fmt.Errorf("v33 malformed/nonzero-retry structured error")
	}
	out.Outcome = "structured_output_error"
	out.StructuredErr = &OpenCodeStructuredV33Error{se.Name, se.Data.Message, se.Data.Retries}
	return out, nil
}
func ValidateOpenCodeStructuredV33Receipt(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, c OpenCodeStructuredV33Capsule, r OpenCodeStructuredV33Receipt) error {
	p, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return err
	}
	return ValidateOpenCodeStructuredV33ReceiptForPlan(d, b, p, c, r)
}
func ValidateOpenCodeStructuredV33ReceiptForPlan(d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, p OpenCodeScreeningPlan, c OpenCodeStructuredV33Capsule, r OpenCodeStructuredV33Receipt) error {
	if err := ValidateOpenCodeStructuredV33CapsuleForPlan(d, b, p, c); err != nil {
		return err
	}
	reparsed, err := ParseOpenCodeStructuredV33HTTPResponse(c, r.HTTPStatus, r.ContentType, r.RawResponse)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(reparsed, r) {
		return fmt.Errorf("v33 derived receipt mismatch")
	}
	if r.ProviderID != "openrouter" || r.ModelID != strings.TrimPrefix(b.Candidate.Model, "openrouter/") || r.Variant != b.Candidate.Effort || r.RoutePolicy != openCodeV33RoutePolicy {
		return fmt.Errorf("v33 receipt route drift")
	}
	total, ok := safeAddV33(r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.ReasoningTokens)
	if !ok {
		return fmt.Errorf("v33 total overflow")
	}
	generated, ok := safeAddV33(r.Usage.OutputTokens, r.Usage.ReasoningTokens)
	if !ok {
		return fmt.Errorf("v33 generated overflow")
	}
	reservation, ok := OpenCodeStructuredV33CostReservation(d, b.Candidate)
	if !ok {
		return fmt.Errorf("v33 price-derived reservation invalid")
	}
	if total > d.PerObservationTotalTokenReservation || generated > d.PerObservationGeneratedTokenCeiling || r.Usage.CostMicros > reservation {
		return fmt.Errorf("v33 reservation exceeded")
	}
	return nil
}

type OpenCodeStructuredV33HTTPClient struct {
	Client  *http.Client
	BaseURL string
}

func (h OpenCodeStructuredV33HTTPClient) PromptOnce(ctx context.Context, d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, c OpenCodeStructuredV33Capsule) (OpenCodeStructuredV33Receipt, error) {
	p, err := BuildProtocolAblationV32OpenCodePlan(d, b)
	if err != nil {
		return OpenCodeStructuredV33Receipt{}, err
	}
	return h.PromptOncePlan(ctx, d, b, p, c)
}

func (h OpenCodeStructuredV33HTTPClient) PromptOncePlan(ctx context.Context, d ProtocolAblationV32Definition, b ProtocolAblationV32Binding, p OpenCodeScreeningPlan, c OpenCodeStructuredV33Capsule) (OpenCodeStructuredV33Receipt, error) {
	if err := ValidateOpenCodeStructuredV33CapsuleForPlan(d, b, p, c); err != nil {
		return OpenCodeStructuredV33Receipt{}, err
	}
	u, err := url.Parse(h.BaseURL)
	if err != nil || u.Scheme != "http" || !isLoopbackV33(u.Hostname()) {
		return OpenCodeStructuredV33Receipt{}, fmt.Errorf("v33 server must be loopback HTTP")
	}
	target := strings.TrimRight(h.BaseURL, "/") + "/session/" + url.PathEscape(c.SessionID) + "/message?directory=" + url.QueryEscape(c.Workspace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(c.RequestBody))
	if err != nil {
		return OpenCodeStructuredV33Receipt{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := h.Client
	if client == nil {
		client = &http.Client{}
	}
	// Clone so redirects cannot silently turn the single logical request into
	// multiple HTTP/provider attempts. PromptOnce itself has no retry loop.
	oneShot := *client
	oneShot.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := oneShot.Do(req)
	if err != nil {
		return OpenCodeStructuredV33Receipt{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return OpenCodeStructuredV33Receipt{}, err
	}
	return ParseOpenCodeStructuredV33HTTPResponse(c, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
}

func validateOpenCodeV33JSONSchema(sr, vr []byte) error {
	var s map[string]any
	var v any
	if err := json.Unmarshal(sr, &s); err != nil {
		return err
	}
	if err := json.Unmarshal(vr, &v); err != nil {
		return err
	}
	return validateOpenCodeV33SchemaNode(s, v, "$")
}
func validateOpenCodeV33SchemaNode(s map[string]any, v any, path string) error {
	t, ok := s["type"].(string)
	if !ok {
		return fmt.Errorf("%s missing type", path)
	}
	switch t {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s not object", path)
		}
		props, _ := s["properties"].(map[string]any)
		req, _ := s["required"].([]any)
		for _, x := range req {
			k, _ := x.(string)
			if _, ok := obj[k]; !ok {
				return fmt.Errorf("%s.%s required", path, k)
			}
		}
		if ap, ok := s["additionalProperties"].(bool); ok && !ap {
			for k := range obj {
				if _, ok := props[k]; !ok {
					return fmt.Errorf("%s.%s additional", path, k)
				}
			}
		}
		for k, val := range obj {
			if child, ok := props[k].(map[string]any); ok {
				if err := validateOpenCodeV33SchemaNode(child, val, path+"."+k); err != nil {
					return err
				}
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s not array", path)
		}
		if n, ok := schemaIntV33(s["minItems"]); ok && int64(len(arr)) < n {
			return fmt.Errorf("%s too short", path)
		}
		if n, ok := schemaIntV33(s["maxItems"]); ok && int64(len(arr)) > n {
			return fmt.Errorf("%s too long", path)
		}
		child, ok := s["items"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s no items", path)
		}
		for n, val := range arr {
			if err := validateOpenCodeV33SchemaNode(child, val, fmt.Sprintf("%s[%d]", path, n)); err != nil {
				return err
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s not string", path)
		}
		if n, ok := schemaIntV33(s["maxLength"]); ok && int64(len([]rune(str))) > n {
			return fmt.Errorf("%s too long", path)
		}
		if enums, ok := s["enum"].([]any); ok {
			found := false
			for _, e := range enums {
				if e == str {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("%s outside enum", path)
			}
		}
	default:
		return fmt.Errorf("%s unsupported type", path)
	}
	return nil
}
func schemaIntV33(v any) (int64, bool) {
	f, ok := v.(float64)
	return int64(f), ok && f >= 0 && f == float64(int64(f))
}
func safeAddV33(v ...int64) (int64, bool) {
	var n int64
	for _, x := range v {
		if x < 0 || x > int64(^uint64(0)>>1)-n {
			return 0, false
		}
		n += x
	}
	return n, true
}
func anyNegativeV33(v ...int64) bool {
	for _, x := range v {
		if x < 0 {
			return true
		}
	}
	return false
}

func validateOpenCodeV33TokenTotal(tokens openCodeV33Tokens) error {
	if tokens.Total == nil || anyNegativeV33(*tokens.Total, tokens.Input, tokens.Output, tokens.Reasoning, tokens.Cache.Read, tokens.Cache.Write) {
		return fmt.Errorf("missing or negative token total")
	}
	total, ok := safeAddV33(tokens.Input, tokens.Output, tokens.Reasoning, tokens.Cache.Read, tokens.Cache.Write)
	if !ok || total != *tokens.Total {
		return fmt.Errorf("token total mismatch")
	}
	return nil
}
func isLoopbackV33(h string) bool { return h == "127.0.0.1" || h == "localhost" || h == "::1" }
func hashOpenCodeV33Capsule(c OpenCodeStructuredV33Capsule) string {
	x := c
	x.RequestBody = nil
	x.InlineConfig = nil
	raw, _ := json.Marshal(x)
	return hashRealisticNextBytes(raw)
}
func decodeOpenCodeStructuredV33Exact(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
