package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/millicano106/kubeOptimum/internal/advisor"
)

// JSONRenderer renders recommendations as a pretty-printed JSON array.
type JSONRenderer struct{}

func (j *JSONRenderer) Render(w io.Writer, recs []advisor.Recommendation) error {
	if recs == nil {
		recs = []advisor.Recommendation{}
	}
	out, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recommendations: %w", err)
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}
