package lsp

import (
	"context"
	"errors"
	"github.com/MineGame159/protocol"
)

func (s *server) WorkDoneProgressCancel(ctx context.Context, params *protocol.WorkDoneProgressCancelParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) LogTrace(ctx context.Context, params *protocol.LogTraceParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) SetTrace(ctx context.Context, params *protocol.SetTraceParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) (result []protocol.CodeAction, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) (result []protocol.CodeLens, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) CodeLensResolve(ctx context.Context, params *protocol.CodeLens) (result *protocol.CodeLens, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) ColorPresentation(ctx context.Context, params *protocol.ColorPresentationParams) (result []protocol.ColorPresentation, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Completion(ctx context.Context, params *protocol.CompletionParams) (result *protocol.CompletionList, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) CompletionResolve(ctx context.Context, params *protocol.CompletionItem) (result *protocol.CompletionItem, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Declaration(ctx context.Context, params *protocol.DeclarationParams) (result []protocol.Location, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Definition(ctx context.Context, params *protocol.DefinitionParams) (result []protocol.Location, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) DocumentColor(ctx context.Context, params *protocol.DocumentColorParams) (result []protocol.ColorInformation, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DocumentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) (result []protocol.DocumentHighlight, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DocumentLink(ctx context.Context, params *protocol.DocumentLinkParams) (result []protocol.DocumentLink, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DocumentLinkResolve(ctx context.Context, params *protocol.DocumentLink) (result *protocol.DocumentLink, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (result interface{}, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) (result []protocol.FoldingRange, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Implementation(ctx context.Context, params *protocol.ImplementationParams) (result []protocol.Location, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) OnTypeFormatting(ctx context.Context, params *protocol.DocumentOnTypeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (result *protocol.Range, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) References(ctx context.Context, params *protocol.ReferenceParams) (result []protocol.Location, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Rename(ctx context.Context, params *protocol.RenameParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (result *protocol.SignatureHelp, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) TypeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) (result []protocol.Location, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) WillSave(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) WillSaveWaitUntil(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (result []protocol.TextEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) ShowDocument(ctx context.Context, params *protocol.ShowDocumentParams) (result *protocol.ShowDocumentResult, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) WillCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DidCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) WillRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DidRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) WillDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) DidDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (err error) {
	return errors.New("not implemented")
}

func (s *server) CodeLensRefresh(ctx context.Context) (err error) {
	return errors.New("not implemented")
}

func (s *server) PrepareCallHierarchy(ctx context.Context, params *protocol.CallHierarchyPrepareParams) (result []protocol.CallHierarchyItem, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) IncomingCalls(ctx context.Context, params *protocol.CallHierarchyIncomingCallsParams) (result []protocol.CallHierarchyIncomingCall, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) OutgoingCalls(ctx context.Context, params *protocol.CallHierarchyOutgoingCallsParams) (result []protocol.CallHierarchyOutgoingCall, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) SemanticTokensFullDelta(ctx context.Context, params *protocol.SemanticTokensDeltaParams) (result interface{}, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (result *protocol.SemanticTokens, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) LinkedEditingRange(ctx context.Context, params *protocol.LinkedEditingRangeParams) (result *protocol.LinkedEditingRanges, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Moniker(ctx context.Context, params *protocol.MonikerParams) (result []protocol.Moniker, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) InlayHint(ctx context.Context, params *protocol.InlayHintParams) (result []protocol.InlayHint, err error) {
	return nil, errors.New("not implemented")
}

func (s *server) Request(ctx context.Context, method string, params interface{}) (result interface{}, err error) {
	return nil, errors.New("not implemented")
}
