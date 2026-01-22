package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

type apiOptions struct {
	method  string
	headers []string
	fields  []string
	input   string
	include bool
	silent  bool
	raw     bool
}

func newAPICmd() *cobra.Command {
	opts := &apiOptions{}

	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated API request",
		Long: `Make an authenticated HTTP request to the TeamCity REST API.

The endpoint argument should be the path portion of the URL,
starting with /app/rest/. The base URL and authentication
are handled automatically.

This command is useful for:
- Accessing API features not yet supported by the CLI
- Scripting and automation
- Debugging and exploration`,
		Args: cobra.ExactArgs(1),
		Example: `  # Get server info
  tc api /app/rest/server

  # List projects
  tc api /app/rest/projects

  # Get a specific build
  tc api /app/rest/builds/id:12345

  # Create a resource with POST
  tc api /app/rest/buildQueue --method POST --field 'buildType=id:MyProject_Build'

  # Use custom headers
  tc api /app/rest/builds -H "Accept: application/xml"

  # Read request body from stdin
  echo '{"buildType":{"id":"MyBuild"}}' | tc api /app/rest/buildQueue -X POST --input -

  # Include response headers in output
  tc api /app/rest/server --include

  # Silent mode (only show errors)
  tc api /app/rest/server --silent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.method, "method", "X", "GET", "HTTP method to use")
	cmd.Flags().StringArrayVarP(&opts.headers, "header", "H", nil, "Add a custom header (can be repeated)")
	cmd.Flags().StringArrayVarP(&opts.fields, "field", "f", nil, "Add a body field as key=value (builds JSON object)")
	cmd.Flags().StringVar(&opts.input, "input", "", "Read request body from file (use - for stdin)")
	cmd.Flags().BoolVarP(&opts.include, "include", "i", false, "Include response headers in output")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Suppress output on success")
	cmd.Flags().BoolVar(&opts.raw, "raw", false, "Output raw response without formatting")

	return cmd
}

func runAPI(endpoint string, opts *apiOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	headers := make(map[string]string)
	for _, h := range opts.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid header format %q (expected 'Key: Value')", h)
		}
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	var body io.Reader
	if opts.input != "" {
		if opts.input == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			body = bytes.NewReader(data)
		} else {
			data, err := os.ReadFile(opts.input)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", opts.input, err)
			}
			body = bytes.NewReader(data)
		}
	} else if len(opts.fields) > 0 {
		jsonBody := make(map[string]interface{})
		for _, f := range opts.fields {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid field format %q (expected 'key=value')", f)
			}
			key := parts[0]
			value := parts[1]

			var jsonValue interface{}
			if err := json.Unmarshal([]byte(value), &jsonValue); err != nil {
				jsonValue = value
			}
			jsonBody[key] = jsonValue
		}

		jsonData, err := json.Marshal(jsonBody)
		if err != nil {
			return fmt.Errorf("failed to build JSON body: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	resp, err := client.RawRequest(opts.method, endpoint, body, headers)
	if err != nil {
		return err
	}

	if opts.silent && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if opts.include {
		fmt.Printf("HTTP/1.1 %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		for k, v := range resp.Headers {
			for _, val := range v {
				fmt.Printf("%s: %s\n", k, val)
			}
		}
		fmt.Println()
	}

	if len(resp.Body) > 0 {
		if opts.raw {
			fmt.Print(string(resp.Body))
		} else {
			var jsonData interface{}
			if err := json.Unmarshal(resp.Body, &jsonData); err == nil {
				prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
				if err == nil {
					fmt.Println(string(prettyJSON))
				} else {
					fmt.Print(string(resp.Body))
				}
			} else {
				fmt.Print(string(resp.Body))
			}
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if !opts.include && len(resp.Body) == 0 {
			output.Warn("Request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil
}
