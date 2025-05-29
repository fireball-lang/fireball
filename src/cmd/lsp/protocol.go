package lsp

import "github.com/MineGame159/protocol"

type SemanticTokensOptions struct {
	protocol.WorkDoneProgressOptions

	Legend protocol.SemanticTokensLegend `json:"legend"`
	Range  bool                          `json:"range,omitempty"`
	Full   *SemanticTokensFull           `json:"full,omitempty"`
}

type SemanticTokensFull struct {
	Delta bool `json:"delta,omitempty"`
}
