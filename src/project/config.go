package project

type Type string

const (
	Library    Type = "library"
	Executable Type = "executable"
)

type Config struct {
	Name string
	Type Type
}
