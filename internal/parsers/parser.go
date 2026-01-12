package parsers

// Parser interface for protocol parsers
type Parser interface {
	Parse() ([]byte, error)
}
