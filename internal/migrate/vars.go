package migrate

import "regexp"

// GitHub Actions expression → TeamCity parameter mapping.

var ghExprMap = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`\$\{\{\s*github\.sha\s*}}`), "%%build.vcs.number%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.ref\s*}}`), "%%teamcity.build.branch%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.ref_name\s*}}`), "%%teamcity.build.branch%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.head_ref\s*}}`), "%%teamcity.build.branch%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.repository\s*}}`), "%%vcsroot.url%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.event_name\s*}}`), "%%teamcity.build.triggeredBy%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.run_id\s*}}`), "%%teamcity.build.id%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.run_number\s*}}`), "%%build.number%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.workspace\s*}}`), "%%teamcity.build.checkoutDir%%"},
	{regexp.MustCompile(`\$\{\{\s*github\.server_url\s*}}`), "%%teamcity.serverUrl%%"},
	{regexp.MustCompile(`\$\{\{\s*runner\.os\s*}}`), "%%teamcity.agent.os.name%%"},
	{regexp.MustCompile(`\$\{\{\s*runner\.arch\s*}}`), "%%teamcity.agent.os.arch%%"},
	{regexp.MustCompile(`\$\{\{\s*runner\.temp\s*}}`), "%%system.teamcity.build.tempDir%%"},
	{regexp.MustCompile(`\$\{\{\s*env\.(\w+)\s*}}`), "%%env.$1%%"},
	{regexp.MustCompile(`\$\{\{\s*secrets\.(\w+)\s*}}`), "%%$1%%"},
}

func MapGHAExpressions(s string) string {
	for _, m := range ghExprMap {
		s = m.pattern.ReplaceAllString(s, m.replace)
	}
	return s
}
