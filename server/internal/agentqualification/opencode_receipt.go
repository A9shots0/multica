package agentqualification

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"
	"time"
)

type openCodeTokens struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	Reasoning int64 `json:"reasoning"`
	Cache     struct {
		Read  int64 `json:"read"`
		Write int64 `json:"write"`
	} `json:"cache"`
}

type openCodeRunPart struct {
	ID        string          `json:"id"`
	SessionID string          `json:"sessionID"`
	MessageID string          `json:"messageID"`
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Cost      json.Number     `json:"cost"`
	Tokens    openCodeTokens  `json:"tokens"`
	State     json.RawMessage `json:"state"`
	Metadata  json.RawMessage `json:"metadata"`
}

type openCodeRunEvent struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      openCodeRunPart `json:"part"`
}

type openCodeExportMessage struct {
	Info struct {
		ID         string          `json:"id"`
		SessionID  string          `json:"sessionID"`
		Role       string          `json:"role"`
		ModelID    string          `json:"modelID"`
		ProviderID string          `json:"providerID"`
		Summary    bool            `json:"summary"`
		Finish     string          `json:"finish"`
		Cost       json.Number     `json:"cost"`
		Tokens     openCodeTokens  `json:"tokens"`
		Error      json.RawMessage `json:"error"`
	} `json:"info"`
	Parts []openCodeRunPart `json:"parts"`
}

type openCodeSessionExport struct {
	Info struct {
		ID string `json:"id"`
	} `json:"info"`
	Messages []openCodeExportMessage `json:"messages"`
}

// ParseOpenCodeRunResult normalizes a fresh run's JSONL plus its exact session
// export. JSONL alone is intentionally insufficient because it omits the
// provider/model identity. Any tool, error, retry-shaped extra output,
// duplicate terminal part, or disagreement between the stream and export
// invalidates the observation.
func ParseOpenCodeRunResult(rawJSONL, rawExport []byte) (ProviderExecutionResult, error) {
	result, _, err := ParseOpenCodeRunArtifact(rawJSONL, rawExport)
	return result, err
}

// ParseOpenCodeRunArtifact returns the exact candidate JSON separately from
// the provider-neutral receipt fields so blind grading never has to recover
// content from hashes or an untrusted session store.
func ParseOpenCodeRunArtifact(rawJSONL, rawExport []byte) (ProviderExecutionResult, json.RawMessage, error) {
	result, output, err := ParseOpenCodeRunEnvelope(rawJSONL, rawExport)
	if err != nil {
		return ProviderExecutionResult{}, nil, err
	}
	if !json.Valid(output) {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode text result is not valid JSON")
	}
	return result, append(json.RawMessage(nil), output...), nil
}

// ParseOpenCodeRunEnvelope validates transport and usage while preserving the
// raw candidate text. Realistic runs need this boundary so malformed candidate
// JSON is recorded as a candidate failure, not misclassified as transport loss.
func ParseOpenCodeRunEnvelope(rawJSONL, rawExport []byte) (ProviderExecutionResult, []byte, error) {
	return parseOpenCodeRunEnvelope(rawJSONL, rawExport)
}

// parseOpenCodeRunEnvelope validates the provider transport, identity, usage,
// routing, and exact streamed text without claiming that the candidate obeyed
// the requested JSON response contract. The latter is a model outcome, not a
// transport failure, and is closed separately without another provider call.
func parseOpenCodeRunEnvelope(rawJSONL, rawExport []byte) (ProviderExecutionResult, []byte, error) {
	var events []openCodeRunEvent
	scanner := bufio.NewScanner(bytes.NewReader(rawJSONL))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.UseNumber()
		var event openCodeRunEvent
		if err := decoder.Decode(&event); err != nil {
			return ProviderExecutionResult{}, nil, fmt.Errorf("decode OpenCode JSONL event: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode JSONL line has trailing data")
		}
		if !contains([]string{"step_start", "step_finish", "text"}, event.Type) {
			return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode emitted disallowed event %q", event.Type)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return ProviderExecutionResult{}, nil, err
	}
	if len(events) == 0 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode JSONL is empty")
	}

	var startedAt, completedAt int64
	var sessionID, messageID, output string
	var finish *openCodeRunPart
	eventTypes := map[string]struct{}{}
	counts := map[string]int{}
	for _, event := range events {
		if event.Timestamp <= 0 || event.SessionID == "" || event.Part.SessionID != event.SessionID {
			return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode event identity or timestamp is invalid")
		}
		if sessionID == "" {
			sessionID = event.SessionID
		} else if sessionID != event.SessionID {
			return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode JSONL contains multiple sessions")
		}
		if startedAt == 0 || event.Timestamp < startedAt {
			startedAt = event.Timestamp
		}
		if event.Timestamp > completedAt {
			completedAt = event.Timestamp
		}
		counts[event.Type]++
		eventTypes[event.Type] = struct{}{}
		switch event.Type {
		case "step_start":
			if event.Part.Type != "step-start" {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode step_start part mismatch")
			}
			if messageID == "" && event.Part.MessageID != "" {
				messageID = event.Part.MessageID
			} else if event.Part.MessageID != "" && messageID != event.Part.MessageID {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode run parts disagree on message")
			}
		case "step_finish":
			if event.Part.Type != "step-finish" || event.Part.MessageID == "" {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode step_finish part mismatch")
			}
			copy := event.Part
			finish = &copy
			messageID = event.Part.MessageID
		case "text":
			if event.Part.Type != "text" || event.Part.MessageID == "" || strings.TrimSpace(event.Part.Text) == "" {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode text part mismatch")
			}
			output = strings.TrimSpace(event.Part.Text)
			if messageID == "" {
				messageID = event.Part.MessageID
			} else if messageID != event.Part.MessageID {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode terminal parts disagree on message")
			}
		}
	}
	if counts["step_start"] < 1 || counts["step_finish"] != 1 || counts["text"] > 1 || finish == nil {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode run must contain at least one step_start, exactly one step_finish, and at most one text result")
	}
	if finish.Tokens.Input < 0 || finish.Tokens.Output < 0 || finish.Tokens.Reasoning < 0 || finish.Tokens.Output+finish.Tokens.Reasoning <= 0 || finish.Tokens.Cache.Read < 0 || finish.Tokens.Cache.Write < 0 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode token receipt is incomplete or invalid")
	}
	if counts["text"] == 0 && finish.Tokens.Output != 0 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode omitted streamed text despite reporting output tokens")
	}

	decoder := json.NewDecoder(bytes.NewReader(rawExport))
	decoder.UseNumber()
	var exported openCodeSessionExport
	if err := decoder.Decode(&exported); err != nil {
		return ProviderExecutionResult{}, nil, fmt.Errorf("decode OpenCode session export: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode session export has trailing data")
	}
	if exported.Info.ID != sessionID {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export does not match run session")
	}
	var assistant *openCodeExportMessage
	for index := range exported.Messages {
		message := &exported.Messages[index]
		if message.Info.Role != "assistant" || message.Info.Summary {
			continue
		}
		if message.Info.ID == messageID && message.Info.SessionID == sessionID {
			if assistant != nil {
				return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export has duplicate terminal assistant")
			}
			assistant = message
		}
	}
	if assistant == nil || assistant.Info.ProviderID == "" || assistant.Info.ModelID == "" || assistant.Info.Finish == "" || len(bytes.TrimSpace(assistant.Info.Error)) != 0 && string(bytes.TrimSpace(assistant.Info.Error)) != "null" {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export lacks a successful model-identified assistant receipt")
	}
	if assistant.Info.Tokens != finish.Tokens || !sameDecimal(assistant.Info.Cost.String(), finish.Cost.String()) {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode JSONL and export usage disagree")
	}
	matchingText := 0
	exportTextParts := 0
	servingProviders := map[string]struct{}{}
	for _, part := range assistant.Parts {
		if part.Type == "tool" {
			return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export contains a tool part")
		}
		if part.Type == "text" {
			exportTextParts++
			if strings.TrimSpace(part.Text) == output {
				matchingText++
			}
		}
		for _, provider := range openRouterSelectedProviders(part.Metadata) {
			servingProviders[provider] = struct{}{}
		}
	}
	if exportTextParts != counts["text"] || matchingText != counts["text"] {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export does not bind the streamed output")
	}
	if len(servingProviders) > 1 {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode export reports multiple selected serving providers")
	}
	var reportedServingProvider string
	for provider := range servingProviders {
		reportedServingProvider = provider
	}
	costMicros, err := decimalUSDMicros(finish.Cost.String())
	if err != nil {
		return ProviderExecutionResult{}, nil, fmt.Errorf("OpenCode cost invalid: %w", err)
	}
	input := finish.Tokens.Input + finish.Tokens.Cache.Read + finish.Tokens.Cache.Write
	cached, outputTokens, reasoning := finish.Tokens.Cache.Read, finish.Tokens.Output, finish.Tokens.Reasoning
	manifest := make([]string, 0, len(eventTypes))
	for eventType := range eventTypes {
		manifest = append(manifest, eventType)
	}
	sort.Strings(manifest)
	result := ProviderExecutionResult{
		ObservationID: "opencode:" + sessionID + ":" + messageID, ReportedModel: assistant.Info.ProviderID + "/" + assistant.Info.ModelID,
		ReportedServingProvider:    reportedServingProvider,
		ReportedModelVersionStatus: "unavailable_truthful", ProviderResponseID: sessionID + ":" + messageID,
		StartedAt: time.UnixMilli(startedAt).UTC().Format(time.RFC3339Nano), CompletedAt: time.UnixMilli(completedAt).UTC().Format(time.RFC3339Nano),
		TerminalState: "completed", ObservedProviderEventTypes: manifest, RawTraceSHA256: sha256Hex(append(append([]byte(nil), rawJSONL...), rawExport...)), OutputSHA256: sha256Hex([]byte(output)),
		Usage: ExecutionReceiptUsage{InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &outputTokens, ReasoningTokens: &reasoning, ReasoningTokensStatus: "reported", ProviderReportedCostMicros: &costMicros},
	}
	return result, append([]byte(nil), []byte(output)...), nil
}

// openRouterSelectedProviders walks only JSON object/array structure and
// extracts endpoints explicitly marked selected by OpenRouter metadata. It
// never guesses the provider from the requested route.
func openRouterSelectedProviders(raw json.RawMessage) []string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if selected, _ := typed["selected"].(bool); selected {
				if provider, _ := typed["provider"].(string); strings.TrimSpace(provider) != "" {
					seen[strings.TrimSpace(provider)] = struct{}{}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	providers := make([]string, 0, len(seen))
	for provider := range seen {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}

func sameDecimal(left, right string) bool {
	a, aOK := new(big.Rat).SetString(left)
	b, bOK := new(big.Rat).SetString(right)
	return aOK && bOK && a.Cmp(b) == 0
}
