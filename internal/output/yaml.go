package output

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/millicano106/kubeOptimum/internal/advisor"
)

// YAMLRenderer renders recommendations as a YAML document.
type YAMLRenderer struct{}

func (y *YAMLRenderer) Render(w io.Writer, recs []advisor.Recommendation) error {
	if recs == nil {
		recs = []advisor.Recommendation{}
	}
	out, err := yaml.Marshal(recs)
	if err != nil {
		return fmt.Errorf("marshal recommendations: %w", err)
	}
	_, err = fmt.Fprint(w, string(out))
	return err
}
