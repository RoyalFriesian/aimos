package aiclients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Sarnga/agent-platform/pkg/threads"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

type OpenAIConfig struct {
	APIKey          string
	BaseURL         string
	MaxOutputTokens int64 // 0 = provider default
}

type OpenAIClient struct {
	client          openai.Client
	logger          *slog.Logger
	maxOutputTokens int64
}

func NewOpenAIClient(config OpenAIConfig, logger *slog.Logger) *OpenAIClient {
	options := []option.RequestOption{}
	if config.APIKey != "" {
		options = append(options, option.WithAPIKey(config.APIKey))
	}
	if config.BaseURL != "" {
		options = append(options, option.WithBaseURL(config.BaseURL))
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &OpenAIClient{
		client:          openai.NewClient(options...),
		logger:          logger,
		maxOutputTokens: config.MaxOutputTokens,
	}
}

func (c *OpenAIClient) Generate(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	messages := []threads.Message{
		{Role: threads.RoleSystem, Content: systemPrompt},
		{Role: threads.RoleUser, Content: userPrompt},
	}
	return c.GenerateFromMessages(ctx, model, messages)
}

func (c *OpenAIClient) GenerateFromMessages(ctx context.Context, model string, messages []threads.Message) (string, error) {
	input := make(responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" && len(msg.ImageDataURLs) == 0 {
			continue
		}

		if len(msg.ImageDataURLs) > 0 {
			// Build multimodal content parts (text + images).
			parts := buildMultimodalParts(content, msg.ImageDataURLs)
			input = append(input, responses.ResponseInputItemParamOfMessage(parts, roleToOpenAI(msg.Role)))
		} else {
			input = append(input, responses.ResponseInputItemParamOfMessage(content, roleToOpenAI(msg.Role)))
		}
	}
	if len(input) == 0 {
		return "", errors.New("no non-empty messages provided")
	}

	params := responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: input},
	}
	if c.maxOutputTokens > 0 {
		params.MaxOutputTokens = param.NewOpt(c.maxOutputTokens)
	}

	response, err := c.client.Responses.New(ctx, params)
	if err != nil {
		wrapped := fmt.Errorf("create response: %w", err)
		c.logger.Error("openai request failed", "error", wrapped, "model", model)
		return "", wrapped
	}

	content := strings.TrimSpace(response.OutputText())
	if content == "" {
		return "", errors.New("empty response content returned from OpenAI")
	}
	return content, nil
}

// buildMultimodalParts constructs a content list with a text part followed by image parts.
func buildMultimodalParts(text string, imageDataURLs []string) responses.ResponseInputMessageContentListParam {
	parts := make(responses.ResponseInputMessageContentListParam, 0, 1+len(imageDataURLs))
	if text != "" {
		parts = append(parts, responses.ResponseInputContentUnionParam{
			OfInputText: &responses.ResponseInputTextParam{
				Text: text,
			},
		})
	}
	for _, dataURL := range imageDataURLs {
		parts = append(parts, responses.ResponseInputContentUnionParam{
			OfInputImage: &responses.ResponseInputImageParam{
				ImageURL: param.NewOpt(dataURL),
				Detail:   responses.ResponseInputImageDetailAuto,
			},
		})
	}
	return parts
}

func roleToOpenAI(role threads.Role) responses.EasyInputMessageRole {
	switch role {
	case threads.RoleSystem:
		return responses.EasyInputMessageRoleSystem
	case threads.RoleAssistant:
		return responses.EasyInputMessageRoleAssistant
	default:
		return responses.EasyInputMessageRoleUser
	}
}

// ToolExecutor is a callback that executes a tool call by name and returns
// the result string to feed back to the model.
type ToolExecutor func(ctx context.Context, name string, args json.RawMessage) (string, error)

// ToolCallRecord captures one executed tool call for the caller.
type ToolCallRecord struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

// ToolCallResult is the final output of GenerateWithTools, containing the
// model's text response plus every tool call that was executed.
type ToolCallResult struct {
	Text      string           `json:"text"`
	ToolCalls []ToolCallRecord `json:"toolCalls,omitempty"`
}

// GenerateWithTools runs an OpenAI Responses API tool-calling loop.
// It sends the initial prompt with the given tools, lets the model call
// tools, feeds results back, and repeats until the model emits a text
// response or maxRounds is reached.
func (c *OpenAIClient) GenerateWithTools(
	ctx context.Context,
	model string,
	systemPrompt string,
	userPrompt string,
	tools []responses.ToolUnionParam,
	executor ToolExecutor,
	maxRounds int,
) (ToolCallResult, error) {
	if maxRounds <= 0 {
		maxRounds = 10
	}

	// Build initial input.
	input := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(systemPrompt, responses.EasyInputMessageRoleSystem),
		responses.ResponseInputItemParamOfMessage(userPrompt, responses.EasyInputMessageRoleUser),
	}

	params := responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: input},
		Tools: tools,
	}
	if c.maxOutputTokens > 0 {
		params.MaxOutputTokens = param.NewOpt(c.maxOutputTokens)
	}

	var allToolCalls []ToolCallRecord

	for round := 0; round < maxRounds; round++ {
		response, err := c.client.Responses.New(ctx, params)
		if err != nil {
			return ToolCallResult{}, fmt.Errorf("tool-calling round %d: %w", round, err)
		}

		// Collect function calls from the response output.
		var functionCalls []responses.ResponseFunctionToolCall
		for _, item := range response.Output {
			if item.Type == "function_call" {
				functionCalls = append(functionCalls, item.AsFunctionCall())
			}
		}

		// If no function calls, the model is done — return text.
		if len(functionCalls) == 0 {
			text := strings.TrimSpace(response.OutputText())
			return ToolCallResult{Text: text, ToolCalls: allToolCalls}, nil
		}

		// Execute each tool call and build the next input.
		// Start the next round's input with all previous output items
		// plus the function call outputs.
		for _, item := range response.Output {
			input = append(input, responseOutputToInputParam(item))
		}

		for _, fc := range functionCalls {
			c.logger.Debug("executing tool call", "name", fc.Name, "callID", fc.CallID, "round", round)

			result, execErr := executor(ctx, fc.Name, json.RawMessage(fc.Arguments))
			if execErr != nil {
				result = fmt.Sprintf("error: %s", execErr.Error())
			}

			allToolCalls = append(allToolCalls, ToolCallRecord{
				Name:      fc.Name,
				Arguments: fc.Arguments,
				Result:    result,
			})

			// Feed the result back as a function_call_output item.
			input = append(input, responses.ResponseInputItemParamOfFunctionCallOutput(fc.CallID, result))
		}

		// Update params for next round.
		params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}
	}

	// If we exhausted maxRounds, return whatever text we have.
	return ToolCallResult{Text: "", ToolCalls: allToolCalls}, errors.New("tool-calling loop exceeded max rounds")
}

// responseOutputToInputParam converts a response output item back into an
// input item so the conversation history is preserved across rounds.
func responseOutputToInputParam(item responses.ResponseOutputItemUnion) responses.ResponseInputItemUnionParam {
	switch item.Type {
	case "function_call":
		fc := item.AsFunctionCall()
		return responses.ResponseInputItemParamOfFunctionCall(fc.Arguments, fc.CallID, fc.Name)
	default:
		// For message, reasoning, and other output types, use the item ID
		// reference pattern. The API accepts output items as input items.
		return responses.ResponseInputItemUnionParam{
			OfItemReference: &responses.ResponseInputItemItemReferenceParam{
				ID: item.ID,
			},
		}
	}
}
