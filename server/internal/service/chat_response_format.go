package service

import "encoding/json"

// Response format is a per-turn presentation preference, not execution authority.
func ValidChatResponseFormat(format string) bool {
	switch format {
	case "", "plan", "compare", "checklist", "diagram":
		return true
	default:
		return false
	}
}

func ChatResponseFormatFromContext(raw []byte) string {
	var saved struct {
		Format string `json:"chat_response_format"`
	}
	if json.Unmarshal(raw, &saved) != nil || !ValidChatResponseFormat(saved.Format) {
		return ""
	}
	return saved.Format
}

func ChatResponseFormatInstruction(format string) string {
	var instruction string
	switch format {
	case "plan":
		instruction = "Draft a plan with steps, dependencies and open questions. Discuss it before starting work."
	case "compare":
		instruction = "Compare the options in a concise table, including tradeoffs, missing information and your recommendation."
	case "checklist":
		instruction = "Present a practical checklist. Separate completed items supported by evidence from proposed next steps."
	case "diagram":
		instruction = "Explain visually using a simple Mermaid diagram and a short plain-language explanation."
	default:
		return ""
	}
	return "\n\nResponse format selected for this reply only: " + instruction + " This selection grants no approval or permission to execute work."
}
