package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

// UnmarshalJSON keeps the broad historical Task wire forward-compatible while
// making the authority-bearing guarded_execution field strict. Unknown outer
// task fields remain tolerated; unknown, duplicate, null, malformed, or
// trailing guarded packages fail the claim decode.
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskAlias Task
	var decoded taskAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	raw, present, err := extractGuardedExecutionField(data)
	if err != nil {
		return err
	}
	if !present {
		decoded.GuardedExecution = nil
		*t = Task(decoded)
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: guarded execution package cannot be null", agentqualification.ErrGuardedDispatchInvalid)
	}
	pkg, err := agentqualification.DecodeGuardedExecutionPackageV1(raw)
	if err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("%w: guarded execution package is empty", agentqualification.ErrGuardedDispatchInvalid)
	}
	decoded.GuardedExecution = pkg
	*t = Task(decoded)
	return nil
}

func extractGuardedExecutionField(data []byte) (json.RawMessage, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil {
		return nil, false, err
	}
	if delim, ok := first.(json.Delim); !ok || delim != '{' {
		return nil, false, fmt.Errorf("task payload must be an object")
	}
	var guarded json.RawMessage
	present := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, false, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, false, fmt.Errorf("task payload key is invalid")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false, err
		}
		if key != "guarded_execution" {
			continue
		}
		if present {
			return nil, false, fmt.Errorf("%w: duplicate guarded execution package", agentqualification.ErrGuardedDispatchInvalid)
		}
		present = true
		guarded = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false, fmt.Errorf("task payload has trailing data")
	}
	return guarded, present, nil
}
