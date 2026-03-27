package aiclients

import (
	"context"
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
	APIKey  string
	BaseURL string
}

type OpenAIClient struct {
	client openai.Client
	logger *slog.Logger
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
		client: openai.NewClient(options...),
		logger: logger,
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

	response, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: input},
	})
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
