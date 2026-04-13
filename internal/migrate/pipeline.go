package migrate

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	Comment    string
	Jobs       []Job
	Parameters map[string]string
}

type Job struct {
	ID                    string
	Name                  string
	RunsOn                string
	Dependencies          []string
	EnableDependencyCache bool
	Parameters            map[string]string
	Steps                 []Step
	FilesPublication      []FilePublication
}

type Step struct {
	Name             string
	ScriptContent    string
	WorkingDirectory string
	Parameters       map[string]string
}

type FilePublication struct {
	Path            string
	ShareWithJobs   bool
	PublishArtifact bool
}

var jobIDReplacer = strings.NewReplacer("-", "_", ".", "_", "/", "_", "*", "", " ", "_")

func SanitizeJobID(id string) string {
	return jobIDReplacer.Replace(id)
}

type jobIDTracker struct {
	seen   map[string]bool
	mapped map[string]string
}

func newJobIDTracker() *jobIDTracker {
	return &jobIDTracker{seen: map[string]bool{}, mapped: map[string]string{}}
}

// register allocates a fresh output key for a job emission. Duplicate raw IDs
// get unique suffixes instead of overwriting each other in the YAML mapping.
// The first occurrence wins for dependency lookup via assign.
func (t *jobIDTracker) register(id string) string {
	base := SanitizeJobID(id)
	result := base
	for i := 2; t.seen[result]; i++ {
		result = fmt.Sprintf("%s_%d", base, i)
	}
	t.seen[result] = true
	if _, ok := t.mapped[id]; !ok {
		t.mapped[id] = result
	}
	return result
}

// assign resolves a raw id to the output key of its first-registered job,
// used for dependency references.
func (t *jobIDTracker) assign(id string) string {
	if tc, ok := t.mapped[id]; ok {
		return tc
	}
	return SanitizeJobID(id)
}

func (p *Pipeline) String() string {
	ids := newJobIDTracker()

	// Pre-register all jobs so each gets a unique output key and dependency
	// references can resolve to the first registration of a raw ID.
	jobKeys := make([]string, len(p.Jobs))
	for i, j := range p.Jobs {
		jobKeys[i] = ids.register(j.ID)
	}

	root := &yaml.Node{Kind: yaml.MappingNode}

	if len(p.Parameters) > 0 {
		addField(root, "parameters", envParamsNode(p.Parameters))
	}

	jobsMap := &yaml.Node{Kind: yaml.MappingNode}
	for i, j := range p.Jobs {
		addField(jobsMap, jobKeys[i], jobNode(&j, ids))
	}
	addField(root, "jobs", jobsMap)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}})
	_ = enc.Close()

	result := buf.String()
	if p.Comment != "" {
		result = p.Comment + result
	}
	return result
}

func jobNode(j *Job, ids *jobIDTracker) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}

	addField(m, "name", quoted(j.Name))

	if j.RunsOn != "" {
		addField(m, "runs-on", scalar(j.RunsOn))
	}

	if len(j.Dependencies) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, dep := range j.Dependencies {
			seq.Content = append(seq.Content, scalar(ids.assign(dep)))
		}
		addField(m, "dependencies", seq)
	}

	if j.EnableDependencyCache {
		addField(m, "enable-dependency-cache", boolNode(true))
	}

	if len(j.Parameters) > 0 {
		addField(m, "parameters", envParamsNode(j.Parameters))
	}

	if len(j.Steps) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for i := range j.Steps {
			seq.Content = append(seq.Content, stepNode(&j.Steps[i]))
		}
		addField(m, "steps", seq)
	}

	if len(j.FilesPublication) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, fp := range j.FilesPublication {
			entry := &yaml.Node{Kind: yaml.MappingNode}
			addField(entry, "path", quoted(fp.Path))
			if fp.ShareWithJobs {
				addField(entry, "share-with-jobs", boolNode(true))
			}
			if fp.PublishArtifact {
				addField(entry, "publish-artifact", boolNode(true))
			}
			seq.Content = append(seq.Content, entry)
		}
		addField(m, "files-publication", seq)
	}

	return m
}

func stepNode(s *Step) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}

	addField(m, "type", scalar("script"))

	if s.Name != "" {
		addField(m, "name", quoted(s.Name))
	}
	if s.WorkingDirectory != "" {
		addField(m, "working-directory", quoted(s.WorkingDirectory))
	}
	if len(s.Parameters) > 0 {
		addField(m, "parameters", envParamsNode(s.Parameters))
	}

	scriptVal := &yaml.Node{Kind: yaml.ScalarNode, Value: s.ScriptContent}
	if strings.Contains(s.ScriptContent, "\n") {
		scriptVal.Style = yaml.LiteralStyle
	}
	addField(m, "script-content", scriptVal)

	return m
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
}

func quoted(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: v, Style: yaml.DoubleQuotedStyle}
}

func boolNode(v bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(v), Tag: "!!bool"}
}

func addField(m *yaml.Node, key string, value *yaml.Node) {
	m.Content = append(m.Content, scalar(key), value)
}

func envParamsNode(params map[string]string) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range SortedKeys(params) {
		addField(m, "env."+k, quoted(params[k]))
	}
	return m
}
