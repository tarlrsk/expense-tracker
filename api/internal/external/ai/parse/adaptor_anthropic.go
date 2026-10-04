package parse

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

const (
	// maxTokens bounds the answer; a note of a few dozen items fits well inside it.
	maxTokens = 4096
	// maxRetries is the SDK's retries of a connection error, 408, 409, 429 or 5xx. All of them
	// fit inside the call's time limit, which cuts them short.
	maxRetries = 2
)

// anthropicAdaptor calls the Anthropic Messages API (github.com/anthropics/anthropic-sdk-go).
type anthropicAdaptor struct {
	// client is the zero Client when no key is set; it is then never used.
	client     anthropic.Client
	configured bool
	model      string
	timeout    time.Duration
}

// NewAnthropic returns the Anthropic adaptor for cfg (ADR-0009). Without a key it still succeeds:
// Parse then returns ErrNotConfigured without any network call. The SDK reads nothing from the
// environment or from profile files: cfg is the only source of the key and the address.
func NewAnthropic(cfg config.AI) (Port, error) {
	return newAnthropic(cfg)
}

// newAnthropic is NewAnthropic with extra client options; tests point the client at their own
// server and shorten the retries.
func newAnthropic(cfg config.AI, opts ...option.RequestOption) (Port, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("anthropic: the model is empty")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("anthropic: the time limit must be greater than 0")
	}
	a := &anthropicAdaptor{model: cfg.Model, timeout: cfg.Timeout}
	if cfg.APIKey == "" {
		return a, nil
	}
	base := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(cfg.APIKey.Reveal()),
		option.WithMaxRetries(maxRetries),
	}
	a.client = anthropic.NewClient(append(base, opts...)...)
	a.configured = true
	return a, nil
}

// Configured implements Port: a key is set.
func (a *anthropicAdaptor) Configured() bool { return a.configured }

// Parse implements Port. Errors never quote the key, the lines, the categories or the answer.
func (a *anthropicAdaptor) Parse(ctx context.Context, req Request) (Response, error) {
	if err := tx.MustBeOutside(ctx); err != nil {
		return Response{}, fmt.Errorf("ai parse: %w", err)
	}
	if !a.configured {
		return Response{}, fmt.Errorf("ai parse: %w", ErrNotConfigured)
	}
	style, err := checkRequest(req)
	if err != nil {
		return Response{}, fmt.Errorf("ai parse: %w", err)
	}
	user, err := userMessage(req, style)
	if err != nil {
		return Response{}, fmt.Errorf("ai parse: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Response{}, fmt.Errorf("ai parse: %w: %w", ErrUnavailable, err)
	}

	// The limit covers the SDK's retries too; ctx's own deadline still applies when it is earlier
	// (ADR-0043).
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	msg, err := a.client.Messages.New(callCtx, anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: answerSchema()},
		},
	})
	if err != nil {
		return Response{}, fmt.Errorf("ai parse: %w", callError(callCtx, err))
	}

	switch msg.StopReason {
	case anthropic.StopReasonEndTurn:
	case anthropic.StopReasonRefusal:
		return Response{}, fmt.Errorf("ai parse: %w: the model refused", ErrUnusable)
	case anthropic.StopReasonMaxTokens:
		return Response{}, fmt.Errorf("ai parse: %w: the answer was cut off at the token limit", ErrUnusable)
	default:
		return Response{}, fmt.Errorf("ai parse: %w: unexpected stop reason %q", ErrUnusable, msg.StopReason)
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	resp, err := decodeAnswer(text.String(), req.Lines, req.Categories)
	if err != nil {
		return Response{}, fmt.Errorf("ai parse: %w", err)
	}
	return resp, nil
}

// callError sorts an error of Messages.New. An API error keeps only its status, type and request
// id: its text holds the response body, which may quote the user's text. Other errors are
// described by their Go type only, for the same reason.
func callError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, ctxErr)
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		desc := fmt.Sprintf("Anthropic answered %d", apiErr.StatusCode)
		if t := apiErr.Type(); t != "" {
			desc += fmt.Sprintf(" (%s)", t)
		}
		if apiErr.RequestID != "" {
			desc += " request id " + apiErr.RequestID
		}
		switch code := apiErr.StatusCode; {
		case code == http.StatusRequestTimeout, code == http.StatusTooManyRequests, code >= http.StatusInternalServerError:
			return fmt.Errorf("%w: %s", ErrUnavailable, desc)
		default:
			// 400, 401, 403, 404, 413 ...: a mistake in the request or the settings, not
			// something a retry fixes.
			return errors.New(desc)
		}
	}
	// A connection error, or a response that could not be read: the service is treated as
	// unavailable.
	return fmt.Errorf("%w: %T", ErrUnavailable, err)
}
