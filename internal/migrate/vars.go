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

// GitLab CI variable mapping.

var mapGitLabVars = NewVarMapper(map[string]string{
	"$CI_COMMIT_SHA":         "%%build.vcs.number%%",
	"$CI_COMMIT_SHORT_SHA":   "%%build.vcs.number%%",
	"$CI_COMMIT_REF_NAME":    "%%teamcity.build.branch%%",
	"$CI_COMMIT_BRANCH":      "%%teamcity.build.branch%%",
	"$CI_COMMIT_TAG":         "%%teamcity.build.branch%%",
	"$CI_PROJECT_DIR":        "%%teamcity.build.checkoutDir%%",
	"$CI_PROJECT_URL":        "%%vcsroot.url%%",
	"$CI_PIPELINE_ID":        "%%teamcity.build.id%%",
	"$CI_PIPELINE_IID":       "%%build.number%%",
	"$CI_JOB_NAME":           "%%teamcity.buildType.id%%",
	"$CI_SERVER_URL":         "%%teamcity.serverUrl%%",
	"$CI_REGISTRY":           "%%docker.registry%%",
	"$CI_REGISTRY_IMAGE":     "%%docker.registry.image%%",
	"${CI_COMMIT_SHA}":       "%%build.vcs.number%%",
	"${CI_COMMIT_SHORT_SHA}": "%%build.vcs.number%%",
	"${CI_COMMIT_REF_NAME}":  "%%teamcity.build.branch%%",
	"${CI_COMMIT_BRANCH}":    "%%teamcity.build.branch%%",
	"${CI_PROJECT_DIR}":      "%%teamcity.build.checkoutDir%%",
})

// Jenkins variable mapping.

var jenkinsEnvRe = regexp.MustCompile(`\$\{env\.(\w+)}`)

var mapJenkinsBaseVars = NewVarMapper(map[string]string{
	"${BUILD_ID}":     "%%teamcity.build.id%%",
	"${BUILD_NUMBER}": "%%build.number%%",
	"${BUILD_URL}":    "%%teamcity.serverUrl%%/viewLog.html?buildId=%%teamcity.build.id%%",
	"${WORKSPACE}":    "%%teamcity.build.checkoutDir%%",
	"${JOB_NAME}":     "%%teamcity.buildType.id%%",
	"${GIT_COMMIT}":   "%%build.vcs.number%%",
	"${GIT_BRANCH}":   "%%teamcity.build.branch%%",
	"${NODE_NAME}":    "%%teamcity.agent.name%%",
	"${JENKINS_URL}":  "%%teamcity.serverUrl%%",
	"$BUILD_ID":       "%%teamcity.build.id%%",
	"$BUILD_NUMBER":   "%%build.number%%",
	"$WORKSPACE":      "%%teamcity.build.checkoutDir%%",
	"$GIT_COMMIT":     "%%build.vcs.number%%",
	"$GIT_BRANCH":     "%%teamcity.build.branch%%",
})

func MapJenkinsVars(s string) string {
	// Rewrite ${env.FOO} → $FOO first so that known env names (e.g. $BUILD_NUMBER)
	// flow through the base mapper below and reach their TeamCity equivalents.
	s = jenkinsEnvRe.ReplaceAllString(s, "$$$1")
	s = mapJenkinsBaseVars(s)
	return s
}

// CircleCI variable mapping.

var mapCircleCIVars = NewVarMapper(map[string]string{
	"$CIRCLE_SHA1":              "%%build.vcs.number%%",
	"$CIRCLE_BRANCH":            "%%teamcity.build.branch%%",
	"$CIRCLE_TAG":               "%%teamcity.build.branch%%",
	"$CIRCLE_BUILD_NUM":         "%%build.number%%",
	"$CIRCLE_BUILD_URL":         "%%teamcity.serverUrl%%/viewLog.html?buildId=%%teamcity.build.id%%",
	"$CIRCLE_WORKING_DIRECTORY": "%%teamcity.build.checkoutDir%%",
	"$CIRCLE_PROJECT_REPONAME":  "%%vcsroot.url%%",
	"${CIRCLE_SHA1}":            "%%build.vcs.number%%",
	"${CIRCLE_BRANCH}":          "%%teamcity.build.branch%%",
	"${CIRCLE_BUILD_NUM}":       "%%build.number%%",
})

// Azure DevOps variable mapping.

var mapAzureVars = NewVarMapper(map[string]string{
	"$(Build.SourceVersion)":            "%%build.vcs.number%%",
	"$(Build.SourceBranch)":             "%%teamcity.build.branch%%",
	"$(Build.SourceBranchName)":         "%%teamcity.build.branch%%",
	"$(Build.BuildId)":                  "%%teamcity.build.id%%",
	"$(Build.BuildNumber)":              "%%build.number%%",
	"$(Build.SourcesDirectory)":         "%%teamcity.build.checkoutDir%%",
	"$(Build.ArtifactStagingDirectory)": "%%teamcity.build.checkoutDir%%/artifacts",
	"$(System.DefaultWorkingDirectory)": "%%teamcity.build.checkoutDir%%",
	"$(Agent.TempDirectory)":            "%%system.teamcity.build.tempDir%%",
	"$(Pipeline.Workspace)":             "%%teamcity.build.checkoutDir%%",
})

// Travis CI variable mapping.

var mapTravisVars = NewVarMapper(map[string]string{
	"$TRAVIS_COMMIT":         "%%build.vcs.number%%",
	"$TRAVIS_BRANCH":         "%%teamcity.build.branch%%",
	"$TRAVIS_TAG":            "%%teamcity.build.branch%%",
	"$TRAVIS_BUILD_NUMBER":   "%%build.number%%",
	"$TRAVIS_BUILD_DIR":      "%%teamcity.build.checkoutDir%%",
	"$TRAVIS_BUILD_WEB_URL":  "%%teamcity.serverUrl%%/viewLog.html?buildId=%%teamcity.build.id%%",
	"${TRAVIS_COMMIT}":       "%%build.vcs.number%%",
	"${TRAVIS_BRANCH}":       "%%teamcity.build.branch%%",
	"${TRAVIS_BUILD_NUMBER}": "%%build.number%%",
	"${TRAVIS_BUILD_DIR}":    "%%teamcity.build.checkoutDir%%",
})

// Bitbucket Pipelines variable mapping.

var mapBitbucketVars = NewVarMapper(map[string]string{
	"$BITBUCKET_COMMIT":         "%%build.vcs.number%%",
	"$BITBUCKET_BRANCH":         "%%teamcity.build.branch%%",
	"$BITBUCKET_TAG":            "%%teamcity.build.branch%%",
	"$BITBUCKET_BUILD_NUMBER":   "%%build.number%%",
	"$BITBUCKET_CLONE_DIR":      "%%teamcity.build.checkoutDir%%",
	"$BITBUCKET_REPO_SLUG":      "%%vcsroot.url%%",
	"${BITBUCKET_COMMIT}":       "%%build.vcs.number%%",
	"${BITBUCKET_BRANCH}":       "%%teamcity.build.branch%%",
	"${BITBUCKET_BUILD_NUMBER}": "%%build.number%%",
	"${BITBUCKET_CLONE_DIR}":    "%%teamcity.build.checkoutDir%%",
})
