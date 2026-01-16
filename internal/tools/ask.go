package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// QuestionOption represents an option for a question
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question represents a question to ask the user
type Question struct {
	Question    string           `json:"question"`
	Header      string           `json:"header"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool             `json:"multiSelect"`
}

// UserInputHandler is a function that gets input from the user
type UserInputHandler func(questions []Question) (map[string]string, error)

// AskUserQuestionTool asks questions to the user
type AskUserQuestionTool struct {
	inputHandler UserInputHandler
}

// NewAskUserQuestionTool creates a new AskUserQuestion tool
func NewAskUserQuestionTool(handler UserInputHandler) *AskUserQuestionTool {
	return &AskUserQuestionTool{inputHandler: handler}
}

func (t *AskUserQuestionTool) Name() string {
	return "AskUserQuestion"
}

func (t *AskUserQuestionTool) Description() string {
	return `Use this tool when you need to ask the user questions during execution.

This allows you to:
1. Gather user preferences or requirements
2. Clarify ambiguous instructions
3. Get decisions on implementation choices as you work
4. Offer choices to the user about what direction to take`
}

func (t *AskUserQuestionTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type":        "array",
				"description": "Questions to ask the user (1-4 questions)",
				"minItems":    1,
				"maxItems":    4,
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"question": map[string]interface{}{
							"type":        "string",
							"description": "The complete question to ask the user",
						},
						"header": map[string]interface{}{
							"type":        "string",
							"description": "Very short label (max 12 chars)",
						},
						"options": map[string]interface{}{
							"type":     "array",
							"minItems": 2,
							"maxItems": 4,
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"label": map[string]interface{}{
										"type":        "string",
										"description": "The display text for this option",
									},
									"description": map[string]interface{}{
										"type":        "string",
										"description": "Explanation of what this option means",
									},
								},
								"required": []string{"label", "description"},
							},
						},
						"multiSelect": map[string]interface{}{
							"type":        "boolean",
							"default":     false,
							"description": "Allow multiple selections",
						},
					},
					"required": []string{"question", "header", "options", "multiSelect"},
				},
			},
		},
		"required": []string{"questions"},
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, params map[string]interface{}) (*Result, error) {
	questionsRaw, ok := params["questions"]
	if !ok {
		return NewErrorResultString("questions parameter is required"), nil
	}

	// Convert to JSON and back to parse the questions
	questionsJSON, err := json.Marshal(questionsRaw)
	if err != nil {
		return NewErrorResult(err), nil
	}

	var questions []Question
	if err := json.Unmarshal(questionsJSON, &questions); err != nil {
		return NewErrorResultString(fmt.Sprintf("Invalid questions format: %s", err.Error())), nil
	}

	if len(questions) == 0 {
		return NewErrorResultString("At least one question is required"), nil
	}

	if len(questions) > 4 {
		return NewErrorResultString("Maximum 4 questions allowed"), nil
	}

	// Validate questions
	for i, q := range questions {
		if q.Question == "" {
			return NewErrorResultString(fmt.Sprintf("Question %d: question text is required", i+1)), nil
		}
		if len(q.Options) < 2 {
			return NewErrorResultString(fmt.Sprintf("Question %d: at least 2 options are required", i+1)), nil
		}
		if len(q.Options) > 4 {
			return NewErrorResultString(fmt.Sprintf("Question %d: maximum 4 options allowed", i+1)), nil
		}
	}

	// Call the input handler
	if t.inputHandler == nil {
		return NewErrorResultString("No input handler configured"), nil
	}

	answers, err := t.inputHandler(questions)
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Build response
	var output strings.Builder
	output.WriteString("User answers:\n")
	for header, answer := range answers {
		output.WriteString(fmt.Sprintf("- %s: %s\n", header, answer))
	}

	return NewResult(output.String()), nil
}

// SetInputHandler sets the input handler
func (t *AskUserQuestionTool) SetInputHandler(handler UserInputHandler) {
	t.inputHandler = handler
}
