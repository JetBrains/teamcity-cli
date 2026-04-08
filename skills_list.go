package teamcitycli

import (
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

const DefaultSkill = "teamcity-cli"

type SkillInfo struct {
	Name        string
	Version     string
	Description string
}

func ListSkills() []SkillInfo {
	var skills []SkillInfo
	_ = fs.WalkDir(SkillsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		data, err := fs.ReadFile(SkillsFS, p)
		if err != nil {
			return err
		}
		info := parseSkillInfo(data)
		if info.Name != "" {
			skills = append(skills, info)
		}
		return fs.SkipDir
	})
	return skills
}

func SkillSubFS(name string) (fs.FS, bool) {
	var skillPath string
	_ = fs.WalkDir(SkillsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		data, err := fs.ReadFile(SkillsFS, p)
		if err != nil {
			return err
		}
		info := parseSkillInfo(data)
		if info.Name == name {
			skillPath = path.Dir(p)
			return fs.SkipAll
		}
		return fs.SkipDir
	})
	if skillPath == "" {
		return nil, false
	}
	// instill.findSkills expects <dirname>/SKILL.md structure.
	// Use fs.Sub to get the skill contents, then wrap in a prefixFS
	// so the result contains only <basename>/SKILL.md (not sibling skills).
	inner, err := fs.Sub(SkillsFS, skillPath)
	if err != nil {
		return nil, false
	}
	return prefixFS{prefix: path.Base(skillPath), inner: inner}, true
}

type prefixFS struct {
	prefix string
	inner  fs.FS
}

func (f prefixFS) Open(name string) (fs.File, error) {
	if name == "." {
		return openPrefixDir(f.prefix), nil
	}
	after, ok := strings.CutPrefix(name, f.prefix+"/")
	if !ok {
		if name == f.prefix {
			return f.inner.Open(".")
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f.inner.Open(after)
}

type prefixDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func openPrefixDir(name string) *prefixDir {
	return &prefixDir{
		name:    name,
		entries: []fs.DirEntry{dirEntry{name}},
	}
}

func (d *prefixDir) Stat() (fs.FileInfo, error) { return dirInfo{d.name}, nil }
func (d *prefixDir) Read([]byte) (int, error)    { return 0, &fs.PathError{Op: "read", Path: ".", Err: fs.ErrInvalid} }
func (d *prefixDir) Close() error                { return nil }

func (d *prefixDir) ReadDir(n int) ([]fs.DirEntry, error) {
	remaining := d.entries[d.offset:]
	if n <= 0 {
		d.offset = len(d.entries)
		return remaining, nil
	}
	if len(remaining) == 0 {
		return nil, io.EOF
	}
	if n > len(remaining) {
		n = len(remaining)
	}
	d.offset += n
	return remaining[:n], nil
}

type dirEntry struct{ name string }

func (e dirEntry) Name() string               { return e.name }
func (e dirEntry) IsDir() bool                { return true }
func (e dirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e dirEntry) Info() (fs.FileInfo, error) { return dirInfo{e.name}, nil }

type dirInfo struct{ name string }

func (i dirInfo) Name() string      { return i.name }
func (i dirInfo) Size() int64       { return 0 }
func (i dirInfo) Mode() fs.FileMode { return fs.ModeDir | 0o755 }
func (i dirInfo) ModTime() time.Time { return time.Time{} }
func (i dirInfo) IsDir() bool       { return true }
func (i dirInfo) Sys() any          { return nil }

func parseSkillInfo(data []byte) SkillInfo {
	var info SkillInfo
	fm := parseFrontmatter(data)
	info.Name = fm["name"]
	info.Version = strings.Trim(fm["version"], `"'`)
	info.Description = fm["description"]
	return info
}

func parseFrontmatter(data []byte) map[string]string {
	s := string(data)
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "---") {
		return nil
	}
	parts := strings.SplitN(s, "---", 3)
	if len(parts) < 3 {
		return nil
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(parts[1], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return fields
}
