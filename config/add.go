package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"jpop-live-calendar/internal/domain"
)

// AddSource appends a source to the configuration file at path, keeping
// the rest of the file, its comments included, as it was, and returns the
// file it wrote. A configuration read from the built-in copy is written,
// with the source, to UserPath, which Load then reads.
func AddSource(path string, src domain.SourceConfig) (string, error) {
	var data []byte
	target := path
	switch {
	case path == Embedded:
		data = embedded
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
		data = b
	}
	out, err := addSource(data, src)
	if err != nil {
		return "", err
	}
	if _, err := Parse(out); err != nil {
		return "", fmt.Errorf("添加后配置无效：%w", err)
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

// addedSource is what AddSource writes: the fields an in-app source form
// sets, in the file's order. Pointers and omitempty keep the rest out.
type addedSource struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"`
	Layer    string `yaml:"layer,omitempty"`
	Artist   string `yaml:"artist,omitempty"`
	URL      string `yaml:"url,omitempty"`
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval,omitempty"`
	Note     string `yaml:"note,omitempty"`
}

// addSource appends one item to the sources list of a sources.yaml.
func addSource(b []byte, src domain.SourceConfig) ([]byte, error) {
	if src.ID == "" {
		return nil, errors.New("数据源缺少 id")
	}
	cur, err := Parse(b)
	if err != nil {
		return nil, err
	}
	for _, s := range cur.Sources {
		if s.ID == src.ID {
			return nil, fmt.Errorf("数据源 id %q 已存在", src.ID)
		}
	}
	f := addedSource{
		ID: src.ID, Name: src.Name, Kind: string(src.Kind), Layer: src.Layer, Artist: src.Artist,
		URL: src.URL, Enabled: src.Enabled, Interval: intervalText(src.Interval), Note: src.Note,
	}
	block, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("配置文件不是映射")
	}
	root := doc.Content[0]
	at := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "sources" {
			at = i
		}
	}
	if at < 0 {
		return nil, errors.New("配置文件没有 sources 列表")
	}
	key, seq := root.Content[at], root.Content[at+1]
	if seq.Kind == yaml.SequenceNode && seq.Style&yaml.FlowStyle != 0 && len(seq.Content) > 0 {
		return nil, errors.New("配置文件的 sources 是行内格式，请在 sources.yaml 中手动添加")
	}
	lines := strings.SplitAfter(string(b), "\n")
	indent := "  "
	if seq.Kind == yaml.SequenceNode && len(seq.Content) > 0 {
		if line := nodeLine(lines, seq.Content[0].Line); line != "" {
			indent = line[:len(line)-len(strings.TrimLeft(line, " "))]
		}
	}
	item := indentBlock(block, indent)
	if seq.Kind != yaml.SequenceNode || len(seq.Content) == 0 {
		// sources: [] or a list with no items yet: write it below the key.
		line := key.Line - 1
		if line < 0 || line >= len(lines) {
			return nil, errors.New("无法定位 sources 列表")
		}
		head := strings.TrimRight(lines[line], "\r\n")
		// sources: [] carries no item; keep only the key.
		if i := strings.Index(head, "[]"); i >= 0 {
			head = strings.TrimRight(head[:i], " ")
		}
		lines[line] = head + "\n"
		lines = insertLines(lines, line+1, item)
		return joinLines(lines), nil
	}

	// The new item goes after the last one, before the blank lines and
	// comments above the next top-level key (or at the end of the file).
	end := len(lines)
	last := seq.Content[len(seq.Content)-1].Line
	for i := at + 2; i+1 < len(root.Content); i += 2 {
		end = root.Content[i].Line - 1
		break
	}
	if end > last {
		for end > last && isBlankOrComment(lines[end-1]) {
			end--
		}
	}
	if end == len(lines) && len(lines) > 0 && lines[len(lines)-1] == "" {
		end--
	}
	item = prependBlankIfNeeded(lines, end, item)
	return joinLines(insertLines(lines, end, item)), nil
}

// nodeLine returns the text of a 1-based YAML line, or "".
func nodeLine(lines []string, n int) string {
	if n < 1 || n > len(lines) {
		return ""
	}
	return strings.TrimSuffix(lines[n-1], "\n")
}

// indentBlock turns a marshaled mapping into a "- " item at indent.
func indentBlock(b []byte, indent string) []string {
	rows := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	out := make([]string, 0, len(rows))
	for i, row := range rows {
		if i == 0 {
			out = append(out, indent+"- "+row+"\n")
			continue
		}
		out = append(out, indent+"  "+row+"\n")
	}
	return out
}

// prependBlankIfNeeded keeps one blank line between the list and an item
// appended at the end of the file.
func prependBlankIfNeeded(lines []string, at int, item []string) []string {
	if at > 0 && strings.TrimSpace(lines[at-1]) == "" {
		return item
	}
	return append([]string{"\n"}, item...)
}

func insertLines(lines []string, at int, add []string) []string {
	out := make([]string, 0, len(lines)+len(add))
	out = append(out, lines[:at]...)
	out = append(out, add...)
	return append(out, lines[at:]...)
}

func joinLines(lines []string) []byte {
	var out bytes.Buffer
	for _, l := range lines {
		out.WriteString(l)
	}
	return out.Bytes()
}

// intervalText writes a duration as the configuration writes it: 6h, 30m,
// or 0 for manual only.
func intervalText(d time.Duration) string {
	switch {
	case d <= 0:
		return "0"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return d.String()
}
