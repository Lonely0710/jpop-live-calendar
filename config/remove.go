package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RemoveSource deletes the source id from the configuration file at
// path, keeping the rest of the file, its comments included, as it was,
// and returns the file it wrote. A configuration read from the built-in
// copy is written, without the source, to UserPath, which Load then reads.
func RemoveSource(path, id string) (string, error) {
	var src []byte
	target := path
	switch {
	case path == Embedded:
		src = embedded
		p, err := UserPath()
		if err != nil {
			return "", err
		}
		target = p
	case strings.HasPrefix(path, Embedded):
		// The user's file is broken and the app runs on the defaults:
		// writing would replace the file being fixed.
		return "", errors.New("配置文件有错误，未写入")
	default:
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		src = b
	}
	out, err := removeSource(src, id)
	if err != nil {
		return "", err
	}
	if _, err := Parse(out); err != nil {
		return "", fmt.Errorf("删除后配置无效：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return target, nil
}

// removeSource cuts the lines of one item of the sources list out of a
// sources.yaml: from the comments right above it to the comments right
// above what follows it, which belong to the next.
func removeSource(b []byte, id string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("配置文件不是映射")
	}
	root := doc.Content[0]
	var seq *yaml.Node
	next := 0 // the line of the key after sources, 0 at the end
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "sources" {
			seq = root.Content[i+1]
			if i+2 < len(root.Content) {
				next = root.Content[i+2].Line
			}
		}
	}
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, errors.New("配置文件没有 sources 列表")
	}
	at := -1
	for i, item := range seq.Content {
		for j := 0; j+1 < len(item.Content); j += 2 {
			if item.Content[j].Value == "id" && item.Content[j+1].Value == id {
				at = i
			}
		}
	}
	if at < 0 {
		return nil, fmt.Errorf("没有数据源 %q", id)
	}
	lines := strings.SplitAfter(string(b), "\n")
	end := len(lines) // exclusive, 0-based
	if at+1 < len(seq.Content) {
		end = seq.Content[at+1].Line - 1
	} else if next > 0 {
		end = next - 1
	}
	// Back over the comments and blank lines above what follows.
	for end > 0 && isBlankOrComment(lines[end-1]) {
		end--
	}
	start := seq.Content[at].Line - 1
	for start > 0 && isComment(lines[start-1]) {
		start--
	}
	// One blank line stays between the items left.
	for end < len(lines) && strings.TrimSpace(lines[end]) == "" && (start == 0 || strings.TrimSpace(lines[start-1]) == "") {
		end++
	}
	var out bytes.Buffer
	for _, l := range lines[:start] {
		out.WriteString(l)
	}
	for _, l := range lines[end:] {
		out.WriteString(l)
	}
	return out.Bytes(), nil
}

func isComment(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "#") }

func isBlankOrComment(l string) bool { return strings.TrimSpace(l) == "" || isComment(l) }
