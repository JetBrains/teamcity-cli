// Package plugin implements commands for managing TeamCity server plugins.
package plugin

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	pluginDescriptorPath = "teamcity-plugin.xml"
	pluginUploadPath     = "/admin/pluginUpload.html"
	pluginsAdminPath     = "/admin/admin.html?item=plugins"
	pluginsActionPath    = "/admin/plugins.html"
)

var (
	uploadErrorPattern      = regexp.MustCompile(`UploadPluginDialog\.error\("((?:\\.|[^"])*)"\)`)
	htmlTagPattern          = regexp.MustCompile(`<[^>]+>`)
	registeredPluginPattern = regexp.MustCompile(
		`(?s)BS\.Plugins\.registerPlugin\(\s*'((?:\\.|[^'])*)'\s*,\s*'((?:\\.|[^'])*)'\s*,\s*(?:true|false)\s*,\s*'((?:\\.|[^'])*)'\s*,\s*'((?:\\.|[^'])*)'\s*\)`,
	)
)

type uploadOptions struct {
	hotReload bool
	json      bool
}

type pluginDescriptor struct {
	Info struct {
		Name    string `xml:"name"`
		Version string `xml:"version"`
	} `xml:"info"`
}

type uploadResult struct {
	File        string `json:"file"`
	Plugin      string `json:"plugin"`
	Version     string `json:"version,omitzero"`
	Uploaded    bool   `json:"uploaded"`
	HotReloaded bool   `json:"hot_reloaded"`
}

// NewCmd creates the plugin command.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage TeamCity server plugins",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newUploadCmd(f))
	return cmd
}

func newUploadCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &uploadOptions{}
	cmd := &cobra.Command{
		Use:   "upload <plugin.zip>",
		Short: "Upload a TeamCity plugin archive",
		Long: `Upload a TeamCity plugin ZIP archive to the server.

Use --hot-reload to apply an update to an already loaded runtime-reloadable
plugin without restarting the TeamCity server.`,
		Example: `  teamcity server plugin upload ./my-plugin.zip
  teamcity server plugin upload ./my-plugin.zip --hot-reload`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpload(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.hotReload, "hot-reload", false, "Reload the uploaded plugin without restarting the server")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	return cmd
}

func runUpload(f *cmdutil.Factory, archivePath string, opts *uploadOptions) error {
	descriptor, err := readPluginDescriptor(archivePath)
	if err != nil {
		return err
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := uploadPlugin(client, f.Context(), archivePath); err != nil {
		return err
	}

	result := uploadResult{
		File:     filepath.Base(archivePath),
		Plugin:   descriptor.Info.Name,
		Version:  descriptor.Info.Version,
		Uploaded: true,
	}

	if opts.hotReload {
		uuid, err := findPluginUUID(client, f.Context(), descriptor.Info.Name)
		if err != nil {
			return fmt.Errorf("plugin was uploaded but hot reload failed: %w", err)
		}
		if err := hotReloadPlugin(client, f.Context(), uuid); err != nil {
			return fmt.Errorf("plugin was uploaded but hot reload failed: %w", err)
		}
		result.HotReloaded = true
	}

	if opts.json {
		return f.Printer.PrintJSON(result)
	}

	f.Printer.Success("Uploaded plugin %q from %s", descriptor.Info.Name, filepath.Base(archivePath))
	if result.HotReloaded {
		f.Printer.Success("Hot-reloaded plugin %q", descriptor.Info.Name)
	}
	return nil
}

func readPluginDescriptor(archivePath string) (*pluginDescriptor, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, api.Validation(
			fmt.Sprintf("failed to open plugin archive: %v", err),
			"Provide a readable TeamCity plugin ZIP archive",
		)
	}
	defer func() { _ = archive.Close() }()

	for _, file := range archive.File {
		if file.Name != pluginDescriptorPath {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return nil, api.Validation(
				fmt.Sprintf("failed to read %s: %v", pluginDescriptorPath, err),
				"Provide a valid TeamCity plugin ZIP archive",
			)
		}

		var descriptor pluginDescriptor
		decodeErr := xml.NewDecoder(reader).Decode(&descriptor)
		closeErr := reader.Close()
		if decodeErr != nil {
			return nil, api.Validation(
				fmt.Sprintf("failed to parse %s: %v", pluginDescriptorPath, decodeErr),
				"Provide a valid TeamCity plugin ZIP archive",
			)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close %s: %w", pluginDescriptorPath, closeErr)
		}
		descriptor.Info.Name = strings.TrimSpace(descriptor.Info.Name)
		descriptor.Info.Version = strings.TrimSpace(descriptor.Info.Version)
		if descriptor.Info.Name == "" {
			return nil, api.Validation(
				pluginDescriptorPath+" does not declare a plugin name",
				"Provide a descriptor that declares info/name",
			)
		}
		return &descriptor, nil
	}

	return nil, api.Validation(
		"plugin archive does not contain "+pluginDescriptorPath,
		"Provide a TeamCity plugin ZIP archive",
	)
}

func uploadPlugin(client api.ClientInterface, ctx context.Context, archivePath string) error {
	body, contentType, err := newUploadBody(archivePath)
	if err != nil {
		return err
	}

	response, err := client.RawRequest(ctx, http.MethodPost, pluginUploadPath, body, map[string]string{
		"Accept":       "text/html",
		"Content-Type": contentType,
		"Origin":       client.ServerURL(),
	})
	if err != nil {
		return fmt.Errorf("failed to upload plugin: %w", err)
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to upload plugin: %w", api.ErrorFromBody(response.StatusCode, response.Body))
	}
	if match := uploadErrorPattern.FindSubmatch(response.Body); match != nil {
		return fmt.Errorf("failed to upload plugin: %s", decodeJSString(string(match[1])))
	}
	return nil
}

func newUploadBody(archivePath string) (io.Reader, string, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open plugin archive: %w", err)
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()
	go func() {
		writeErr := multipartWriter.WriteField("fileName", filepath.Base(archivePath))
		if writeErr == nil {
			var part io.Writer
			part, writeErr = multipartWriter.CreateFormFile("file:fileToUpload", filepath.Base(archivePath))
			if writeErr == nil {
				_, writeErr = io.Copy(part, archive)
			}
		}
		if closeErr := multipartWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if closeErr := archive.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = writer.CloseWithError(writeErr)
	}()

	return reader, contentType, nil
}

func findPluginUUID(client api.ClientInterface, ctx context.Context, pluginName string) (string, error) {
	response, err := client.RawRequest(ctx, http.MethodGet, pluginsAdminPath, nil, map[string]string{"Accept": "text/html"})
	if err != nil {
		return "", fmt.Errorf("failed to inspect installed plugins: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to inspect installed plugins: %w", api.ErrorFromBody(response.StatusCode, response.Body))
	}

	for _, match := range registeredPluginPattern.FindAllSubmatch(response.Body, -1) {
		if decodeJSString(string(match[1])) == pluginName {
			return decodeJSString(string(match[4])), nil
		}
	}
	return "", fmt.Errorf("could not find plugin %q on the TeamCity plugins page", pluginName)
}

func hotReloadPlugin(client api.ClientInterface, ctx context.Context, uuid string) error {
	form := url.Values{
		"action":  {"setEnabled"},
		"enabled": {"true"},
		"reload":  {"true"},
		"uuid":    {uuid},
	}
	response, err := client.RawRequest(ctx, http.MethodPost, pluginsActionPath, strings.NewReader(form.Encode()), map[string]string{
		"Accept":       "application/xml",
		"Content-Type": "application/x-www-form-urlencoded",
		"Origin":       client.ServerURL(),
	})
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return api.ErrorFromBody(response.StatusCode, response.Body)
	}

	message := ajaxResponseMessage(response.Body)
	if message != "Plugin successfully reloaded" {
		if message == "" {
			message = "TeamCity returned an empty hot-reload response"
		}
		return errors.New(message)
	}
	return nil
}

func ajaxResponseMessage(body []byte) string {
	var response struct {
		Text   string `xml:",chardata"`
		Errors []struct {
			Message string `xml:"message,attr"`
		} `xml:"errors>error"`
	}
	if err := xml.Unmarshal(body, &response); err == nil {
		if message := strings.TrimSpace(response.Text); message != "" {
			return message
		}
		if len(response.Errors) > 0 {
			return strings.TrimSpace(response.Errors[0].Message)
		}
	}

	return strings.TrimSpace(html.UnescapeString(string(htmlTagPattern.ReplaceAll(body, nil))))
}

func decodeJSString(value string) string {
	value = strings.ReplaceAll(value, `\/`, `/`)
	value = strings.ReplaceAll(value, `\'`, `'`)
	decoded, err := strconv.Unquote(`"` + value + `"`)
	if err != nil {
		return value
	}
	return html.UnescapeString(decoded)
}
