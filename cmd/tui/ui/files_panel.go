package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// THE FILES PANEL (Greg, 2026-10-10: "TUI but better than emacs for files
// navigation", "less mental burning out feature is welcome"). What a person
// wants after a turn is not a file tree, it is "what did it touch, and where":
// ctrl+f (or /files) opens the project's files with the ones THIS
// CONVERSATION read or changed first, newest first, each with what happened
// to it (✎ 2 edits · 3 reads); the rest of the tree is behind the same
// filter. Typing filters, up/down move, enter previews the file beside the
// list, ctrl+d shows its diff since the turn started, ctrl+e opens it in
// $EDITOR, tab puts @path into the prompt, esc closes. No modes, no mouse,
// one key in and one key out.

type fileEntry struct {
	path     string
	reads    int
	edits    int
	hits     int // named by a search's result (grep, glob, locate)
	last     time.Time
	lastTool string
}

type filesPanelState struct {
	root          string
	all           []string              // the project's files, loadFileList's order
	touched       map[string]*fileEntry // what this conversation read or changed
	query         string
	matches       []fileEntry
	index         int
	preview       []string // the selected file's text, or its diff
	previewPath   string
	previewDiff   bool
	previewScroll int
	note          string
}

type filesEditorDoneMsg struct{ err error }

const (
	filesPanelRows    = 12
	filesPreviewLines = 200
)

var patchFilesRe = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+)$`)

// filesCommand is "/files [query]".
func (m *Model) filesCommand(args []string) tea.Cmd {
	m.openFilesPanel(strings.Join(args, " "))
	return nil
}

// openFilesPanel opens (or refreshes) the panel on the working directory.
func (m *Model) openFilesPanel(query string) {
	wd, err := os.Getwd()
	if err != nil {
		m.note("The working directory could not be read.")
		return
	}
	if m.fileList == nil || time.Since(m.fileListAt) > fileListTTL {
		m.fileList = loadFileList(wd)
		m.fileListAt = time.Now()
	}
	p := &filesPanelState{root: wd, all: m.fileList, touched: m.filesTouched(), query: query}
	m.filesPanel = p
	m.filesRefresh()
	m.filesLoadPreview()
}

// filesTouched reads the conversation's tool frames: a read tool's path
// counts as a read, an editing tool's as an edit, a patch's every file.
func (m *Model) filesTouched() map[string]*fileEntry {
	out := map[string]*fileEntry{}
	bump := func(path string, edit bool, tool string, at time.Time) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		path = filepath.ToSlash(path)
		e := out[path]
		if e == nil {
			e = &fileEntry{path: path}
			out[path] = e
		}
		if edit {
			e.edits++
		} else {
			e.reads++
		}
		if at.After(e.last) {
			e.last, e.lastTool = at, tool
		}
	}
	for _, msg := range m.messages {
		if msg.Role != "tool_call" || msg.ToolInput == "" {
			continue
		}
		var in map[string]any
		if json.Unmarshal([]byte(msg.ToolInput), &in) != nil {
			continue
		}
		kind := toolKindOf(msg.ToolName)
		// A search's hits are files the conversation found, newest first
		// like the rest: ctrl+f after a grep is the hit list (2026-10-11).
		switch msg.ToolName {
		case "grep", "search", "glob", "locate":
			files, counts := searchHitFiles(msg.ToolName, msg.ToolOutput)
			for _, f := range files {
				n := counts[f]
				if m.filesPanel != nil && filepath.IsAbs(f) {
					if rel, err := filepath.Rel(m.filesPanel.root, f); err == nil && !strings.HasPrefix(rel, "..") {
						f = rel
					}
				}
				f = filepath.ToSlash(strings.TrimSpace(f))
				if f == "" {
					continue
				}
				e := out[f]
				if e == nil {
					e = &fileEntry{path: f}
					out[f] = e
				}
				e.hits += n
				if msg.Timestamp.After(e.last) {
					e.last, e.lastTool = msg.Timestamp, msg.ToolName
				}
			}
		}
		switch kind {
		case fileToolRead, fileToolEdit:
			for _, k := range []string{"path", "file_path", "file"} {
				if v, ok := in[k].(string); ok {
					bump(v, kind == fileToolEdit, msg.ToolName, msg.Timestamp)
				}
			}
			for _, k := range []string{"patch", "input"} {
				if v, ok := in[k].(string); ok {
					for _, mm := range patchFilesRe.FindAllStringSubmatch(v, -1) {
						bump(mm[1], true, msg.ToolName, msg.Timestamp)
					}
				}
			}
		}
	}
	return out
}

type fileToolKind int

const (
	fileToolOther fileToolKind = iota
	fileToolRead
	fileToolEdit
)

func toolKindOf(name string) fileToolKind {
	switch name {
	case "read_file", "jread", "jgrep", "grep", "locate", "glob", "list_files":
		return fileToolRead
	case "write_file", "edit_file", "apply_patch", "search_replace":
		return fileToolEdit
	}
	return fileToolOther
}

// filesRefresh recomputes the list for the query: the touched files first,
// newest first, then the rest of the tree by matchFiles.
func (m *Model) filesRefresh() {
	p := m.filesPanel
	if p == nil {
		return
	}
	q := strings.ToLower(strings.TrimSpace(p.query))
	var touched []fileEntry
	for _, e := range p.touched {
		if q == "" || strings.Contains(strings.ToLower(e.path), q) {
			touched = append(touched, *e)
		}
	}
	sort.Slice(touched, func(i, j int) bool { return touched[i].last.After(touched[j].last) })
	seen := map[string]bool{}
	for _, e := range touched {
		seen[e.path] = true
	}
	out := touched
	for _, f := range matchFiles(p.all, q, 400) {
		if seen[f] {
			continue
		}
		out = append(out, fileEntry{path: f})
		if len(out) >= 400 {
			break
		}
	}
	p.matches = out
	if p.index >= len(out) {
		p.index = 0
	}
}

// filesLoadPreview reads the selected file (or its diff) for the right column.
func (m *Model) filesLoadPreview() {
	p := m.filesPanel
	if p == nil || len(p.matches) == 0 {
		return
	}
	sel := p.matches[p.index].path
	if sel == p.previewPath && p.preview != nil {
		return
	}
	p.previewPath, p.previewScroll, p.preview = sel, 0, nil
	full := filepath.Join(p.root, filepath.FromSlash(sel))
	if strings.HasSuffix(sel, "/") {
		entries, err := os.ReadDir(full)
		if err != nil {
			p.preview = []string{err.Error()}
			return
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			p.preview = append(p.preview, name)
		}
		return
	}
	if p.previewDiff {
		rel := filepath.FromSlash(sel)
		out, _ := exec.Command("git", "-C", p.root, "diff", "--", rel).CombinedOutput()
		if strings.TrimSpace(string(out)) == "" {
			out, _ = exec.Command("git", "-C", p.root, "diff", "--cached", "--", rel).CombinedOutput()
		}
		if strings.TrimSpace(string(out)) == "" {
			// A file the turn created is untracked: git diff says nothing;
			// against /dev/null it is the whole file, added.
			if exec.Command("git", "-C", p.root, "ls-files", "--error-unmatch", rel).Run() != nil {
				out, _ = exec.Command("git", "-C", p.root, "diff", "--no-index", "--", os.DevNull, rel).CombinedOutput()
			}
		}
		if strings.TrimSpace(string(out)) == "" {
			p.preview = []string{"(no uncommitted change in this file)"}
			return
		}
		p.preview = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		return
	}
	b, err := os.ReadFile(full)
	if err != nil {
		p.preview = []string{err.Error()}
		return
	}
	if len(b) > 512*1024 {
		b = b[:512*1024]
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > filesPreviewLines*4 {
		lines = append(lines[:filesPreviewLines*4], fmt.Sprintf("… %d more lines", len(lines)-filesPreviewLines*4))
	}
	for i, l := range lines {
		lines[i] = fmt.Sprintf("%4d  %s", i+1, strings.ReplaceAll(l, "\t", "    "))
	}
	p.preview = lines
}

// updateFilesPanel takes every key while the panel is open.
func (m *Model) updateFilesPanel(msg tea.Msg) (handled bool, cmd tea.Cmd) {
	if done, ok := msg.(filesEditorDoneMsg); ok {
		if done.err != nil {
			m.note("The editor returned: " + done.err.Error())
		}
		if m.filesPanel != nil {
			m.filesPanel.preview = nil // re-read what the editor may have changed
			m.filesLoadPreview()
		}
		return true, nil
	}
	p := m.filesPanel
	if p == nil {
		return false, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return false, nil
	}
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlF:
		m.filesPanel = nil
		return true, nil
	case tea.KeyUp:
		if p.index > 0 {
			p.index--
		}
		m.filesLoadPreview()
		return true, nil
	case tea.KeyDown:
		if p.index < len(p.matches)-1 {
			p.index++
		}
		m.filesLoadPreview()
		return true, nil
	case tea.KeyPgUp:
		p.previewScroll -= filesPanelRows
		if p.previewScroll < 0 {
			p.previewScroll = 0
		}
		return true, nil
	case tea.KeyPgDown:
		if p.previewScroll+filesPanelRows < len(p.preview) {
			p.previewScroll += filesPanelRows
		}
		return true, nil
	case tea.KeyEnter:
		if len(p.matches) == 0 {
			return true, nil
		}
		sel := p.matches[p.index].path
		if strings.HasSuffix(sel, "/") {
			// A folder: narrow to it.
			p.query, p.index = sel, 0
			m.filesRefresh()
			m.filesLoadPreview()
			return true, nil
		}
		p.previewDiff = false
		p.preview = nil
		m.filesLoadPreview()
		return true, nil
	case tea.KeyCtrlD:
		p.previewDiff = !p.previewDiff
		p.preview = nil
		m.filesLoadPreview()
		return true, nil
	case tea.KeyTab:
		if len(p.matches) == 0 {
			return true, nil
		}
		v := m.input.Value()
		if v != "" && !strings.HasSuffix(v, " ") {
			v += " "
		}
		m.input.SetValue(v + "@" + p.matches[p.index].path + " ")
		m.input.CursorEnd()
		m.filesPanel = nil
		return true, nil
	case tea.KeyCtrlE:
		if len(p.matches) == 0 || strings.HasSuffix(p.matches[p.index].path, "/") {
			return true, nil
		}
		editor := strings.TrimSpace(os.Getenv("VISUAL"))
		if editor == "" {
			editor = strings.TrimSpace(os.Getenv("EDITOR"))
		}
		if editor == "" {
			editor = "vi"
		}
		parts := strings.Fields(editor)
		full := filepath.Join(p.root, filepath.FromSlash(p.matches[p.index].path))
		c := exec.Command(parts[0], append(parts[1:], full)...)
		return true, tea.ExecProcess(c, func(err error) tea.Msg { return filesEditorDoneMsg{err: err} })
	case tea.KeyBackspace:
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
			p.index = 0
			m.filesRefresh()
			m.filesLoadPreview()
		}
		return true, nil
	case tea.KeyRunes, tea.KeySpace:
		s := string(k.Runes)
		if k.Type == tea.KeySpace {
			s = " "
		}
		p.query += s
		p.index = 0
		m.filesRefresh()
		m.filesLoadPreview()
		return true, nil
	}
	return true, nil
}

// renderFilesPanel draws the two columns in the panel slot below the
// conversation: the list, then the preview of the selected entry.
func (m Model) renderFilesPanel() string {
	p := m.filesPanel
	if p == nil {
		return ""
	}
	width := boxWidth(m.width) - 2
	if width < 40 {
		width = 40
	}
	listW := width * 2 / 5
	if listW < 28 {
		listW = 28
	}
	if listW > 56 {
		listW = 56
	}
	prevW := width - listW - 3
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colText))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
	faint := lipgloss.NewStyle().Foreground(lipgloss.Color(colFaint))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG)).Bold(true)
	edited := lipgloss.NewStyle().Foreground(lipgloss.Color(colWrite))
	read := lipgloss.NewStyle().Foreground(lipgloss.Color(colRead))
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(colRule))

	// The list: a window of filesPanelRows around the cursor.
	start := 0
	if p.index >= filesPanelRows {
		start = p.index - filesPanelRows + 1
	}
	var left []string
	head := "files"
	if p.query != "" {
		head += "  " + p.query + "▏"
	} else {
		head += "  " + dim.Render("type to filter") + "▏"
	}
	left = append(left, title.Render(truncateTo(head, listW)))
	for i := start; i < len(p.matches) && len(left) < filesPanelRows+1; i++ {
		e := p.matches[i]
		mark, style := "  ", dim
		if e.edits > 0 {
			mark, style = "✎ ", edited
		} else if e.reads > 0 {
			mark, style = "◦ ", read
		} else if e.hits > 0 {
			mark, style = "⌕ ", read
		}
		label := e.path
		if e.edits > 0 || e.reads > 0 || e.hits > 0 {
			var parts []string
			if e.edits > 0 {
				parts = append(parts, fmt.Sprintf("%d edit%s", e.edits, plural(e.edits)))
			}
			if e.reads > 0 {
				parts = append(parts, fmt.Sprintf("%d read%s", e.reads, plural(e.reads)))
			}
			if e.hits > 0 {
				parts = append(parts, fmt.Sprintf("%d hit%s", e.hits, plural(e.hits)))
			}
			label += "  " + strings.Join(parts, " · ")
		}
		line := truncateTo(mark+label, listW)
		if i == p.index {
			left = append(left, sel.Render(padTo(line, listW)))
		} else {
			left = append(left, style.Render(line))
		}
	}
	if len(p.matches) == 0 {
		left = append(left, faint.Render("nothing matches"))
	}
	for len(left) < filesPanelRows+1 {
		left = append(left, "")
	}

	// The preview.
	var right []string
	ph := p.previewPath
	if p.previewDiff {
		ph += "  (diff)"
	}
	if ph == "" {
		ph = "preview"
	}
	right = append(right, title.Render(truncateTo(ph, prevW)))
	end := p.previewScroll + filesPanelRows
	if end > len(p.preview) {
		end = len(p.preview)
	}
	for _, l := range p.preview[min(p.previewScroll, len(p.preview)):end] {
		style := dim
		switch {
		case p.previewDiff && strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(colOK))
		case p.previewDiff && strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(colErr))
		case p.previewDiff && strings.HasPrefix(l, "@@"):
			style = read
		}
		right = append(right, style.Render(truncateTo(l, prevW)))
	}
	for len(right) < filesPanelRows+1 {
		right = append(right, "")
	}

	var b strings.Builder
	for i := 0; i < filesPanelRows+1; i++ {
		b.WriteString(padTo(left[i], listW))
		b.WriteString(rule.Render(" │ "))
		b.WriteString(right[i])
		b.WriteString("\n")
	}
	b.WriteString(faint.Render("↑↓ move · enter preview · ctrl+d diff · ctrl+e open in $EDITOR · tab @mention · pgup/pgdn scroll · esc close"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colRule)).Padding(0, 1).Width(boxWidth(m.width)).Render(b.String())
}

func truncateTo(s string, w int) string {
	if w <= 1 {
		return ""
	}
	if r := []rune(s); len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s
}

func padTo(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
