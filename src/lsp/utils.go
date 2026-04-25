package lsp

import (
	"context"
	"fireball/core"
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

func toLspRange(r core.Range) lsp.Range {
	return lsp.Range{
		Start: toLspPos(r.Start),
		End:   toLspPos(r.End),
	}
}

func toLspPos(pos core.Pos) lsp.Position {
	return lsp.Position{
		Line:      int(pos.Line) - 1,
		Character: int(pos.Column) - 1,
	}
}

func toCoreRange(r lsp.Range) core.Range {
	return core.Range{
		Start: toCorePos(r.Start),
		End:   toCorePos(r.End),
	}
}

func toCorePos(pos lsp.Position) core.Pos {
	return core.Pos{
		Line:   uint32(pos.Line) + 1,
		Column: uint32(pos.Character) + 1,
	}
}

func uriPath(uri lsp.DocumentURI) string {
	str := string(uri)

	if !strings.HasPrefix(str, "file://") {
		panic("lsp.uriPath() - Non file URIs are not supported")
	}

	return str[7:]
}
