package mcp

import (
	"io/fs"
	"strings"
)

// Skills serves the SKILL.md files under fsys (one directory per skill, as
// in a Claude plugin's skills/) as prompts, so an installed binary carries
// them without the repository.
//
//	//go:embed */SKILL.md
//	var FS embed.FS
//	server.Prompts = mcp.Skills(skills.FS)
func Skills(fsys fs.FS) Prompts { return skillFS{fsys} }

type skillFS struct{ fsys fs.FS }

func (s skillFS) Text(name string) string {
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return ""
	}
	b, _ := fs.ReadFile(s.fsys, name+"/SKILL.md")
	return string(b)
}

func (s skillFS) Names() []string {
	dirs, _ := fs.ReadDir(s.fsys, ".")
	var names []string
	for _, d := range dirs {
		if d.IsDir() && s.Text(d.Name()) != "" {
			names = append(names, d.Name())
		}
	}
	return names
}

// Description is the description line of the skill's frontmatter.
func (s skillFS) Description(name string) string {
	for line := range strings.SplitSeq(s.Text(name), "\n") {
		if rest, ok := strings.CutPrefix(line, "description: "); ok {
			return rest
		}
	}
	return ""
}
