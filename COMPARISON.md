# Feature Comparison: GitHub CLI vs TeamCity CLI

Quick reference guide comparing `gh` commands with `tc` equivalents and gaps.

---

## ✅ Feature Parity - What Works Well

| GitHub CLI | TeamCity CLI | Notes |
|-----------|--------------|-------|
| `gh auth login` | `tc auth login` | ✅ Full parity with multi-server support |
| `gh auth status` | `tc auth status` | ✅ Full parity |
| `gh run list` | `tc run list` | ✅ Similar filtering capabilities |
| `gh run view <id>` | `tc run view <id>` | ✅ Full parity |
| `gh run watch <id>` | `tc run watch <id>` | ✅ Full parity with real-time updates |
| `gh run download <id>` | `tc run download <id>` | ✅ Full parity |
| `gh run cancel <id>` | `tc run cancel <id>` | ✅ Full parity |
| `gh run rerun <id>` | `tc run restart <id>` | ✅ Full parity |
| `gh workflow disable` | `tc job pause` | ✅ Full parity |
| `gh workflow enable` | `tc job resume` | ✅ Full parity |
| `gh workflow list` | `tc job list` | ✅ Full parity |
| `gh workflow view` | `tc job view` | ✅ Full parity |
| `gh pr comment` | `tc run comment` | ✅ Full parity |
| `gh repo view` | `tc project view` | ✅ Similar concept |
| `gh api <endpoint>` | `tc api <endpoint>` | ✅ Full parity with raw API access |

---

## 🟡 Partial Parity - Could Be Enhanced

| GitHub CLI | TeamCity CLI | Gap Description | Priority |
|-----------|--------------|----------------|----------|
| `gh run list --json` | `tc run list --json` | ✅ Have JSON, but could add more filter options | 🟡 Medium |
| `gh pr checks` | `tc run view` | ✅ Shows status, but not as detailed for test failures | 🟡 Medium |
| `gh release view` | `tc run download` | ✅ Can download, but limited artifact browsing | 🟢 Low |
| `gh pr list` | `tc run list` | ✅ Lists runs, but no interactive picker | 🔴 High |
| `gh repo list` | `tc project list` | ✅ Lists projects, but limited search | 🟡 Medium |

---

## ❌ Missing Features - Gaps to Fill

### 🔴 High Priority Gaps

| GitHub CLI Concept | TeamCity Equivalent | Current Status | Proposed Command |
|-------------------|---------------------|----------------|------------------|
| `gh issue assign` | Build Investigation | ❌ Not implemented | `tc run investigate <id> --user <name>` |
| Issue assignment workflow | Assign failures to team members | ❌ Missing | `tc investigation list --active` |
| `gh issue close` | Resolve investigation | ❌ Missing | `tc run uninvestigate <id>` |
| Interactive pickers | Select from list | ❌ Missing | `tc run log --interactive` |
| Fuzzy search | Quick find | ❌ Missing | Built into interactive mode |

**Why these matter:**
- Investigation is critical for team collaboration
- Interactive pickers dramatically improve UX
- These are daily workflows for development teams

---

### 🟡 Medium Priority Gaps

| GitHub CLI Concept | TeamCity Equivalent | Current Status | Proposed Command |
|-------------------|---------------------|----------------|------------------|
| `gh pr diff` | Build comparison | ❌ Not implemented | `tc run diff <id1> <id2>` |
| Compare PR changes | Compare build changes | ❌ Missing | `tc run compare <id1> <id2>` |
| `gh search` | Advanced search | ❌ Limited | `tc search builds <query>` |
| `gh repo clone` | Clone VCS root | ❌ Missing (Git does this) | Not needed |
| Workflow dependencies | Build dependencies | ❌ Missing | `tc run deps <id>` |
| VCS status | VCS root status | ❌ Missing | `tc vcs check <vcs-root-id>` |
| Branch operations | Branch management | ❌ Missing | `tc job branches <id>` |

**Why these matter:**
- Common debugging workflows
- Better discovery in large TeamCity instances
- Understanding complex build pipelines

---

### 🟢 Low Priority Gaps

| GitHub CLI Concept | TeamCity Equivalent | Current Status | Proposed Command |
|-------------------|---------------------|----------------|------------------|
| `gh repo view --web` | ✅ `tc project view --web` | Already have `--web` flag | - |
| `gh release list` | Build history | ✅ Similar to `tc run list` | Could enhance filtering |
| `gh pr ready` | Mark build for promotion | ❌ Missing | `tc run promote <id>` |
| User management | User management | ❌ Missing | `tc user list` |
| Organization commands | User groups | ❌ Missing | `tc group list` |
| Workflow enable/disable | ✅ `tc job pause/resume` | Already implemented | - |
| `gh api --method POST` | ✅ `tc api -X POST` | Already implemented | - |

**Why these matter:**
- Nice to have for completeness
- Admin/management tasks
- Less frequent workflows

---

## 🆕 TeamCity-Specific Features (No GitHub Equivalent)

Features unique to TeamCity that GitHub doesn't have:

| Feature | TeamCity CLI | Status | Notes |
|---------|-------------|--------|-------|
| Build pinning | `tc run pin/unpin` | ✅ Implemented | Prevent cleanup |
| Build tagging | `tc run tag/untag` | ✅ Implemented | Categorize builds |
| Build queue management | `tc queue top/approve` | ✅ Implemented | Queue priority |
| Secure tokens | `tc project token` | ✅ Implemented | Credentials management |
| Build parameters | `tc job param` | ✅ Implemented | Runtime configuration |
| Build investigation | **Missing** | ❌ Not implemented | 🔴 High priority |
| Problem/test muting | **Missing** | ❌ Not implemented | 🔴 High priority |
| VCS roots | **Missing** | ❌ Not implemented | 🟡 Medium priority |
| Build templates | **Missing** | ❌ Not implemented | 🟢 Low priority |
| Cleanup rules | **Missing** | ❌ Not implemented | 🟢 Low priority |

---

## 📊 Command Structure Comparison

### GitHub CLI Structure
```
gh <noun> <verb> [arguments]

Examples:
gh pr create
gh issue list
gh run view
gh repo clone
```

### TeamCity CLI Structure  
```
tc <noun> <verb> [arguments]

Examples:
tc run start
tc job list
tc project view
tc queue approve
```

**Assessment:** ✅ Both use same noun-verb pattern, consistent UX

---

## 🎨 UX Pattern Comparison

| Pattern | GitHub CLI | TeamCity CLI | Notes |
|---------|-----------|--------------|-------|
| **Interactive prompts** | ✅ Yes | 🟡 Limited | gh prompts for missing args |
| **Web browser fallback** | ✅ `--web` flag | ✅ `--web` flag | Both support |
| **JSON output** | ✅ `--json` | ✅ `--json` | Full parity |
| **Color output** | ✅ Yes | ✅ Yes | Both support |
| **Quiet mode** | ✅ `--silent` | ✅ `--quiet` | Different flag names |
| **Verbose mode** | ✅ `--verbose` | ✅ `--verbose` | Full parity |
| **No-input mode** | ✅ Auto-detected | ✅ `--no-input` | tc is explicit |
| **Table formatting** | ✅ Pretty tables | 🟡 Basic tables | gh has better formatting |
| **Fuzzy search** | ✅ Built-in | ❌ No | gh has interactive pickers |
| **Progress indicators** | ✅ Spinners/bars | 🟡 Limited | gh shows more feedback |
| **Help system** | ✅ Excellent | ✅ Good | Both use Cobra |
| **Shell completion** | ✅ Yes | ✅ Yes | Full parity |

---

## 🔄 Workflow Comparison

### Starting a Build/Run

**GitHub:**
```bash
# Manual workflow trigger
gh workflow run build.yml --ref main -f version=1.0

# From a PR
gh pr checks
```

**TeamCity:**
```bash
# Start a build
tc run start MyProject_Build --branch main -P version=1.0

# Watch it run
tc run watch <build-id>
```

**Assessment:** ✅ Similar capabilities, tc has more build options

---

### Viewing Results

**GitHub:**
```bash
# List recent runs
gh run list --workflow build.yml --limit 10

# View specific run
gh run view 12345

# Watch in real-time
gh run watch 12345
```

**TeamCity:**
```bash
# List recent runs
tc run list --job MyProject_Build --limit 10

# View specific run
tc run view 12345

# Watch in real-time
tc run watch 12345
```

**Assessment:** ✅ Full parity

---

### Viewing Logs

**GitHub:**
```bash
# Download logs
gh run download 12345

# View specific job logs
gh run view 12345 --log
```

**TeamCity:**
```bash
# Download artifacts
tc run download 12345

# View logs interactively
tc run log 12345

# Just failed steps
tc run log 12345 --failed
```

**Assessment:** ✅ tc has better log viewing (interactive viewer)

---

### Debugging Failures

**GitHub:**
```bash
# View checks
gh pr checks

# View run with logs
gh run view 12345 --log-failed

# Re-run failed jobs
gh run rerun 12345 --failed
```

**TeamCity:**
```bash
# View build
tc run view 12345

# View logs
tc run log 12345 --failed

# Restart build
tc run restart 12345

# ❌ MISSING: View structured problems
# ❌ MISSING: Assign investigation
```

**Assessment:** 🔴 TeamCity missing investigation features

---

### Managing Configuration

**GitHub:**
```bash
# List workflows
gh workflow list

# Enable/disable
gh workflow enable build.yml
gh workflow disable build.yml

# View workflow file
gh workflow view build.yml
```

**TeamCity:**
```bash
# List jobs
tc job list

# Pause/resume
tc job pause MyProject_Build
tc job resume MyProject_Build

# View job details
tc job view MyProject_Build
```

**Assessment:** ✅ Full parity

---

## 🎯 Key Takeaways

### What TeamCity CLI Does Better
1. ✅ **Interactive log viewer** - Better than downloading logs
2. ✅ **Build pinning** - Unique TeamCity feature
3. ✅ **Queue management** - More granular control
4. ✅ **Secure token management** - Built-in secrets handling
5. ✅ **More build trigger options** - Personal builds, clean sources, etc.

### What GitHub CLI Does Better
1. ❌ **Interactive pickers** - Fuzzy search and selection
2. ❌ **Better table formatting** - More polished output
3. ❌ **Progress indicators** - Better user feedback
4. ❌ **Issue assignment** - Investigation equivalent missing
5. ❌ **Search functionality** - More powerful filtering

### Must-Have Additions
1. 🔴 **Build investigation commands** - Assign failures to users
2. 🔴 **Problem/test muting** - Reduce noise from known issues
3. 🔴 **Interactive selection** - Match gh CLI UX
4. 🟡 **Build comparison** - Debug regressions
5. 🟡 **Better search** - Find builds across large instances

---

## 📈 Recommended Improvements

### Quick Wins (Easy to Implement)
1. Add interactive prompts for missing arguments
2. Improve table formatting with borders/colors
3. Add progress spinners for long operations
4. Better error messages with suggestions
5. Smart defaults (latest build, current project)

### Medium Effort
1. Interactive pickers for all list commands
2. Build problem details view
3. VCS root management commands
4. Enhanced search with saved queries

### Larger Projects
1. Full investigation workflow
2. Problem and test muting
3. Build comparison and diff
4. Dependency visualization

---

## Conclusion

**Overall Assessment:** teamcity-cli has **strong feature parity** with GitHub CLI for core workflows, but **missing critical team collaboration features** (investigation, muting) and **interactive UX improvements** that would significantly improve daily use.

**Priority Order:**
1. 🔴 Add investigation & muting (unique TeamCity value)
2. 🔴 Add interactive pickers (UX improvement)
3. 🟡 Enhance search & filtering (discoverability)
4. 🟡 Add build comparison (debugging)
5. 🟢 Polish and quality-of-life improvements

The gap analysis shows that while the fundamentals are solid, adding these missing features would make teamcity-cli a best-in-class CI/CD CLI tool.
