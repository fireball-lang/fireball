package project

import "strings"

type Variables struct {
	Os   string
	Arch string
}

func GetReplacer(vars Variables) *strings.Replacer {
	replacements := make([]string, 4)

	replacements[0] = "%(OS)"
	replacements[1] = vars.Os

	replacements[2] = "%(ARCH)"
	replacements[3] = vars.Arch

	return strings.NewReplacer(replacements...)
}
