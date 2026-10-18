package workhistory

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	outputredact "github.com/multica-ai/multica/server/pkg/redact"
)

const (
	ProviderCodex  = "codex"
	ProviderClaude = "claude"

	EvidenceClassConversationRecollection = "conversation_recollection"
	RedactionVersion                      = 1
	ImporterVersion                       = 1

	defaultMaxBytes      = int64(8 << 20)
	defaultMaxFragments  = 1000
	defaultMaxLineBytes  = 16 << 20
	defaultMaxTextBytes  = 64 << 10
	semanticChunkBytes   = 1800
	semanticChunkOverlap = 160
	checkpointWindowSize = int64(4 << 10)
)

type Fragment struct {
	Provider         string    `json:"provider"`
	SessionID        string    `json:"session_id"`
	Project          string    `json:"project"`
	OccurredAt       time.Time `json:"occurred_at"`
	Role             string    `json:"role"`
	Text             string    `json:"text"`
	TextSHA256       string    `json:"text_sha256"`
	SourceReference  string    `json:"source_reference"`
	SourceOrdinal    int64     `json:"source_ordinal"`
	SourceOffset     int64     `json:"source_offset"`
	EvidenceClass    string    `json:"evidence_class"`
	RedactionVersion int       `json:"redaction_version"`
	ImporterVersion  int       `json:"importer_version"`
}

// Checkpoint is local importer state. CanonicalPath is deliberately not copied
// into Fragment so durable search results do not expose a workstation path.
type Checkpoint struct {
	Version        int    `json:"version"`
	Provider       string `json:"provider"`
	Project        string `json:"project"`
	CanonicalPath  string `json:"canonical_path"`
	SessionID      string `json:"session_id,omitempty"`
	Offset         int64  `json:"offset"`
	LineNumber     int64  `json:"line_number"`
	PrefixSHA256   string `json:"prefix_sha256,omitempty"`
	BoundaryStart  int64  `json:"boundary_start"`
	BoundarySHA256 string `json:"boundary_sha256,omitempty"`
}

type ScanOptions struct {
	Provider     string
	Project      string
	AllowedRoot  string
	Path         string
	Checkpoint   *Checkpoint
	MaxBytes     int64
	MaxFragments int
	MaxLineBytes int
	MaxTextBytes int
}

type ScanResult struct {
	Fragments      []Fragment `json:"fragments"`
	Checkpoint     Checkpoint `json:"checkpoint"`
	BytesRead      int64      `json:"bytes_read"`
	IgnoredRecords int        `json:"ignored_records"`
	Redactions     int        `json:"redactions"`
	Complete       bool       `json:"complete"`
}

// SourceLocation is the non-secret portion of a work-history source reference.
// The workstation path remains only in the local checkpoint and is resolved by
// the CLI on the machine that imported the fragment.
type SourceLocation struct {
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
	Ordinal   int64  `json:"source_ordinal"`
	Offset    int64  `json:"source_offset"`
	Part      int    `json:"part"`
}

var (
	errSourceChanged = errors.New("work history source changed before the saved checkpoint")

	privateKeyPattern    = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	authorizationPattern = regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*(?:bearer\s+)?[A-Za-z0-9._~+/=-]{8,}`)
	bearerPattern        = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	jwtPattern           = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	providerKeyPattern   = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9]{20,})\b`)
	assignmentPattern    = regexp.MustCompile(`(?i)\b(password|passwd|pwd|api[_-]?key|access[_-]?token|refresh[_-]?token|secret)\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;]+)`)
	emailPattern         = regexp.MustCompile(`\b[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+\b`)
	phonePattern         = regexp.MustCompile(`\b(?:\+?1[-. ]?)?\(?[2-9][0-9]{2}\)?[-. ][0-9]{3}[-. ][0-9]{4}\b`)
	unixHomePattern      = regexp.MustCompile(`/Users/[^/\s]+/`)
	windowsHomePattern   = regexp.MustCompile(`(?i)\b[A-Z]:\\Users\\[^\\\s]+\\`)
)

var internalUserBlocks = []string{
	"codex_internal_context",
	"external_codex_apps_open_page",
	"in-app-browser-context",
	"app-context",
	"environment_context",
	"system-reminder",
	"task-notification",
	"context_guidance",
	"tool-result",
}

func ScanFile(options ScanOptions) (ScanResult, error) {
	var result ScanResult
	options = withDefaults(options)
	if options.Provider != ProviderCodex && options.Provider != ProviderClaude {
		return result, fmt.Errorf("unsupported work history provider %q", options.Provider)
	}
	if strings.TrimSpace(options.Project) == "" {
		return result, errors.New("work history project is required")
	}
	canonicalRoot, canonicalPath, err := resolveAllowedPath(options.AllowedRoot, options.Path)
	if err != nil {
		return result, err
	}
	_ = canonicalRoot

	file, err := os.Open(canonicalPath)
	if err != nil {
		return result, fmt.Errorf("open work history source: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return result, fmt.Errorf("inspect work history source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return result, errors.New("work history source must be a regular file")
	}

	prefixHash, err := firstLineHash(file, options.MaxLineBytes)
	if err != nil {
		return result, err
	}
	checkpoint := Checkpoint{
		Version:       ImporterVersion,
		Provider:      options.Provider,
		Project:       options.Project,
		CanonicalPath: canonicalPath,
		PrefixSHA256:  prefixHash,
	}
	if options.Checkpoint != nil {
		checkpoint = *options.Checkpoint
		if checkpoint.Version != ImporterVersion || checkpoint.Provider != options.Provider || checkpoint.Project != options.Project || checkpoint.CanonicalPath != canonicalPath {
			return result, errors.New("work history checkpoint does not match this source")
		}
		if checkpoint.Offset < 0 || checkpoint.LineNumber < 0 || checkpoint.Offset > info.Size() {
			return result, errSourceChanged
		}
		if checkpoint.PrefixSHA256 != "" && checkpoint.PrefixSHA256 != prefixHash {
			return result, errSourceChanged
		}
		if err := verifyBoundary(file, checkpoint); err != nil {
			return result, err
		}
	}
	if _, err := file.Seek(checkpoint.Offset, io.SeekStart); err != nil {
		return result, fmt.Errorf("seek work history source: %w", err)
	}

	reader := bufio.NewReaderSize(file, 64<<10)
	currentOffset := checkpoint.Offset
	lineNumber := checkpoint.LineNumber
	sessionID := checkpoint.SessionID
	for {
		lineStart := currentOffset
		line, readErr := readBoundedLine(reader, options.MaxLineBytes)
		if errors.Is(readErr, io.EOF) && len(line) == 0 {
			result.Complete = true
			break
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return result, fmt.Errorf("read work history source: %w", readErr)
		}
		if !bytes.HasSuffix(line, []byte{'\n'}) {
			// A writer may still be appending this JSON record. Leave the
			// checkpoint before it so the next scan reads the whole line.
			result.Complete = false
			break
		}
		if result.BytesRead+int64(len(line)) > options.MaxBytes {
			if result.BytesRead == 0 {
				return result, fmt.Errorf("one work history record exceeds the %d byte scan limit", options.MaxBytes)
			}
			result.Complete = false
			break
		}
		fragments, discoveredSession, supported, redactions, err := parseLine(options.Provider, bytes.TrimSpace(line), options.Project, sessionID, lineNumber+1, lineStart, options.MaxTextBytes)
		if err != nil {
			return result, fmt.Errorf("parse %s work history line %d: %w", options.Provider, lineNumber+1, err)
		}
		if discoveredSession != "" {
			if sessionID != "" && sessionID != discoveredSession {
				return result, errors.New("work history source contains multiple session IDs")
			}
			sessionID = discoveredSession
		}
		if len(fragments) > options.MaxFragments {
			return result, errors.New("one work history record exceeds the fragment limit")
		}
		if len(result.Fragments) > 0 && len(result.Fragments)+len(fragments) > options.MaxFragments {
			result.Complete = false
			break
		}
		currentOffset += int64(len(line))
		result.BytesRead += int64(len(line))
		lineNumber++
		if !supported {
			result.IgnoredRecords++
		} else {
			if sessionID == "" {
				return result, errors.New("visible work history record appeared before a session ID")
			}
			for index := range fragments {
				fragments[index].SessionID = sessionID
				fragments[index].SourceReference = sourceReference(options.Provider, sessionID, fragments[index].SourceOrdinal, fragments[index].SourceOffset, index)
			}
			result.Fragments = append(result.Fragments, fragments...)
			result.Redactions += redactions
			if len(result.Fragments) >= options.MaxFragments {
				result.Complete = false
				break
			}
		}
		if errors.Is(readErr, io.EOF) {
			result.Complete = true
			break
		}
	}

	checkpoint.SessionID = sessionID
	checkpoint.Offset = currentOffset
	checkpoint.LineNumber = lineNumber
	checkpoint.PrefixSHA256 = prefixHash
	checkpoint.BoundaryStart, checkpoint.BoundarySHA256, err = boundaryHash(file, currentOffset)
	if err != nil {
		return result, err
	}
	result.Checkpoint = checkpoint
	return result, nil
}

func withDefaults(options ScanOptions) ScanOptions {
	if options.MaxBytes <= 0 {
		options.MaxBytes = defaultMaxBytes
	}
	if options.MaxFragments <= 0 {
		options.MaxFragments = defaultMaxFragments
	}
	if options.MaxLineBytes <= 0 {
		options.MaxLineBytes = defaultMaxLineBytes
	}
	if options.MaxTextBytes <= 0 {
		options.MaxTextBytes = defaultMaxTextBytes
	}
	return options
}

func resolveAllowedPath(root, path string) (string, string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(path) == "" {
		return "", "", errors.New("work history root and source path are required")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve work history root: %w", err)
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve work history root: %w", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve work history source: %w", err)
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve work history source: %w", err)
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("work history source is outside the allowed root")
	}
	return canonicalRoot, canonicalPath, nil
}

func readBoundedLine(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > maxBytes {
			return nil, fmt.Errorf("work history record exceeds %d bytes", maxBytes)
		}
		line = append(line, part...)
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, err
		}
	}
}

func firstLineHash(file *os.File, maxBytes int) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek work history source prefix: %w", err)
	}
	line, err := readBoundedLine(bufio.NewReaderSize(file, 64<<10), maxBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read work history source prefix: %w", err)
	}
	if len(line) == 0 || !bytes.HasSuffix(line, []byte{'\n'}) {
		return "", nil
	}
	sum := sha256.Sum256(line)
	return hex.EncodeToString(sum[:]), nil
}

func boundaryHash(file *os.File, offset int64) (int64, string, error) {
	if offset == 0 {
		return 0, "", nil
	}
	start := offset - checkpointWindowSize
	if start < 0 {
		start = 0
	}
	window := make([]byte, offset-start)
	if _, err := file.ReadAt(window, start); err != nil && !errors.Is(err, io.EOF) {
		return 0, "", fmt.Errorf("read work history checkpoint boundary: %w", err)
	}
	sum := sha256.Sum256(window)
	return start, hex.EncodeToString(sum[:]), nil
}

func verifyBoundary(file *os.File, checkpoint Checkpoint) error {
	if checkpoint.Offset == 0 {
		return nil
	}
	start, hash, err := boundaryHash(file, checkpoint.Offset)
	if err != nil {
		return err
	}
	if start != checkpoint.BoundaryStart || hash != checkpoint.BoundarySHA256 {
		return errSourceChanged
	}
	return nil
}

type codexEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Ordinal   int64           `json:"ordinal"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type messagePayload struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
}

type claudeEnvelope struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	SessionID string         `json:"sessionId"`
	Message   messagePayload `json:"message"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func parseLine(provider string, line []byte, project, knownSession string, lineNumber, offset int64, maxTextBytes int) ([]Fragment, string, bool, int, error) {
	if len(line) == 0 {
		return nil, "", false, 0, nil
	}
	if provider == ProviderCodex {
		return parseCodexLine(line, project, knownSession, lineNumber, offset, maxTextBytes)
	}
	return parseClaudeLine(line, project, knownSession, lineNumber, offset, maxTextBytes)
}

func parseCodexLine(line []byte, project, knownSession string, lineNumber, offset int64, maxTextBytes int) ([]Fragment, string, bool, int, error) {
	var envelope codexEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, "", false, 0, err
	}
	if envelope.Type == "session_meta" {
		var meta messagePayload
		if err := json.Unmarshal(envelope.Payload, &meta); err != nil {
			return nil, "", false, 0, err
		}
		return nil, firstNonEmpty(meta.ID, meta.SessionID), false, 0, nil
	}
	if envelope.Type != "response_item" {
		return nil, "", false, 0, nil
	}
	var message messagePayload
	if err := json.Unmarshal(envelope.Payload, &message); err != nil {
		return nil, "", false, 0, err
	}
	if message.Type != "message" || (message.Role != "user" && message.Role != "assistant") {
		return nil, "", false, 0, nil
	}
	allowedBlock := "input_text"
	if message.Role == "assistant" {
		allowedBlock = "output_text"
	}
	text, err := visibleContent(message.Content, allowedBlock, false)
	if err != nil {
		return nil, "", false, 0, err
	}
	if message.Role == "user" {
		text = visibleUserText(text)
	}
	ordinal := envelope.Ordinal
	if ordinal <= 0 {
		ordinal = lineNumber
	}
	fragments, redactions, err := newFragments(ProviderCodex, knownSession, project, envelope.Timestamp, message.Role, text, ordinal, offset, maxTextBytes)
	return fragments, "", len(fragments) != 0, redactions, err
}

func parseClaudeLine(line []byte, project, knownSession string, lineNumber, offset int64, maxTextBytes int) ([]Fragment, string, bool, int, error) {
	var envelope claudeEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, "", false, 0, err
	}
	discoveredSession := strings.TrimSpace(envelope.SessionID)
	if envelope.Type != "user" && envelope.Type != "assistant" {
		return nil, discoveredSession, false, 0, nil
	}
	role := envelope.Type
	text, err := visibleContent(envelope.Message.Content, "text", role == "user")
	if err != nil {
		return nil, "", false, 0, err
	}
	if role == "user" {
		text = visibleUserText(text)
	}
	sessionID := firstNonEmpty(knownSession, discoveredSession)
	fragments, redactions, err := newFragments(ProviderClaude, sessionID, project, envelope.Timestamp, role, text, lineNumber, offset, maxTextBytes)
	return fragments, discoveredSession, len(fragments) != 0, redactions, err
}

func visibleContent(raw json.RawMessage, allowedBlock string, allowString bool) (string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	if allowString && raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", err
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == allowedBlock && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// visibleUserText removes application-generated context that some providers
// serialize as a user-role message. Those wrappers are useful to the live
// agent but are not user-authored conversation and must not enter durable
// company history. Text outside a known wrapper is preserved.
func visibleUserText(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "# AGENTS.md instructions for ") && strings.Contains(trimmed, "<environment_context>") {
		return ""
	}
	if strings.HasPrefix(trimmed, "You are running as a local coding agent for a Multica workspace.") &&
		strings.Contains(trimmed, "Your assigned issue ID is:") &&
		strings.Contains(trimmed, "## Session Continuity Notice") {
		return ""
	}
	for _, tag := range internalUserBlocks {
		trimmed = stripInternalBlock(trimmed, tag)
	}
	trimmed = strings.TrimSpace(trimmed)
	for _, heading := range []string{"# Files mentioned by the user:", "# Files pasted by the user:"} {
		if strings.HasPrefix(trimmed, heading) {
			const requestMarker = "## My request:"
			if marker := strings.LastIndex(trimmed, requestMarker); marker >= 0 {
				trimmed = strings.TrimSpace(trimmed[marker+len(requestMarker):])
			} else {
				return ""
			}
			break
		}
	}
	return trimmed
}

func stripInternalBlock(text, tag string) string {
	openPrefix := "<" + tag
	closeTag := "</" + tag + ">"
	for {
		start := internalBlockStart(text, openPrefix)
		if start < 0 {
			return text
		}
		openEndRelative := strings.IndexByte(text[start:], '>')
		if openEndRelative < 0 {
			// Fail closed for a malformed reserved wrapper.
			return text[:start]
		}
		openEnd := start + openEndRelative + 1
		closeRelative := strings.Index(text[openEnd:], closeTag)
		if closeRelative < 0 {
			// Fail closed for a malformed reserved wrapper.
			return text[:start]
		}
		end := openEnd + closeRelative + len(closeTag)
		text = text[:start] + text[end:]
	}
}

func internalBlockStart(text, openPrefix string) int {
	searchFrom := 0
	for {
		relative := strings.Index(text[searchFrom:], openPrefix)
		if relative < 0 {
			return -1
		}
		start := searchFrom + relative
		next := start + len(openPrefix)
		if next == len(text) || text[next] == '>' || text[next] == ' ' || text[next] == '\t' || text[next] == '\r' || text[next] == '\n' {
			return start
		}
		searchFrom = next
	}
}

func newFragments(provider, sessionID, project, timestamp, role, text string, ordinal, offset int64, maxTextBytes int) ([]Fragment, int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, 0, nil
	}
	redacted, redactions := redact(text)
	redacted = truncateUTF8(redacted, maxTextBytes)
	if strings.TrimSpace(redacted) == "" {
		return nil, redactions, nil
	}
	when, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return nil, redactions, errors.New("visible message has an invalid timestamp")
	}
	chunks := chunkUTF8(redacted, semanticChunkBytes, semanticChunkOverlap)
	fragments := make([]Fragment, 0, len(chunks))
	for _, chunk := range chunks {
		sum := sha256.Sum256([]byte(chunk))
		fragments = append(fragments, Fragment{
			Provider:         provider,
			SessionID:        sessionID,
			Project:          project,
			OccurredAt:       when.UTC(),
			Role:             role,
			Text:             chunk,
			TextSHA256:       hex.EncodeToString(sum[:]),
			SourceOrdinal:    ordinal,
			SourceOffset:     offset,
			EvidenceClass:    EvidenceClassConversationRecollection,
			RedactionVersion: RedactionVersion,
			ImporterVersion:  ImporterVersion,
		})
	}
	return fragments, redactions, nil
}

func chunkUTF8(text string, maximum, overlap int) []string {
	if len(text) <= maximum {
		return []string{text}
	}
	var chunks []string
	for start := 0; start < len(text); {
		end := start + maximum
		if end >= len(text) {
			end = len(text)
		} else {
			for end > start && (text[end]&0xc0) == 0x80 {
				end--
			}
			searchStart := end - 300
			if searchStart < start {
				searchStart = start
			}
			if boundary := strings.LastIndexAny(text[searchStart:end], " \t\r\n"); boundary >= 0 {
				end = searchStart + boundary
			}
		}
		chunk := strings.TrimSpace(text[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(text) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		for next < len(text) && (text[next]&0xc0) == 0x80 {
			next++
		}
		start = next
	}
	return chunks
}

func redact(text string) (string, int) {
	type replacement struct {
		pattern *regexp.Regexp
		value   string
	}
	replacements := []replacement{
		{privateKeyPattern, "[REDACTED:PRIVATE_KEY]"},
		{authorizationPattern, "Authorization: [REDACTED:TOKEN]"},
		{bearerPattern, "Bearer [REDACTED:TOKEN]"},
		{jwtPattern, "[REDACTED:JWT]"},
		{providerKeyPattern, "[REDACTED:API_KEY]"},
		{emailPattern, "[REDACTED:EMAIL]"},
		{phonePattern, "[REDACTED:PHONE]"},
		{unixHomePattern, "/Users/[REDACTED:USER]/"},
		{windowsHomePattern, `C:\Users\[REDACTED:USER]\`},
	}
	count := 0
	for _, item := range replacements {
		matches := item.pattern.FindAllStringIndex(text, -1)
		count += len(matches)
		text = item.pattern.ReplaceAllString(text, item.value)
	}
	matches := assignmentPattern.FindAllStringSubmatchIndex(text, -1)
	count += len(matches)
	text = assignmentPattern.ReplaceAllString(text, "${1}=[REDACTED:SECRET]")
	if broadlyRedacted := outputredact.Text(text); broadlyRedacted != text {
		count++
		text = broadlyRedacted
	}
	return text, count
}

func truncateUTF8(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	const suffix = "\n[TRUNCATED]"
	limit := maxBytes - len(suffix)
	if limit <= 0 {
		return suffix[:maxBytes]
	}
	for limit > 0 && (text[limit]&0xc0) == 0x80 {
		limit--
	}
	return text[:limit] + suffix
}

func sourceReference(provider, sessionID string, ordinal, offset int64, part int) string {
	return fmt.Sprintf("work-history://%s/session/%s/record/%d?offset=%d&part=%d", provider, url.PathEscape(sessionID), ordinal, offset, part)
}

// ParseSourceReference validates a durable reference before the local CLI uses
// its private checkpoint to map it back to the original session file.
func ParseSourceReference(reference string) (SourceLocation, error) {
	var location SourceLocation
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme != "work-history" || parsed.User != nil || parsed.Fragment != "" {
		return location, errors.New("invalid work history source reference")
	}
	provider := parsed.Host
	if provider != ProviderCodex && provider != ProviderClaude {
		return location, errors.New("invalid work history source provider")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "session" || parts[1] == "" || parts[2] != "record" {
		return location, errors.New("invalid work history source path")
	}
	ordinal, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || ordinal <= 0 {
		return location, errors.New("invalid work history source ordinal")
	}
	query := parsed.Query()
	if len(query) != 2 || len(query["offset"]) != 1 || len(query["part"]) != 1 {
		return location, errors.New("invalid work history source query")
	}
	offset, err := strconv.ParseInt(query.Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		return location, errors.New("invalid work history source offset")
	}
	part, err := strconv.Atoi(query.Get("part"))
	if err != nil || part < 0 {
		return location, errors.New("invalid work history source part")
	}
	location = SourceLocation{Provider: provider, SessionID: parts[1], Ordinal: ordinal, Offset: offset, Part: part}
	return location, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
