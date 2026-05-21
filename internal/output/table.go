package output

import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"github.com/millicano106/kubeOptimum/internal/advisor"
)

// TableRenderer renders recommendations as a coloured ASCII table.
type TableRenderer struct{}

func (t *TableRenderer) Render(w io.Writer, recs []advisor.Recommendation) error {
	if len(recs) == 0 {
		fmt.Fprintln(w, "No recommendations — cluster looks well-optimised.")
		return nil
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"SEVERITY", "CATEGORY", "RESOURCE", "SUMMARY", "EST. SAVING/MO"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetColumnSeparator(" ")
	table.SetHeaderLine(true)

	for _, rec := range recs {
		resource := rec.Resource.Name
		if rec.Resource.Namespace != "" {
			resource = rec.Resource.Namespace + "/" + rec.Resource.Name
		}

		saving := ""
		if rec.EstimatedMonthlySaving > 0 {
			saving = fmt.Sprintf("~$%.2f", rec.EstimatedMonthlySaving)
		}

		severityStr := colourSeverity(rec.Severity)

		table.Append([]string{
			severityStr,
			string(rec.Category),
			resource,
			rec.Summary,
			saving,
		})
	}

	table.Render()
	return nil
}

func colourSeverity(s advisor.Severity) string {
	switch s {
	case advisor.SeverityHigh:
		return color.New(color.FgRed, color.Bold).Sprint(string(s))
	case advisor.SeverityMedium:
		return color.New(color.FgYellow, color.Bold).Sprint(string(s))
	case advisor.SeverityLow:
		return color.New(color.FgGreen).Sprint(string(s))
	default:
		return string(s)
	}
}
