package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// readYAMLMapping loads a YAML file whose top level is a mapping.
// It returns nil (no error) when the file is missing or empty.
func readYAMLMapping(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parsing %s: expected a mapping of keys", path)
	}
	return &doc, nil
}

// decodeYAMLFile decodes path into v; a missing file leaves v untouched.
// Unknown keys are ignored (forward compatibility).
func decodeYAMLFile(path string, v any) error {
	doc, err := readYAMLMapping(path)
	if err != nil || doc == nil {
		return err
	}
	if err := doc.Decode(v); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

// editYAMLFile applies fn to the top-level mapping of path and writes it back,
// keeping comments and unknown keys. A missing file is created with
// headComment as its first line.
func editYAMLFile(path, headComment string, fn func(m *yaml.Node)) error {
	doc, err := readYAMLMapping(path)
	if err != nil {
		return err
	}
	if doc == nil {
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{
			Kind:        yaml.MappingNode,
			HeadComment: headComment,
		}}}
	}
	m := doc.Content[0]
	fn(m)
	if len(m.Content) == 0 {
		// An empty mapping would be written as "{}": keep only the comments.
		return writeFileAtomic(path, []byte(commentLines(doc.HeadComment, m.HeadComment, m.FootComment, doc.FootComment)))
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return writeFileAtomic(path, data)
}

// commentLines renders YAML node comments as "# ..." lines.
func commentLines(comments ...string) string {
	var b strings.Builder
	for _, c := range comments {
		for _, line := range strings.Split(c, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "#") {
				line = "# " + line
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// setMappingValue sets key to a scalar value (tag e.g. "!!int", "!!str").
func setMappingValue(m *yaml.Node, key, value, tag string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			v.Kind, v.Tag, v.Value, v.Style, v.Content = yaml.ScalarNode, tag, value, 0, nil
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value},
	)
}

// deleteMappingKey removes key (and its value) from the mapping, if present.
// Comments above the removed key (e.g. a file header) are kept by moving them
// to the next key, or to the mapping itself when no key follows.
func deleteMappingKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != key {
			continue
		}
		head := m.Content[i].HeadComment
		m.Content = append(m.Content[:i], m.Content[i+2:]...)
		if head != "" {
			if i < len(m.Content) {
				m.Content[i].HeadComment = joinComments(head, m.Content[i].HeadComment)
			} else {
				m.HeadComment = joinComments(m.HeadComment, head)
			}
		}
		return
	}
}

func joinComments(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "\n" + b
	}
}
