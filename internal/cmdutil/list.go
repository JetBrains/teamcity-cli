package cmdutil

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// ListFlags holds the common flags shared by all list commands.
type ListFlags struct {
	Limit      int
	JSONFields string
}

// AddListFlags registers --limit and --json flags on a command.
func AddListFlags(cmd *cobra.Command, flags *ListFlags, defaultLimit int) {
	cmd.Flags().IntVarP(&flags.Limit, "limit", "n", defaultLimit, "Maximum number of items")
	AddJSONFieldsFlag(cmd, &flags.JSONFields)
}

// ListTable holds the data needed to print a table.
type ListTable struct {
	Headers  []string
	Rows     [][]string
	FlexCols []int
}

// ListResult is returned by a list command's fetch function.
// Set either JSON (for JSON output) or Table (for table output).
type ListResult struct {
	JSON     any
	Table    ListTable
	EmptyMsg string
}

// RunList handles the shared boilerplate for list commands:
// limit validation, JSON field parsing, client creation, fetch, and output.
func RunList(
	f *Factory,
	cmd *cobra.Command,
	flags *ListFlags,
	fieldSpec *api.FieldSpec,
	fetch func(client api.ClientInterface, fields []string) (*ListResult, error),
) error {
	if cmd.Flags().Lookup("limit") != nil {
		if err := ValidateLimit(flags.Limit); err != nil {
			return err
		}
	}

	jsonResult, showHelp, err := ParseJSONFields(cmd, flags.JSONFields, fieldSpec)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	result, err := fetch(client, jsonResult.Fields)
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return f.Printer.PrintJSON(result.JSON)
	}

	if len(result.Table.Rows) == 0 {
		msg := result.EmptyMsg
		if msg == "" {
			msg = "No items found"
		}
		f.Printer.Info(msg)
		return nil
	}

	if len(result.Table.FlexCols) > 0 {
		output.AutoSizeColumns(result.Table.Headers, result.Table.Rows, 2, result.Table.FlexCols...)
	}
	output.PrintTable(result.Table.Headers, result.Table.Rows)
	return nil
}
