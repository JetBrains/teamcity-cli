package migrate

import (
	"cmp"
	"fmt"
	"strings"
)

var skippedActions = []struct{ action, note string }{
	{"actions/setup-node", "setup-node (use agent tooling or nvm)"},
	{"actions/setup-java", "setup-java (use agent JDK)"},
	{"actions/setup-go", "setup-go (use agent Go installation)"},
	{"actions/setup-python", "setup-python (use agent Python)"},
	{"actions/setup-dotnet", "setup-dotnet (use agent .NET SDK)"},
	{"actions/setup-ruby", "setup-ruby (use agent Ruby or rbenv)"},
	{"actions/setup-php", "setup-php (use agent PHP)"},
	{"shivammathur/setup-php", "setup-php (use agent PHP)"},
	{"julia-actions/setup-julia", "setup-julia (use agent Julia)"},
	{"actions/setup-elixir", "setup-elixir (use agent Elixir)"},
	{"actions/setup-haskell", "setup-haskell (use agent GHC)"},
	{"erlef/setup-beam", "setup-beam (use agent Erlang/Elixir)"},
	{"ruby/setup-ruby", "setup-ruby (use agent Ruby or rbenv)"},
	{"haskell-actions/setup", "setup-haskell (use agent GHC)"},
	{"gradle/actions/setup-gradle", "setup-gradle (Gradle wrapper used directly)"},
	{"gradle/gradle-build-action", "setup-gradle (Gradle wrapper used directly)"},
	{"ATiltedTree/setup-rust", "setup-rust (use agent Rust toolchain)"},
	{"dtolnay/rust-toolchain", "rust-toolchain (use agent Rust toolchain)"},
	{"actions-rust-lang/setup-rust-toolchain", "setup-rust-toolchain (use agent Rust)"},
	{"subosito/flutter-action", "flutter-action (use agent Flutter SDK)"},
	{"dart-lang/setup-dart", "setup-dart (use agent Dart SDK)"},
	{"swift-actions/setup-swift", "setup-swift (use agent Swift toolchain)"},
	{"pnpm/action-setup", "pnpm-setup (use agent pnpm)"},
	{"oven-sh/setup-bun", "setup-bun (use agent Bun)"},
	{"denoland/setup-deno", "setup-deno (use agent Deno)"},
	{"hashicorp/setup-terraform", "setup-terraform (use agent Terraform installation)"},
	{"google-github-actions/setup-gcloud", "setup-gcloud (use agent gcloud installation)"},
	{"docker/setup-buildx-action", "setup-buildx (configure on agent)"},
	{"docker/setup-qemu-action", "setup-qemu (configure on agent for multi-arch builds)"},
	{"dorny/test-reporter", "test-reporter → TeamCity has built-in XML test report processing"},
	{"mikepenz/action-junit-report", "junit-report → TeamCity has built-in JUnit report processing"},
	{"EnricoMi/publish-unit-test-result-action", "publish-unit-test-result → TeamCity has built-in test report processing"},
	{"actions/configure-pages", "configure-pages (GitHub Pages metadata, no-op for TeamCity)"},
}

var scriptActions = []struct{ action, script, name string }{
	{"codecov/codecov-action", "curl -Os https://cli.codecov.io/latest/linux/codecov && chmod +x codecov && ./codecov", "Codecov upload"},
	{"sonarsource/sonarqube-scan-action", "# TODO: Configure SonarQube connection in TeamCity project settings\nsonar-scanner", "SonarQube scan"},
	{"sonarsource/sonarcloud-github-action", "# TODO: Configure SonarCloud connection in TeamCity project settings\nsonar-scanner", "SonarCloud scan"},
	{"pre-commit/action", "pre-commit run --all-files", "Pre-commit checks"},
	{"super-linter/super-linter", "docker run --rm -v \"$(pwd)\":/tmp/lint ghcr.io/super-linter/super-linter:latest", "Super-Linter"},
	{"helm/kind-action", "kind create cluster", "Create kind cluster"},
	{"peter-evans/create-pull-request", "gh pr create --fill", "Create pull request"},
	{"cypress-io/github-action", "npx cypress run", "Cypress E2E tests"},
	{"snyk/actions", "snyk test", "Snyk security scan"},
	{"wagoid/commitlint-github-action", "npx commitlint --from HEAD~1", "Commitlint"},
	{"treosh/lighthouse-ci-action", "npx @lhci/cli autorun", "Lighthouse CI"},
	{"ad-m/github-push-action", "git push origin HEAD", "Git push"},
	{"stefanzweifel/git-auto-commit-action", "git add -A && git diff --cached --quiet || git commit -m \"Auto-commit\" && git push", "Git auto-commit"},
	{"EndBug/add-and-commit", "git add -A && git diff --cached --quiet || git commit -m \"Auto-commit\" && git push", "Git add and commit"},
	{"JetBrains/qodana-action", "# TeamCity has native Qodana integration\n# Add Qodana build feature in TeamCity project settings\n# Or run via Docker:\ndocker run --rm -v \"$(pwd)\":/data/project jetbrains/qodana-jvm-community:latest", "Qodana"},
}

// manualActions convert to a fixed script step plus manual-setup notes (typically credentials to recreate as TC parameters).
var manualActions = []struct {
	action, name, script string
	manual               []string
}{
	{"docker/metadata-action", "Docker metadata",
		"# TODO: docker/metadata-action generates tags/labels from git context\necho 'Set IMAGE tag from TeamCity build parameters'",
		[]string{"docker/metadata-action → use TeamCity build parameters for image tags (%build.vcs.number%, %build.number%)"}},
	{"azure/login", "Azure login",
		"az login --service-principal -u \"$AZURE_CLIENT_ID\" -p \"$AZURE_CLIENT_SECRET\" --tenant \"$AZURE_TENANT_ID\"",
		[]string{"Azure credentials → create TeamCity parameters: AZURE_CLIENT_ID, AZURE_CLIENT_SECRET (password), AZURE_TENANT_ID"}},
	{"google-github-actions/auth", "Google Cloud auth",
		"gcloud auth activate-service-account --key-file=\"$GOOGLE_APPLICATION_CREDENTIALS\"",
		[]string{"GCP credentials → configure service account key as TeamCity secure parameter"}},
	{"aws-actions/amazon-ecr-login", "ECR login",
		"aws ecr get-login-password --region \"$AWS_DEFAULT_REGION\" | docker login --username AWS --password-stdin \"$ECR_REGISTRY\"",
		[]string{"ECR login → ensure AWS credentials and ECR_REGISTRY parameter are configured"}},
	{"webfactory/ssh-agent", "SSH agent",
		"eval \"$(ssh-agent -s)\" && echo \"$SSH_PRIVATE_KEY\" | ssh-add -",
		[]string{"SSH private key → create TeamCity parameter SSH_PRIVATE_KEY (type: password)"}},
	{"hashicorp/vault-action", "Vault secrets",
		"# TODO: Fetch secrets from HashiCorp Vault\nexport VAULT_ADDR=\"$VAULT_ADDR\"\nvault kv get -format=json secret/data/ci",
		[]string{"Vault → configure VAULT_ADDR and VAULT_TOKEN as TeamCity parameters"}},
	{"FirebaseExtended/action-hosting-deploy", "Firebase deploy",
		"firebase deploy --only hosting",
		[]string{"Firebase → create TeamCity parameter FIREBASE_TOKEN (type: password)"}},
	{"tj-actions/changed-files", "Get changed files",
		"CHANGED_FILES=$(git diff --name-only HEAD~1)\necho \"$CHANGED_FILES\"",
		[]string{"tj-actions/changed-files → TeamCity provides %teamcity.build.changedFiles.file%"}},
	{"slackapi/slack-github-action", "Slack notification",
		"# TeamCity has built-in Slack integration\n# Configure in: Project Settings → Build Features → Slack Notifier",
		[]string{"Slack notification → configure TeamCity Slack Notifier"}},
	{"JS-DevTools/npm-publish", "npm publish",
		"npm publish",
		[]string{"npm publish token → create TeamCity parameter NPM_TOKEN (type: password)"}},
	{"pypa/gh-action-pypi-publish", "PyPI publish",
		"pip install twine && twine upload dist/*",
		[]string{"PyPI credentials → create TeamCity parameters TWINE_USERNAME, TWINE_PASSWORD (type: password)"}},
	{"aws-actions/amazon-ecs-deploy-task-definition", "ECS deploy",
		"aws ecs update-service --cluster \"${CLUSTER:?Set CLUSTER}\" --service \"${SERVICE:?Set SERVICE}\" --force-new-deployment",
		[]string{"ECS deploy → create TeamCity parameters CLUSTER and SERVICE for the target cluster/service"}},
}

var unsupportedActions = []struct {
	action, reason string
	manual         []string
}{
	{"github/codeql-action/init", "CodeQL init → use Qodana or third-party SAST in TeamCity",
		[]string{"CodeQL → consider Qodana build feature for static analysis in TeamCity"}},
	{"github/codeql-action/analyze", "CodeQL analyze → use Qodana or third-party SAST in TeamCity", nil},
	{"github/codeql-action/autobuild", "CodeQL autobuild → not needed with Qodana", nil},
	{"actions/deploy-pages", "deploy-pages → performs the actual Pages deployment; recreate as a deploy step or keep the Pages workflow",
		[]string{"actions/deploy-pages → deploy the site from TeamCity (see peaceiris/JamesIves conversions) or keep GitHub Pages deployment on GitHub"}},
	{"actions/labeler", "actions/labeler → GitHub-specific; no TeamCity equivalent", nil},
	{"actions/stale", "actions/stale → GitHub-specific; no TeamCity equivalent", nil},
	{"actions/first-interaction", "actions/first-interaction → GitHub-specific", nil},
	{"ossf/scorecard-action", "scorecard-action → GitHub-specific security scoring", nil},
	{"peter-evans/repository-dispatch", "repository-dispatch → GitHub-specific event system", nil},
	{"hmarr/auto-approve-action", "auto-approve → GitHub PR-specific", nil},
	{"pascalgn/automerge-action", "automerge → GitHub PR-specific", nil},
	{"dessant/lock-threads", "lock-threads → GitHub-specific issue management", nil},
	{"dorny/paths-filter", "paths-filter → use TeamCity VCS trigger rules",
		[]string{"dorny/paths-filter → configure VCS trigger rules with path patterns in TeamCity"}},
}

func initActionRegistry() map[string]actionTransformer {
	m := map[string]actionTransformer{}

	for _, a := range skippedActions {
		action, note := a.action, a.note
		m[a.action] = func(_ string, inputs map[string]string) StepResult {
			r := StepResult{Status: StatusSimplified, Note: note}
			for _, k := range SortedKeys(inputs) {
				pinKey := strings.HasSuffix(k, "-version") || strings.HasSuffix(k, "_version") || k == "version" || k == "toolchain"
				if pinKey && inputs[k] != "" {
					r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("%s pins %s %s → ensure the agent provides that version", action, k, inputs[k]))
				}
			}
			return r
		}
	}
	for _, a := range scriptActions {
		script, name := a.script, a.name
		m[a.action] = func(stepName string, _ map[string]string) StepResult {
			return Converted([]Step{{Name: cmp.Or(stepName, name), ScriptContent: script}})
		}
	}
	for _, a := range unsupportedActions {
		reason, manual := a.reason, a.manual
		m[a.action] = func(_ string, _ map[string]string) StepResult {
			return StepResult{Status: StatusUnsupported, Note: reason, ManualTasks: manual}
		}
	}
	for _, a := range manualActions {
		name, script, manual := a.name, a.script, a.manual
		m[a.action] = func(stepName string, _ map[string]string) StepResult {
			return StepResult{Status: StatusConverted,
				Steps:       []Step{{Name: cmp.Or(stepName, name), ScriptContent: script}},
				ManualTasks: manual}
		}
	}

	m["actions/checkout"] = func(_ string, inputs map[string]string) StepResult {
		r := StepResult{Status: StatusSimplified, Note: "checkout (TeamCity VCS checkout is automatic)"}
		// Non-default checkout options change what lands on disk; TC auto-checkout won't replicate them.
		for _, k := range []string{"path", "submodules", "lfs", "fetch-depth", "ref"} {
			if v := inputs[k]; v != "" && v != "false" {
				r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("actions/checkout sets %s: %s → configure checkout rules / submodules / fetch depth on the TC VCS root", k, v))
			}
		}
		return r
	}

	m["actions/cache"] = func(_ string, _ map[string]string) StepResult {
		return StepResult{Status: StatusSimplified, Note: "cache → enable-dependency-cache: true", EnableDependencyCache: true}
	}

	m["actions/upload-artifact"] = func(_ string, inputs map[string]string) StepResult {
		var arts []FilePublication
		// `path:` is a newline-separated list of files/globs; emit one publication entry each.
		for p := range strings.SplitSeq(cmp.Or(inputs["path"], "**/*"), "\n") {
			if p = strings.TrimSpace(p); p != "" {
				arts = append(arts, FilePublication{Path: p, ShareWithJobs: true, PublishArtifact: true})
			}
		}
		return StepResult{Status: StatusSimplified, Note: "upload-artifact → files-publication", Artifacts: arts}
	}
	m["actions/download-artifact"] = func(_ string, inputs map[string]string) StepResult {
		r := StepResult{Status: StatusSimplified, Note: "download-artifact → files-publication with share-with-jobs"}
		if name := inputs["name"]; name != "" {
			r.ManualTasks = []string{fmt.Sprintf("Artifact download %q → ensure upstream job publishes via files-publication with share-with-jobs: true", name)}
		}
		return r
	}

	m["docker/login-action"] = func(name string, inputs map[string]string) StepResult {
		registry := cmp.Or(inputs["registry"], "Docker Hub")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Docker login"), ScriptContent: "# Configure Docker registry connection in TeamCity project settings\n# Registry: " + registry}},
			ManualTasks: []string{fmt.Sprintf("Docker registry %s → configure Docker connection in TeamCity project settings", registry)}}
	}
	m["docker/build-push-action"] = transformDockerBuild

	m["aws-actions/configure-aws-credentials"] = func(name string, inputs map[string]string) StepResult {
		region := cmp.Or(inputs["aws-region"], "us-east-1")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Configure AWS credentials"), ScriptContent: "export AWS_ACCESS_KEY_ID=\"$AWS_ACCESS_KEY_ID\"\nexport AWS_SECRET_ACCESS_KEY=\"$AWS_SECRET_ACCESS_KEY\"\nexport AWS_DEFAULT_REGION=\"" + region + "\""}},
			ManualTasks: []string{"AWS credentials → create TeamCity parameters: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (type: password)"}}
	}
	m["azure/webapps-deploy"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Azure Web App deploy"), ScriptContent: fmt.Sprintf("az webapp deploy --name %q --src-path \"${PACKAGE:-.}\"", requiredInput(inputs, "app-name", "APP_NAME"))}})
	}
	m["azure/k8s-deploy"] = func(name string, inputs map[string]string) StepResult {
		var cmd strings.Builder
		cmd.WriteString("kubectl apply")
		// `manifests:` is a newline-separated list; emit one -f per entry so a multiline value can't spill onto a new shell line.
		for f := range strings.FieldsSeq(cmp.Or(inputs["manifests"], "k8s/")) {
			cmd.WriteString(" -f ")
			cmd.WriteString(f)
		}
		return Converted([]Step{{Name: cmp.Or(name, "Kubernetes deploy"), ScriptContent: cmd.String()}})
	}
	m["azure/k8s-set-context"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "K8s set context"), ScriptContent: fmt.Sprintf("az aks get-credentials --resource-group %q --name %q", requiredInput(inputs, "resource-group", "RESOURCE_GROUP"), requiredInput(inputs, "cluster-name", "CLUSTER_NAME"))}})
	}
	m["google-github-actions/deploy-cloudrun"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Cloud Run deploy"), ScriptContent: fmt.Sprintf("gcloud run deploy %q --image %q --region \"${REGION:-us-central1}\"", requiredInput(inputs, "service", "SERVICE"), requiredInput(inputs, "image", "IMAGE"))}})
	}
	m["aws-actions/amazon-ecs-render-task-definition"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "ECS render task def"), ScriptContent: fmt.Sprintf("jq '.containerDefinitions[0].image = %q' %s > new-task-def.json", requiredInput(inputs, "image", "IMAGE"), cmp.Or(inputs["task-definition"], "task-definition.json"))}})
	}
	m["appleboy/scp-action"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "SCP deploy"), ScriptContent: fmt.Sprintf("scp -r %s %s:%s", cmp.Or(inputs["source"], "."), requiredInput(inputs, "host", "DEPLOY_HOST"), cmp.Or(inputs["target"], "~/"))}})
	}
	m["SamKirkland/FTP-Deploy-Action"] = func(name string, inputs map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "FTP deploy"), ScriptContent: fmt.Sprintf("lftp -c \"open -u $FTP_USER,$FTP_PASSWORD %s; mirror -R %s %s\"", requiredInput(inputs, "server", "FTP_SERVER"), cmp.Or(inputs["local-dir"], "./"), cmp.Or(inputs["server-dir"], "/"))}},
			ManualTasks: []string{"FTP credentials → create TeamCity parameters FTP_USER, FTP_PASSWORD (type: password)"}}
	}
	m["ncipollo/release-action"] = func(name string, inputs map[string]string) StepResult {
		tag := cmp.Or(inputs["tag"], "%teamcity.build.branch%")
		cmd := fmt.Sprintf("gh release create %q --generate-notes", tag)
		if body := inputs["body"]; body != "" {
			cmd += " --notes " + shellQuote(body)
		}
		r := Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd}})
		r.ManualTasks = ghReleaseAuthNote
		return r
	}
	m["golangci/golangci-lint-action"] = func(name string, inputs map[string]string) StepResult {
		cmd := "golangci-lint run"
		if args := inputs["args"]; args != "" {
			cmd += " " + args
		}
		r := Converted([]Step{{Name: cmp.Or(name, "golangci-lint"), ScriptContent: cmd}})
		if v := inputs["version"]; v != "" {
			r.ManualTasks = []string{fmt.Sprintf("golangci-lint version %s → ensure installed on agent", v)}
		}
		return r
	}

	m["actions/create-release"] = func(name string, inputs map[string]string) StepResult {
		r := Converted([]Step{{Name: cmp.Or(name, "Create release"), ScriptContent: fmt.Sprintf("gh release create %q --generate-notes", cmp.Or(inputs["tag_name"], "%teamcity.build.branch%"))}})
		r.ManualTasks = ghReleaseAuthNote
		return r
	}
	m["softprops/action-gh-release"] = func(name string, inputs map[string]string) StepResult {
		var cmd strings.Builder
		fmt.Fprintf(&cmd, "gh release create %q --generate-notes", cmp.Or(inputs["tag_name"], "%teamcity.build.branch%"))
		// `files:` is a whitespace/newline-separated glob list; split per token (left unquoted so globs still expand).
		for f := range strings.FieldsSeq(inputs["files"]) {
			cmd.WriteString(" ")
			cmd.WriteString(f)
		}
		r := Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd.String()}})
		r.ManualTasks = ghReleaseAuthNote
		return r
	}
	m["peaceiris/actions-gh-pages"] = func(name string, inputs map[string]string) StepResult {
		dir := cmp.Or(inputs["publish_dir"], "./public")
		return Converted([]Step{{Name: cmp.Or(name, "Deploy to GitHub Pages"), ScriptContent: ghPagesScript("gh-pages", dir)}})
	}
	m["JamesIves/github-pages-deploy-action"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Deploy to GitHub Pages"), ScriptContent: ghPagesScript(cmp.Or(inputs["branch"], "gh-pages"), cmp.Or(inputs["folder"], "."))}})
	}

	m["actions/github-script"] = func(name string, inputs map[string]string) StepResult {
		script := cmp.Or(inputs["script"], "echo 'TODO: convert GitHub Script to shell commands'")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "GitHub script"), ScriptContent: "# TODO: This was a GitHub Script action using Octokit\n" + commentBlock(script)}},
			ManualTasks: []string{"actions/github-script → convert Octokit JS to shell/curl commands"}}
	}
	m["aquasecurity/trivy-action"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "Trivy security scan"), ScriptContent: fmt.Sprintf("trivy %s %s", cmp.Or(inputs["scan-type"], "fs"), cmp.Or(inputs["image-ref"], "."))}})
	}

	return m
}

// ghReleaseAuthNote flags that gh needs a token on the agent, which GHA injected automatically.
var ghReleaseAuthNote = []string{"gh release create → set GH_TOKEN as a TC password parameter (GHA injected GITHUB_TOKEN automatically)"}

// requiredInput falls back to a ${VAR:?} shell guard so a missing action input fails the step instead of emitting empty arguments.
func requiredInput(inputs map[string]string, key, envVar string) string {
	return cmp.Or(inputs[key], fmt.Sprintf("${%s:?Set %s}", envVar, envVar))
}

func transformDockerBuild(name string, inputs map[string]string) StepResult {
	context := cmp.Or(inputs["context"], ".")
	tags := inputs["tags"]
	file := inputs["file"]

	var lines []string
	if file != "" && file != "Dockerfile" {
		lines = append(lines, "DOCKERFILE="+shellQuote(file))
	}
	var extraTags []string
	if tagList := strings.Fields(tags); len(tagList) > 0 {
		lines = append(lines, "IMAGE="+shellQuote(tagList[0]))
		extraTags = tagList[1:]
	} else {
		lines = append(lines, `IMAGE="${IMAGE:?Set IMAGE variable}"`)
	}

	var buildCmd strings.Builder
	buildCmd.WriteString("docker build")
	if file != "" && file != "Dockerfile" {
		buildCmd.WriteString(` -f "$DOCKERFILE"`)
	}
	if buildArgs := inputs["build-args"]; buildArgs != "" {
		for arg := range strings.SplitSeq(strings.TrimSpace(buildArgs), "\n") {
			if arg = strings.TrimSpace(arg); arg != "" {
				buildCmd.WriteString(" --build-arg " + shellQuote(arg))
			}
		}
	}
	buildCmd.WriteString(` -t "$IMAGE"`)
	// The action publishes every tag, so emit each extra as -t plus its own push.
	for _, t := range extraTags {
		buildCmd.WriteString(" -t " + shellQuote(t))
	}
	buildCmd.WriteString(" " + shellQuote(context))
	lines = append(lines, buildCmd.String())

	if inputs["push"] == "true" {
		lines = append(lines, `docker push "$IMAGE"`)
		for _, t := range extraTags {
			lines = append(lines, "docker push "+shellQuote(t))
		}
	}
	return Converted([]Step{{Name: cmp.Or(name, "Docker build and push"), ScriptContent: strings.Join(lines, "\n")}})
}

func ghPagesScript(branch, folder string) string {
	b, f := shellQuote(branch), shellQuote(folder)
	return fmt.Sprintf("git config user.name \"TeamCity\"\ngit config user.email \"teamcity@localhost\"\ngit checkout --orphan %s\ncp -r %s/* .\ngit add .\ngit commit -m \"Deploy\"\ngit push origin %s --force", b, f, b)
}

// shellQuote single-quotes s so it is one inert shell word — unlike double quotes, $(), backticks, and $vars do not expand.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
