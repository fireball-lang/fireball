package lsp

import (
	"context"
	"fireball/core"
	"fmt"
	"strings"

	"github.com/fireball-lang/protocol"
)

func (s *Server) info(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.Logger.Info(msg)

	_ = s.Client.LogMessage(ctx, &protocol.LogMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: msg,
	})
}

func (s *Server) warn(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.Logger.Warn(msg)

	_ = s.Client.LogMessage(ctx, &protocol.LogMessageParams{
		Type:    protocol.MessageTypeWarning,
		Message: msg,
	})
}

func (s *Server) error(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.Logger.Error(msg)

	_ = s.Client.LogMessage(ctx, &protocol.LogMessageParams{
		Type:    protocol.MessageTypeError,
		Message: msg,
	})
}

func toLspRange(r core.Range) protocol.Range {
	return protocol.Range{
		Start: toLspPos(r.Start),
		End:   toLspPos(r.End),
	}
}

func toLspPos(pos core.Pos) protocol.Position {
	return protocol.Position{
		Line:      pos.Line - 1,
		Character: pos.Column - 1,
	}
}

func toCoreRange(r protocol.Range) core.Range {
	return core.Range{
		Start: toCorePos(r.Start),
		End:   toCorePos(r.End),
	}
}

func toCorePos(pos protocol.Position) core.Pos {
	return core.Pos{
		Line:   pos.Line + 1,
		Column: pos.Character + 1,
	}
}

func uriPath(uri protocol.DocumentURI) string {
	str := string(uri)

	if !strings.HasPrefix(str, "file://") {
		panic("lsp.uriPath() - Non file URIs are not supported")
	}

	return str[7:]
}
