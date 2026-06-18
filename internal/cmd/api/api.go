package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
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

func NewCmd(f *cmdutil.Factory) *cobra.Command {
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
- Debugging and exploration

See: https://www.jetbrains.com/help/teamcity/rest/teamcity-rest-api-documentation.html`,
		Args: cobra.ExactArgs(1),
		Example: `  # Get server info
  teamcity api '/app/rest/server'

  # List projects
  teamcity api '/app/rest/projects'

  # Create a resource with POST
  teamcity api '/app/rest/buildQueue' -X POST -f 'buildType=id:MyBuild'

  # Fetch all pages and combine into array
  teamcity api '/app/rest/builds' --paginate --slurp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(f, args[0], opts)
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

	_ = cmd.RegisterFlagCompletionFunc("method", completion.HTTPMethods())
	_ = cmd.MarkFlagFilename("input")

	return cmd
}

func runAPI(f *cmdutil.Factory, endpoint string, opts *apiOptions) error {
	if opts.paginate && opts.method != "GET" {
		return errors.New("--paginate can only be used with GET requests")
	}
	if opts.slurp && !opts.paginate {
		return errors.New("--slurp requires --paginate")
	}
	if opts.method == "GET" && len(opts.fields) > 0 {
		f.Printer.Warn("--field is ignored for GET requests. Use -X POST to send a request body.")
	}
	if opts.method == "GET" && opts.input != "" {
		f.Printer.Warn("--input is ignored for GET requests. Use -X POST to send a request body.")
	}

	client, err := f.Client()
	if err != nil {
		return err
	}
	client.SetCommandName("api")

	headers := make(map[string]string)
	for _, h := range opts.headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return fmt.Errorf("invalid header format %q (expected 'Key: Value')", h)
		}
		headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	var body io.Reader
	if opts.input != "" {
		if opts.input == "-" {
			data, err := io.ReadAll(f.IOStreams.In)
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
		jsonBody := make(map[string]any)
		for _, f := range opts.fields {
			key, value, ok := strings.Cut(f, "=")
			if !ok {
				return fmt.Errorf("invalid field format %q (expected 'key=value')", f)
			}

			var jsonValue any
			if err := json.Unmarshal([]byte(value), &jsonValue); err != nil {
				if k, v, ok := strings.Cut(value, ":"); ok && k != "" && v != "" {
					jsonValue = map[string]string{k: v}
				} else {
					jsonValue = value
				}
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
		lastStatus, err := runAPIPaginated(f.Context(), f.Printer, client, endpoint, headers, opts)
		f.Analytics.TrackAPI(analytics.APIEvent{
			Method:     opts.method,
			Endpoint:   endpoint,
			StatusCode: statusCodeForTracking(err, lastStatus),
			Paginated:  true,
			Slurp:      opts.slurp,
			HadFields:  len(opts.fields) > 0,
			HadInput:   opts.input != "",
		})
		return err
	}

	resp, err := client.RawRequest(f.Context(), opts.method, endpoint, body, headers)
	f.Analytics.TrackAPI(analytics.APIEvent{
		Method:     opts.method,
		Endpoint:   endpoint,
		StatusCode: statusCodeForTracking(err, statusCodeOf(resp)),
		Paginated:  false,
		Slurp:      false,
		HadFields:  len(opts.fields) > 0,
		HadInput:   opts.input != "",
	})
	if err != nil {
		return err
	}

	return outputAPIResponse(f.Printer, resp.Body, resp.StatusCode, resp.Headers, opts)
}

func statusCodeOf(r *api.RawResponse) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

func statusCodeForTracking(err error, observed int) int {
	if err != nil && observed == 0 {
		return 0
	}
	return observed
}

// runAPIPaginated drives the multi-page fetch and returns (lastStatus, err); lastStatus is the HTTP status of the failed request when err is non-nil and 200 on success, so analytics never silently records 200 for a failed pagination.
func runAPIPaginated(ctx context.Context, p *output.Printer, client api.ClientInterface, endpoint string, headers map[string]string, opts *apiOptions) (int, error) {
	pages, lastStatus, err := fetchAllPages(ctx, client, endpoint, headers)
	if err != nil {
		return lastStatus, err
	}

	if len(pages) == 0 {
		return lastStatus, nil
	}

	if opts.slurp {
		arrayKey, err := detectArrayKey(pages[0])
		if err != nil {
			return lastStatus, fmt.Errorf("failed to detect array key: %w", err)
		}
		if arrayKey == "" {
			return lastStatus, errors.New("--slurp requires response with array field (build, project, etc.)")
		}

		merged, err := mergePages(pages, arrayKey)
		if err != nil {
			return lastStatus, fmt.Errorf("failed to merge pages: %w", err)
		}
		return lastStatus, outputAPIResponse(p, merged, http.StatusOK, nil, opts)
	}

	for i, page := range pages {
		if i > 0 {
			_, _ = fmt.Fprintln(p.Out)
		}
		if err := outputAPIResponse(p, page, http.StatusOK, nil, opts); err != nil {
			return lastStatus, err
		}
	}

	return lastStatus, nil
}

func outputAPIResponse(p *output.Printer, body []byte, statusCode int, respHeaders map[string][]string, opts *apiOptions) error {
	if opts.silent && statusCode >= 200 && statusCode < 300 {
		return nil
	}

	if opts.include && respHeaders != nil {
		_, _ = fmt.Fprintf(p.Out, "HTTP/1.1 %d %s\n", statusCode, http.StatusText(statusCode))
		for k, v := range respHeaders {
			for _, val := range v {
				_, _ = fmt.Fprintf(p.Out, "%s: %s\n", k, val)
			}
		}
		_, _ = fmt.Fprintln(p.Out)
	}

	isError := statusCode < 200 || statusCode >= 300
	isHTML := len(body) > 0 && (strings.HasPrefix(strings.TrimSpace(string(body)), "<!") ||
		strings.HasPrefix(strings.TrimSpace(string(body)), "<html"))

	if isError {
		if opts.raw && len(body) > 0 {
			_, _ = fmt.Fprint(p.Out, string(body))
		}
		return api.ErrorFromBody(statusCode, body)
	}

	if len(body) > 0 {
		switch {
		case opts.raw:
			_, _ = fmt.Fprint(p.Out, string(body))
		case isHTML:
			p.Warn("Server returned HTML page (status %d)", statusCode)
		default:
			if prettyJSON, ok := prettyPrintJSON(body); ok {
				_, _ = fmt.Fprintln(p.Out, prettyJSON)
			} else {
				_, _ = fmt.Fprint(p.Out, string(body))
			}
		}
	}

	return nil
}

// fetchAllPages walks the pagination chain and returns (pages, lastStatus, err); lastStatus is the HTTP status of the failed request when err is non-nil (0 for transport errors that produced no response), or 200 on success.
func fetchAllPages(ctx context.Context, client api.ClientInterface, endpoint string, headers map[string]string) ([][]byte, int, error) {
	var pages [][]byte
	currentEndpoint := endpoint
	lastStatus := 0

	for range maxPaginationPages {
		resp, err := client.RawRequest(ctx, "GET", currentEndpoint, nil, headers)
		if err != nil {
			// Transport error: no HTTP response observed. Returning the previous page's status here
			// would misclassify the failure as the prior page's success in analytics.
			return nil, 0, err
		}
		lastStatus = resp.StatusCode

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, lastStatus, api.ErrorFromBody(resp.StatusCode, resp.Body)
		}

		pages = append(pages, resp.Body)

		nextHref, err := extractNextHref(resp.Body)
		if err != nil {
			return nil, lastStatus, fmt.Errorf("--paginate requires JSON response: %w", err)
		}

		if nextHref == "" {
			return pages, lastStatus, nil
		}

		// nextHref carries the server's context path (e.g. /bs); strip it so RawRequest's BaseURL doesn't double it.
		currentEndpoint = client.NormalizePaginationPath(nextHref)
	}

	return nil, lastStatus, fmt.Errorf("--paginate hit the page cap of %d with more pages still available; refine your query (e.g. add a locator filter)", maxPaginationPages)
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

// prettyPrintJSON formats body as indented JSON, converting XML errors to JSON first if needed.
func prettyPrintJSON(body []byte) (string, bool) {
	var data any

	if err := json.Unmarshal(body, &data); err != nil {
		if xmlErrs := api.ParseXMLErrors(body); xmlErrs != nil {
			data = xmlErrs
		} else {
			return "", false
		}
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", false
	}
	return string(pretty), true
}
