package cmdutil

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// ListFlags holds the common flags shared by all list commands.
type ListFlags struct {
	Limit          int
	Skip           int
	ContinueToken  string
	ContinuePath   string
	ContinueOffset int
	JSONFields     string
	Plain          bool
	NoHeader       bool
}

// AddListFlags registers --limit, --json, --plain, and --no-header flags on a command.
func AddListFlags(cmd *cobra.Command, flags *ListFlags, defaultLimit int) {
	cmd.Flags().IntVarP(&flags.Limit, "limit", "n", defaultLimit, "Maximum number of items")
	AddJSONFieldsFlag(cmd, &flags.JSONFields)
	AddPlainFlags(cmd, flags)
}

// AddPaginatedListFlags registers list flags plus --skip and --continue pagination flags.
func AddPaginatedListFlags(cmd *cobra.Command, flags *ListFlags, defaultLimit int) {
	AddListFlags(cmd, flags, defaultLimit)
	cmd.Flags().IntVar(&flags.Skip, "skip", 0, "Skip the first N items")
	cmd.Flags().StringVar(&flags.ContinueToken, "continue", "", "Continue from a previous page token")
	cmd.MarkFlagsMutuallyExclusive("skip", "continue")
}

// AddPlainFlags registers --plain and --no-header flags on a command.
// Use this for list commands that already register --json separately.
func AddPlainFlags(cmd *cobra.Command, flags *ListFlags) {
	cmd.Flags().BoolVar(&flags.Plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&flags.NoHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")
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
	Page     *ListPageInfo
}

type paginatedListJSON struct {
	Count    int    `json:"count"`
	Items    any    `json:"items"`
	Continue string `json:"continue,omitzero"`
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
	if err := ValidateContinueConflicts(cmd); err != nil {
		return err
	}
	if cmd.Flags().Lookup("limit") != nil {
		if err := ValidateLimit(flags.Limit); err != nil {
			return err
		}
	}
	if cmd.Flags().Lookup("skip") != nil {
		if err := ValidateSkip(flags.Skip); err != nil {
			return err
		}
	}
	if cmd.Flags().Lookup("continue") != nil && flags.ContinueToken != "" {
		continuePath, continueOffset, err := DecodeContinueToken(cmd.CommandPath(), flags.ContinueToken)
		if err != nil {
			return err
		}
		flags.ContinuePath = continuePath
		flags.ContinueOffset = continueOffset
	}

	jsonResult, showHelp, err := ParseJSONFields(cmd, flags.JSONFields, fieldSpec, f.Printer.Out)
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

	continueToken := ""
	if result.Page != nil && result.Page.ContinuePath != "" {
		continueToken, err = EncodeContinueToken(cmd.CommandPath(), result.Page.ContinuePath, result.Page.ContinueOffset)
		if err != nil {
			return err
		}
	}

	if jsonResult.Enabled {
		if result.Page != nil {
			return f.Printer.PrintJSON(paginatedListJSON{
				Count:    max(result.Page.Count, len(result.Table.Rows)),
				Items:    result.JSON,
				Continue: continueToken,
			})
		}
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

	if flags.Plain {
		f.Printer.PrintPlainTable(result.Table.Headers, result.Table.Rows, flags.NoHeader)
	} else {
		if len(result.Table.FlexCols) > 0 {
			output.AutoSizeColumns(result.Table.Headers, result.Table.Rows, 2, result.Table.FlexCols...)
		}
		f.Printer.PrintTable(result.Table.Headers, result.Table.Rows)
	}

	if continueToken != "" {
		_, _ = fmt.Fprintf(f.Printer.ErrOut, "Continue: %s\n", continueToken)
	}
	return nil
}
