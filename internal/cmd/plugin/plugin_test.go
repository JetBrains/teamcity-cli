package plugin_test

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginUpload(t *testing.T) {
	archivePath := writePluginArchive(t, "demo-plugin", "1.2.3")
	ts := cmdtest.NewTestServer(t)
	ts.Handle("POST /admin/pluginUpload.html", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, ts.URL, r.Header.Get("Origin"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, filepath.Base(archivePath), r.FormValue("fileName"))

		file, header, err := r.FormFile("file:fileToUpload")
		require.NoError(t, err)
		defer file.Close()
		assert.Equal(t, filepath.Base(archivePath), header.Filename)
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.NotEmpty(t, contents)

		cmdtest.Text(w, `parent.BS.Plugins.UploadPluginDialog.closeAndRefresh('');`)
	})

	output := cmdtest.CaptureOutput(t, ts.Factory, "server", "plugin", "upload", archivePath)
	assert.Contains(t, output, `Uploaded plugin "demo-plugin"`)
}

func TestPluginUploadHotReload(t *testing.T) {
	archivePath := writePluginArchive(t, "demo-plugin", "1.2.3")
	ts := cmdtest.NewTestServer(t)
	ts.Handle("POST /admin/pluginUpload.html", successfulUpload)
	ts.Handle("GET /admin/admin.html", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "plugins", r.URL.Query().Get("item"))
		cmdtest.Text(w, `
BS.Plugins.registerPlugin('another-plugin', '/another', true, '1.0', 'other-uuid');
BS.Plugins.registerPlugin('demo-plugin', '/demo', false, '1.2.2', 'demo-uuid');
BS.Plugins.registerPlugin('demo-plugin', '/demo-new', true, '1.2.3', 'new-uuid');`)
	})
	ts.Handle("POST /admin/plugins.html", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, ts.URL, r.Header.Get("Origin"))
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "setEnabled", r.FormValue("action"))
		assert.Equal(t, "true", r.FormValue("enabled"))
		assert.Equal(t, "true", r.FormValue("reload"))
		assert.Equal(t, "demo-uuid", r.FormValue("uuid"))
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<response>Plugin successfully reloaded</response>`))
	})

	output := cmdtest.CaptureOutput(t, ts.Factory, "server", "plugin", "upload", archivePath, "--hot-reload")
	assert.Contains(t, output, `Uploaded plugin "demo-plugin"`)
	assert.Contains(t, output, `Hot-reloaded plugin "demo-plugin"`)
}

func TestPluginUploadJSON(t *testing.T) {
	archivePath := writePluginArchive(t, "demo-plugin", "1.2.3")
	ts := cmdtest.NewTestServer(t)
	ts.Handle("POST /admin/pluginUpload.html", successfulUpload)

	output := cmdtest.CaptureOutput(t, ts.Factory, "server", "plugin", "upload", archivePath, "--json")
	assert.JSONEq(t, `{
  "file": "demo-plugin.zip",
  "plugin": "demo-plugin",
  "version": "1.2.3",
  "uploaded": true,
  "hot_reloaded": false
}`, output)
}

func TestPluginUploadSurfacesServerError(t *testing.T) {
	archivePath := writePluginArchive(t, "demo-plugin", "1.2.3")
	ts := cmdtest.NewTestServer(t)
	ts.Handle("POST /admin/pluginUpload.html", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.Text(w, `parent.BS.Plugins.UploadPluginDialog.error("Selected archive is invalid");`)
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "server", "plugin", "upload", archivePath)
	assert.ErrorContains(t, err, "Selected archive is invalid")
}

func TestPluginHotReloadSurfacesServerError(t *testing.T) {
	archivePath := writePluginArchive(t, "demo-plugin", "1.2.3")
	ts := cmdtest.NewTestServer(t)
	ts.Handle("POST /admin/pluginUpload.html", successfulUpload)
	ts.Handle("GET /admin/admin.html", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.Text(w, `BS.Plugins.registerPlugin('demo-plugin', '/demo', false, '1.2.2', 'demo-uuid');`)
	})
	ts.Handle("POST /admin/plugins.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<response>Cannot unload plugin: another plugin depends on it</response>`))
	})

	err := cmdtest.CaptureErr(t, ts.Factory, "server", "plugin", "upload", archivePath, "--hot-reload")
	assert.ErrorContains(t, err, "plugin was uploaded but hot reload failed")
	assert.ErrorContains(t, err, "Cannot unload plugin: another plugin depends on it")
}

func TestPluginUploadClassifiesInvalidArchivesAsValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		archivePath func(*testing.T) string
		wantError   string
	}{
		{
			name: "missing archive",
			archivePath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing.zip")
			},
			wantError: "failed to open plugin archive",
		},
		{
			name: "malformed archive",
			archivePath: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "invalid.zip")
				require.NoError(t, os.WriteFile(path, []byte("not a ZIP archive"), 0o600))
				return path
			},
			wantError: "failed to open plugin archive",
		},
		{
			name: "non-plugin archive",
			archivePath: func(t *testing.T) string {
				return writeArchive(t, "readme.txt", "not a plugin")
			},
			wantError: "plugin archive does not contain teamcity-plugin.xml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := cmdtest.CaptureErr(t, cmdtest.NewTestServer(t).Factory, "server", "plugin", "upload", test.archivePath(t), "--json")
			assert.ErrorContains(t, err, test.wantError)
			code, _, _ := output.ClassifyError(err)
			assert.Equal(t, output.ErrCodeValidation, code)
		})
	}
}

func successfulUpload(w http.ResponseWriter, r *http.Request) {
	cmdtest.Text(w, `parent.BS.Plugins.UploadPluginDialog.closeAndRefresh('');`)
}

func writePluginArchive(t *testing.T, name, version string) string {
	t.Helper()
	return writeArchive(t, "teamcity-plugin.xml", `<teamcity-plugin><info><name>`+name+`</name><version>`+version+`</version></info><deployment allow-runtime-reload="true"/></teamcity-plugin>`)
}

func writeArchive(t *testing.T, entryName, contents string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "demo-plugin.zip")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	writer := zip.NewWriter(file)
	entry, err := writer.Create(entryName)
	require.NoError(t, err)
	_, err = entry.Write([]byte(contents))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
	return archivePath
}
