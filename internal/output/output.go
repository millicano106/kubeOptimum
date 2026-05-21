package output

import (
	"fmt"
	"io"

	"github.com/millicano106/kubeOptimum/internal/advisor"
)

// Renderer writes a slice of Recommendations to an io.Writer.
type Renderer interface {
	Render(w io.Writer, recs []advisor.Recommendation) error
}

// NewRenderer returns the Renderer for the requested format.
func NewRenderer(format string) (Renderer, error) {
	switch format {
	case "table", "":
		return &TableRenderer{}, nil
	case "json":
		return &JSONRenderer{}, nil
	case "yaml":
		return &YAMLRenderer{}, nil
	default:
		return nil, fmt.Errorf("unknown output format %q (use table, json, yaml)", format)
	}
}
