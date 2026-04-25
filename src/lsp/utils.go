package lsp

import (
	"context"
	"fmt"
	"strings"

	"github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) info(ctx context.Context, format string, args ...any) {
	_ = h.Client.LogMessage(ctx, &lsp.LogMessageParams{
		Type:    lsp.MessageTypeInfo,
		Message: fmt.Sprintf(format, args...),
	})
}

func (h *Handler) warning(ctx context.Context, format string, args ...any) {
	_ = h.Client.LogMessage(ctx, &lsp.LogMessageParams{
		Type:    lsp.MessageTypeWarning,
		Message: fmt.Sprintf(format, args...),
	})
}

func (h *Handler) error(ctx context.Context, format string, args ...any) {
	_ = h.Client.LogMessage(ctx, &lsp.LogMessageParams{
		Type:    lsp.MessageTypeError,
		Message: fmt.Sprintf(format, args...),
	})
}

func uriPath(uri lsp.DocumentURI) string {
	str := string(uri)

	if !strings.HasPrefix(str, "file://") {
		panic("lsp.uriPath() - Non file URIs are not supported")
	}

	return str[7:]
}
