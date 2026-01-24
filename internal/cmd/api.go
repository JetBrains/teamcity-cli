package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

const maxPaginationPages = 100

var knownArrayKeys = []string{
	"build", "buildType", "project", "agent", "agentPool",
	"vcsRoot", "change", "user", "group", "test", "problem",
}

type apiOptions struct {
	method   string
	headers  []string
	fields   []string
	input    string
	include  bool
	silent   bool
	raw      bool
	paginate bool
	slurp    bool
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

  # Create a resource with POST
  tc api /app/rest/buildQueue -X POST -f 'buildType=id:MyBuild'

  # Fetch all pages and combine into array
  tc api /app/rest/builds --paginate --slurp`,
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
	cmd.Flags().BoolVar(&opts.paginate, "paginate", false, "Make additional requests to fetch all pages")
	cmd.Flags().BoolVar(&opts.slurp, "slurp", false, "Combine paginated results into a JSON array (requires --paginate)")

	cmd.MarkFlagsMutuallyExclusive("input", "field")

	return cmd
}

func runAPI(endpoint string, opts *apiOptions) error {
	if opts.paginate && opts.method != "GET" {
		return fmt.Errorf("--paginate can only be used with GET requests")
	}
	if opts.slurp && !opts.paginate {
		return fmt.Errorf("--slurp requires --paginate")
	}

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

	if opts.paginate {
		return runAPIPaginated(client, endpoint, headers, opts)
	}

	resp, err := client.RawRequest(opts.method, endpoint, body, headers)
	if err != nil {
		return err
	}

	return outputAPIResponse(resp.Body, resp.StatusCode, resp.Headers, opts)
}

func runAPIPaginated(client api.ClientInterface, endpoint string, headers map[string]string, opts *apiOptions) error {
	pages, err := fetchAllPages(client, endpoint, headers)
	if err != nil {
		return err
	}

	if len(pages) == 0 {
		return nil
	}

	if opts.slurp {
		arrayKey, err := detectArrayKey(pages[0])
		if err != nil {
			return fmt.Errorf("failed to detect array key: %w", err)
		}
		if arrayKey == "" {
			return fmt.Errorf("--slurp requires response with array field (build, project, etc.)")
		}

		merged, err := mergePages(pages, arrayKey)
		if err != nil {
			return fmt.Errorf("failed to merge pages: %w", err)
		}
		return outputAPIResponse(merged, http.StatusOK, nil, opts)
	}

	for i, page := range pages {
		if i > 0 {
			fmt.Println()
		}
		if err := outputAPIResponse(page, http.StatusOK, nil, opts); err != nil {
			return err
		}
	}

	return nil
}

func outputAPIResponse(body []byte, statusCode int, respHeaders map[string][]string, opts *apiOptions) error {
	if opts.silent && statusCode >= 200 && statusCode < 300 {
		return nil
	}

	if opts.include && respHeaders != nil {
		fmt.Printf("HTTP/1.1 %d %s\n", statusCode, http.StatusText(statusCode))
		for k, v := range respHeaders {
			for _, val := range v {
				fmt.Printf("%s: %s\n", k, val)
			}
		}
		fmt.Println()
	}

	isError := statusCode < 200 || statusCode >= 300
	isHTML := len(body) > 0 && (strings.HasPrefix(strings.TrimSpace(string(body)), "<!") ||
		strings.HasPrefix(strings.TrimSpace(string(body)), "<html"))

	if len(body) > 0 {
		if opts.raw {
			fmt.Print(string(body))
		} else if isHTML && isError {
			// Don't dump HTML error pages, show clean error
			output.Warn("Server returned HTML error page (status %d)", statusCode)
		} else {
			var jsonData interface{}
			if err := json.Unmarshal(body, &jsonData); err == nil {
				prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
				if err == nil {
					fmt.Println(string(prettyJSON))
				} else {
					fmt.Print(string(body))
				}
			} else {
				fmt.Print(string(body))
			}
		}
	}

	if isError {
		if !opts.include && len(body) == 0 {
			output.Warn("Request failed with status %d", statusCode)
		}
		return fmt.Errorf("request failed with status %d", statusCode)
	}

	return nil
}

func fetchAllPages(client api.ClientInterface, endpoint string, headers map[string]string) ([][]byte, error) {
	var pages [][]byte
	currentEndpoint := endpoint

	for i := 0; i < maxPaginationPages; i++ {
		resp, err := client.RawRequest("GET", currentEndpoint, nil, headers)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if len(resp.Body) > 0 {
				return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(resp.Body))
			}
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}

		pages = append(pages, resp.Body)

		nextHref, err := extractNextHref(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("--paginate requires JSON response: %w", err)
		}

		if nextHref == "" {
			break
		}

		currentEndpoint = nextHref
	}

	return pages, nil
}

func extractNextHref(data []byte) (string, error) {
	var resp struct {
		NextHref string `json:"nextHref"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.NextHref, nil
}

func detectArrayKey(data []byte) (string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}

	for _, key := range knownArrayKeys {
		if raw, exists := obj[key]; exists {
			var arr []json.RawMessage
			if json.Unmarshal(raw, &arr) == nil {
				return key, nil
			}
		}
	}
	return "", nil
}

func extractArrayItems(data []byte, key string) ([]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	raw, exists := obj[key]
	if !exists {
		return nil, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func mergePages(pages [][]byte, arrayKey string) ([]byte, error) {
	allItems := make([]json.RawMessage, 0)

	for _, page := range pages {
		items, err := extractArrayItems(page, arrayKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract items from page: %w", err)
		}
		allItems = append(allItems, items...)
	}

	return json.Marshal(allItems)
}
