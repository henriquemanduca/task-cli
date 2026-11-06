package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Supported output formats
const (
	FormatTSV   = "tsv"
	FormatTable = "table"
)

// validateFormat checks if the provided format string is supported.
// If format is empty, it returns the default "tsv".
func validateFormat(format string) (string, error) {
	fmtLower := strings.ToLower(strings.TrimSpace(format))
	if fmtLower == "" {
		return FormatTSV, nil
	}
	switch fmtLower {
	case FormatTSV, FormatTable:
		return fmtLower, nil
	default:
		return "", fmt.Errorf("invalid format '%s'. Valid options: tsv, table", format)
	}
}

// printOutput prints tabular data using the specified format ("tsv" or "table").
// It writes to the given io.Writer (typically os.Stdout).
func printOutput(w io.Writer, format string, noHeader bool, headers []string, rows [][]string) error {
	resolvedFormat, err := validateFormat(format)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	switch resolvedFormat {
	case FormatTSV:
		if !noHeader && len(headers) > 0 {
			fmt.Fprintln(w, strings.Join(headers, "\t"))
		}
		for _, row := range rows {
			fmt.Fprintln(w, strings.Join(row, "\t"))
		}
		return nil

	case FormatTable:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if !noHeader && len(headers) > 0 {
			upperHeaders := make([]string, len(headers))
			for i, h := range headers {
				upperHeaders[i] = strings.ToUpper(h)
			}
			fmt.Fprintln(tw, strings.Join(upperHeaders, "\t"))
		}
		for _, row := range rows {
			fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		return tw.Flush()

	default:
		return fmt.Errorf("invalid format '%s'. Valid options: tsv, table", format)
	}
}
