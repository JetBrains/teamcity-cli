package migrate

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"
	"unicode"
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
	{"hashicorp/vault-action", "Vault secrets",
		"# TODO: Fetch secrets from HashiCorp Vault\nexport VAULT_ADDR=\"$VAULT_ADDR\"\nvault kv get -format=json secret/data/ci",
		[]string{"Vault → configure VAULT_ADDR and VAULT_TOKEN as TeamCity parameters"}},
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
		// A `repository:` other than the build's own repo is a secondary checkout TC auto-checkout won't fetch.
		if repo := inputs["repository"]; repo != "" {
			r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("actions/checkout fetches a secondary repository %q → add a second TC VCS root with a checkout path or an explicit `git clone` step", repo))
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
			r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("Artifact download %q → ensure upstream job publishes via files-publication with share-with-jobs: true", name))
		}
		if path := inputs["path"]; path != "" {
			r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("Artifact download into %q → TC delivers shared artifacts at the producer's published path; adjust downstream commands or move the files in a script step", path))
		}
		return r
	}

	m["docker/login-action"] = func(name string, inputs map[string]string) StepResult {
		registry := cmp.Or(inputs["registry"], "Docker Hub")
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "Docker login"), ScriptContent: "# Configure Docker registry connection in TeamCity project settings\n" + commentBlock("Registry: "+registry)}},
			ManualTasks: []string{fmt.Sprintf("Docker registry %s → configure Docker connection in TeamCity project settings", registry)}}
	}
	m["docker/build-push-action"] = transformDockerBuild

	m["aws-actions/amazon-ecs-deploy-task-definition"] = func(name string, inputs map[string]string) StepResult {
		cluster, service := requiredInput(inputs, "cluster", "CLUSTER"), requiredInput(inputs, "service", "SERVICE")
		update := fmt.Sprintf("aws ecs update-service --cluster %q --service %q", cluster, service)
		var lines []string
		// The action registers the rendered task definition and deploys that revision; plain --force-new-deployment would roll the old one.
		if td := inputs["task-definition"]; td != "" {
			lines = append(lines,
				"TASK_DEF_ARN=$(aws ecs register-task-definition --cli-input-json "+shellQuote("file://"+td)+" --query taskDefinition.taskDefinitionArn --output text)",
				update+` --task-definition "$TASK_DEF_ARN"`)
		} else {
			lines = append(lines, update+" --force-new-deployment")
		}
		r := Converted([]Step{{Name: cmp.Or(name, "ECS deploy"), ScriptContent: strings.Join(lines, "\n")}})
		r.ManualTasks = []string{"ECS deploy → ensure AWS credentials are configured as TeamCity parameters"}
		return r
	}
	m["FirebaseExtended/action-hosting-deploy"] = func(name string, inputs map[string]string) StepResult {
		var cmd string
		// Non-live channelIds are ephemeral preview deploys; plain `firebase deploy` would update production hosting.
		if ch := inputs["channelId"]; ch != "" && ch != "live" {
			cmd = "firebase hosting:channel:deploy " + shellQuote(ch)
			if tg := inputs["target"]; tg != "" {
				cmd += " --only " + shellQuote(tg)
			}
		} else if tg := inputs["target"]; tg != "" {
			cmd = "firebase deploy --only " + shellQuote("hosting:"+tg)
		} else {
			cmd = "firebase deploy --only hosting"
		}
		if pid := inputs["projectId"]; pid != "" {
			cmd += " --project " + shellQuote(pid)
		}
		r := Converted([]Step{{Name: cmp.Or(name, "Firebase deploy"), ScriptContent: cmd}})
		r.ManualTasks = []string{"Firebase → create TeamCity parameter FIREBASE_TOKEN (type: password)"}
		return r
	}
	m["webfactory/ssh-agent"] = func(name string, inputs map[string]string) StepResult {
		// A step-local ssh-agent dies with the step process, so this maps to TC's build-wide SSH Agent feature.
		return StepResult{Status: StatusSimplified, Note: "ssh-agent → TeamCity SSH Agent build feature",
			ManualTasks: []string{"webfactory/ssh-agent → upload the key with `teamcity project ssh upload` and enable the SSH Agent build feature so every step gets the agent"}}
	}
	m["aws-actions/configure-aws-credentials"] = func(name string, inputs map[string]string) StepResult {
		region := cmp.Or(inputs["aws-region"], "us-east-1")
		// A step-local export dies with the step process in TC, so this maps to job env parameters instead of a script step.
		return StepResult{Status: StatusSimplified, Note: "configure-aws-credentials → job env parameters",
			ManualTasks: []string{fmt.Sprintf("AWS credentials → add env.AWS_ACCESS_KEY_ID / env.AWS_SECRET_ACCESS_KEY under `secrets:` and `env.AWS_DEFAULT_REGION: %q` under the job's `parameters:` so every step sees them", region)}}
	}
	m["azure/webapps-deploy"] = func(name string, inputs map[string]string) StepResult {
		app := requiredInput(inputs, "app-name", "APP_NAME")
		slot := ""
		// slot-name targets a non-production deployment slot; omitting --slot would deploy to production.
		if s := inputs["slot-name"]; s != "" {
			slot = " --slot " + shellQuote(s)
		}
		// `images:` means a container deploy; `az webapp deploy --src-path` (package mode) would ignore the built image.
		if img := strings.TrimSpace(strings.ReplaceAll(inputs["images"], "\n", " ")); img != "" {
			cmd := fmt.Sprintf("az webapp config container set --name %q --container-image-name %s", app, shellQuote(strings.Fields(img)[0])) + slot
			r := Converted([]Step{{Name: cmp.Or(name, "Azure Web App deploy"), ScriptContent: cmd}})
			r.ManualTasks = []string{"azure/webapps-deploy container deploy → pass --resource-group and ensure the registry credentials/connection are configured on the agent"}
			return r
		}
		srcPath := `"${PACKAGE:-.}"`
		if pkg := inputs["package"]; pkg != "" {
			srcPath = shellQuote(pkg)
		}
		return Converted([]Step{{Name: cmp.Or(name, "Azure Web App deploy"), ScriptContent: fmt.Sprintf("az webapp deploy --name %q --src-path %s", app, srcPath) + slot}})
	}
	m["azure/k8s-deploy"] = func(name string, inputs map[string]string) StepResult {
		var cmd strings.Builder
		cmd.WriteString("kubectl apply")
		// `manifests:` is a newline-separated list; split by line so paths with spaces stay one quoted -f operand.
		for f := range strings.SplitSeq(cmp.Or(inputs["manifests"], "k8s/"), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				cmd.WriteString(" -f ")
				cmd.WriteString(shellQuote(f))
			}
		}
		r := Converted([]Step{{Name: cmp.Or(name, "Kubernetes deploy"), ScriptContent: cmd.String()}})
		// The action substitutes `images:` into the manifests before applying; plain kubectl apply would deploy the manifest's stale image.
		if imgs := strings.Join(strings.Fields(strings.ReplaceAll(inputs["images"], "\n", " ")), " "); imgs != "" {
			r.ManualTasks = []string{fmt.Sprintf("k8s-deploy substitutes images (%s) into the manifests → run `kubectl set image` (or `kustomize edit set image`) before apply so the built image is deployed", imgs)}
		}
		return r
	}
	m["azure/k8s-set-context"] = func(name string, inputs map[string]string) StepResult {
		return Converted([]Step{{Name: cmp.Or(name, "K8s set context"), ScriptContent: fmt.Sprintf("az aks get-credentials --resource-group %q --name %q", requiredInput(inputs, "resource-group", "RESOURCE_GROUP"), requiredInput(inputs, "cluster-name", "CLUSTER_NAME"))}})
	}
	m["google-github-actions/deploy-cloudrun"] = func(name string, inputs map[string]string) StepResult {
		region := "\"${REGION:-us-central1}\""
		if r := inputs["region"]; r != "" {
			region = shellQuote(r)
		}
		// `source:` deploys from a directory (build-from-source) and is mutually exclusive with image.
		deployTarget := fmt.Sprintf("--image %q", requiredInput(inputs, "image", "IMAGE"))
		if src := inputs["source"]; src != "" {
			deployTarget = "--source " + shellQuote(src)
		}
		cmd := fmt.Sprintf("gcloud run deploy %q %s --region %s", requiredInput(inputs, "service", "SERVICE"), deployTarget, region)
		if pid := inputs["project_id"]; pid != "" {
			cmd += " --project " + shellQuote(pid)
		}
		// no_traffic withholds traffic from the new revision (canary/manual rollout); without --no-traffic the deploy shifts production traffic immediately.
		if inputs["no_traffic"] == "true" {
			cmd += " --no-traffic"
		}
		r := Converted([]Step{{Name: cmp.Or(name, "Cloud Run deploy"), ScriptContent: cmd}})
		if inputs["revision_traffic"] != "" || inputs["tag_traffic"] != "" {
			r.ManualTasks = []string{"Cloud Run traffic split → run `gcloud run services update-traffic` for the revision_traffic/tag_traffic the action managed"}
		}
		return r
	}
	m["aws-actions/amazon-ecs-render-task-definition"] = func(name string, inputs map[string]string) StepResult {
		// container-name selects which container's image to replace in multi-container task definitions.
		selector := ".containerDefinitions[0].image"
		if cn := inputs["container-name"]; cn != "" {
			selector = fmt.Sprintf("(.containerDefinitions[] | select(.name == %q) | .image)", cn)
		}
		return Converted([]Step{{Name: cmp.Or(name, "ECS render task def"), ScriptContent: fmt.Sprintf("jq '%s = %q' %s > new-task-def.json", selector, requiredInput(inputs, "image", "IMAGE"), shellQuote(cmp.Or(inputs["task-definition"], "task-definition.json")))}})
	}
	m["appleboy/scp-action"] = func(name string, inputs map[string]string) StepResult {
		port := ""
		if p := inputs["port"]; p != "" {
			port = "-P " + shellQuote(p) + " "
		}
		// source and host are comma-separated lists in the action; emit operands per source and one command per host.
		var sources []string
		for src := range strings.SplitSeq(cmp.Or(inputs["source"], "."), ",") {
			if src = strings.TrimSpace(src); src != "" {
				sources = append(sources, quoteReleaseAsset(src))
			}
		}
		var cmds []string
		for h := range strings.SplitSeq(requiredInput(inputs, "host", "DEPLOY_HOST"), ",") {
			if h = strings.TrimSpace(h); h == "" {
				continue
			}
			if u := inputs["username"]; u != "" {
				h = u + "@" + h
			}
			// host: stays unquoted so the ${DEPLOY_HOST:?} fallback expands; the target is quoted into the same operand.
			cmds = append(cmds, fmt.Sprintf("scp %s-r %s %s:%s", port, strings.Join(sources, " "), h, shellQuote(cmp.Or(inputs["target"], "~/"))))
		}
		r := Converted([]Step{{Name: cmp.Or(name, "SCP deploy"), ScriptContent: strings.Join(cmds, "\n")}})
		if inputs["key"] != "" || inputs["password"] != "" {
			r.ManualTasks = []string{"scp-action key/password auth → upload the SSH key with `teamcity project ssh upload` and enable the SSH Agent build feature (passwords cannot be inlined)"}
		}
		return r
	}
	m["SamKirkland/FTP-Deploy-Action"] = func(name string, inputs map[string]string) StepResult {
		return StepResult{Status: StatusConverted,
			Steps:       []Step{{Name: cmp.Or(name, "FTP deploy"), ScriptContent: fmt.Sprintf("lftp -c \"open -u $FTP_USER,$FTP_PASSWORD %s; mirror -R %s %s\"", requiredInput(inputs, "server", "FTP_SERVER"), shellQuote(cmp.Or(inputs["local-dir"], "./")), shellQuote(cmp.Or(inputs["server-dir"], "/")))}},
			ManualTasks: []string{"FTP credentials → create TeamCity parameters FTP_USER, FTP_PASSWORD (type: password)"}}
	}
	m["ncipollo/release-action"] = func(name string, inputs map[string]string) StepResult {
		tag := cmp.Or(inputs["tag"], "%teamcity.build.branch%")
		var cmd strings.Builder
		cmd.WriteString("gh release create " + shellQuote(tag) + " --generate-notes" + ghReleaseStateFlags(inputs))
		if body := inputs["body"]; body != "" {
			cmd.WriteString(" --notes " + shellQuote(body))
		}
		// `artifacts:` is a comma-delimited path/glob list the action uploads to the release.
		for a := range strings.SplitSeq(cmp.Or(inputs["artifacts"], inputs["artifact"]), ",") {
			if a = strings.TrimSpace(a); a != "" {
				cmd.WriteString(" " + quoteReleaseAsset(a))
			}
		}
		r := Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd.String()}})
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
		r := Converted([]Step{{Name: cmp.Or(name, "Create release"), ScriptContent: "gh release create " + shellQuote(cmp.Or(inputs["tag_name"], "%teamcity.build.branch%")) + " --generate-notes" + ghReleaseStateFlags(inputs)}})
		r.ManualTasks = ghReleaseAuthNote
		return r
	}
	m["softprops/action-gh-release"] = func(name string, inputs map[string]string) StepResult {
		var cmd strings.Builder
		cmd.WriteString("gh release create " + shellQuote(cmp.Or(inputs["tag_name"], "%teamcity.build.branch%")) + " --generate-notes")
		cmd.WriteString(ghReleaseStateFlags(inputs))
		// `files:` is a newline-separated glob list; keep one asset per line so a multiline value can't become a new shell line.
		for f := range strings.SplitSeq(inputs["files"], "\n") {
			if f = strings.TrimSpace(f); f != "" {
				cmd.WriteString(" ")
				cmd.WriteString(quoteReleaseAsset(f))
			}
		}
		r := Converted([]Step{{Name: cmp.Or(name, "GitHub release"), ScriptContent: cmd.String()}})
		r.ManualTasks = ghReleaseAuthNote
		return r
	}
	m["peaceiris/actions-gh-pages"] = func(name string, inputs map[string]string) StepResult {
		dir := cmp.Or(inputs["publish_dir"], "./public")
		return Converted([]Step{{Name: cmp.Or(name, "Deploy to GitHub Pages"), ScriptContent: ghPagesScript(cmp.Or(inputs["publish_branch"], "gh-pages"), dir)}})
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

// ghReleaseStateFlags propagates draft/prerelease inputs so migrated releases keep their visibility.
func ghReleaseStateFlags(inputs map[string]string) string {
	var flags string
	if inputs["draft"] == "true" {
		flags += " --draft"
	}
	if inputs["prerelease"] == "true" {
		flags += " --prerelease"
	}
	return flags
}

// releaseAssetSafe matches plain path/glob tokens that may stay unquoted so the shell expands them.
var releaseAssetSafe = regexp.MustCompile(`^[A-Za-z0-9._/*?\[\]-]+$`)

// quoteReleaseAsset keeps glob patterns expandable but quotes anything carrying other shell metacharacters.
func quoteReleaseAsset(f string) string {
	if releaseAssetSafe.MatchString(f) {
		return f
	}
	return shellQuote(f)
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
	var tagExpr bool
	// An unresolved ${{ }} expression (e.g. from docker/metadata-action) has no TeamCity equivalent; tokenizing it would emit broken -t operands.
	if strings.Contains(tags, "${{") {
		tagExpr = true
		lines = append(lines, `IMAGE="${IMAGE:?Set IMAGE variable}"`)
	} else if tagList := strings.FieldsFunc(tags, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }); len(tagList) > 0 {
		// `tags:` is documented as list/CSV, so split on commas as well as whitespace.
		lines = append(lines, "IMAGE="+shellQuote(tagList[0]))
		extraTags = tagList[1:]
	} else {
		lines = append(lines, `IMAGE="${IMAGE:?Set IMAGE variable}"`)
	}

	// Multi-platform manifest lists require buildx; plain docker build only produces a single local image.
	// `platforms:` is list/CSV like tags, so normalize newlines and spaces to the comma form buildx expects.
	platforms := strings.Join(strings.FieldsFunc(inputs["platforms"], func(r rune) bool { return r == ',' || unicode.IsSpace(r) }), ",")
	pushViaBuildx := platforms != "" && inputs["push"] == "true"
	var buildCmd strings.Builder
	if platforms != "" {
		buildCmd.WriteString("docker buildx build --platform " + shellQuote(platforms))
	} else {
		buildCmd.WriteString("docker build")
	}
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
	if target := inputs["target"]; target != "" {
		buildCmd.WriteString(" --target " + shellQuote(target))
	}
	buildCmd.WriteString(` -t "$IMAGE"`)
	// The action publishes every tag, so emit each extra as -t plus its own push.
	for _, t := range extraTags {
		buildCmd.WriteString(" -t " + shellQuote(t))
	}
	if pushViaBuildx {
		buildCmd.WriteString(" --push")
	}
	// buildx leaves results in the build cache; --load keeps them visible to later docker run steps.
	if platforms != "" && inputs["load"] == "true" {
		buildCmd.WriteString(" --load")
	}
	buildCmd.WriteString(" " + shellQuote(context))
	lines = append(lines, buildCmd.String())

	if inputs["push"] == "true" && !pushViaBuildx {
		lines = append(lines, `docker push "$IMAGE"`)
		for _, t := range extraTags {
			lines = append(lines, "docker push "+shellQuote(t))
		}
	}
	r := Converted([]Step{{Name: cmp.Or(name, "Docker build and push"), ScriptContent: strings.Join(lines, "\n")}})
	if platforms != "" {
		r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("Multi-platform build (%s) → ensure the agent has docker buildx and QEMU configured", platforms))
	}
	if tagExpr {
		r.ManualTasks = append(r.ManualTasks, fmt.Sprintf("Docker tags from expression %q → set the IMAGE variable (e.g. a TC parameter) to the desired tag(s)", condense(tags)))
	}
	return r
}

// ghPagesScript stages the site in a temp dir, then clears the orphan index/worktree so only the folder is published.
func ghPagesScript(branch, folder string) string {
	b, f := shellQuote(branch), shellQuote(folder)
	lines := []string{
		`git config user.name "TeamCity"`,
		`git config user.email "teamcity@localhost"`,
		`SITE_TMP=$(mktemp -d)`,
		// The dir/. form copies contents including dotfiles (.nojekyll etc.), which * would skip.
		fmt.Sprintf(`cp -r %s/. "$SITE_TMP"/`, f),
		// Publishing the repo root would stage .git and clobber the orphan branch on copy-back.
		`rm -rf "$SITE_TMP"/.git`,
		"git checkout --orphan " + b,
		"git rm -rfq .",
		"git clean -fdx",
		`cp -r "$SITE_TMP"/. .`,
		"git add .",
		`git commit -m "Deploy"`,
		fmt.Sprintf("git push origin %s --force", b),
	}
	return strings.Join(lines, "\n")
}

// shellQuote single-quotes s so it is one inert shell word — unlike double quotes, $(), backticks, and $vars do not expand.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
