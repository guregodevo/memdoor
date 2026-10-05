package ui

import (
	"fmt"
	"reflect"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Pages: one per agent (docs/roadmap/MUST.md, "TUI table stakes", 2026-09-26:
// "let's keep a page for each agent"). Each page gets its own conversation,
// its own channel and its own screen; a page strip at the top says which page
// is showing and what the others are doing, and ctrl+t switches. Today there
// is one page, the coder's; the machinery stays for an agent a plugin adds.
//
// A page is the whole existing Model, untouched: App is a thin outer model
// that owns the pages, shows the current one, and routes messages. Every
// message that belongs to a page carries its index (pageMsg): the page's
// websocket sends through a pageSender, and the commands a page returns are
// wrapped so what they produce comes back to the same page (tagCmd). A page
// in the background does not print to terminal scrollback (Model.background)
// and marks itself unseen when its turn ends.

// Agents a page can be opened for, in strip order.
func pageAgents() []string {
	return []string{"coder"}
}

// THE SKILL LIST IS A MENU, and the same "less is more" that took the
// non-coding commands out of the top-level listing (root.go:
// notCodingCommands) applies to it: a solo developer opening the dropdown
// should read coding skills, not this repo's own
// fixtures. Nothing is deleted — every skill below still runs when typed.
var (
	notCodingSkills = []string{
		// The clipper's, moved to github.com/guregodevo/clipper (2026-10-03);
		// a machine seeded before then still has them in ~/.memdoor/skills.
		"clipping", "documentary", "scraping",
		// Fixtures this repo tests the agent with.
		"hello-world", "greet-check",
		// Research on prompts, not work on a codebase.
		"semantic-compression", "system-prompts", "tool-prompt-optimization",
	}
)

// ListedSkills is the skills a menu offers: every one resolvable here, less
// the ones this build does not show.
func ListedSkills(names []string) []string {
	hide := make(map[string]bool, len(notCodingSkills))
	for _, n := range notCodingSkills {
		hide[n] = true
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !hide[n] {
			out = append(out, n)
		}
	}
	return out
}

// pageMsg is a message for one page.
type pageMsg struct {
	page int
	msg  tea.Msg
}

// msgSender is what the adapters send Bubble Tea messages through: the
// program itself, or a pageSender that tags them with the page.
type msgSender interface {
	Send(tea.Msg)
}

type pageSender struct {
	program *tea.Program
	page    int
}

func (s pageSender) Send(msg tea.Msg) {
	if s.program != nil {
		s.program.Send(pageMsg{page: s.page, msg: msg})
	}
}

var ourPkg = reflect.TypeOf(pageMsg{}).PkgPath()

// tagCmd wraps a page's command so its result comes back to that page.
// Bubble Tea's own messages (quit, println, exec, window title) must reach
// the runtime untagged; its batch and sequence messages are lists of
// commands and are rebuilt with each command wrapped.
func tagCmd(page int, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg { return tagMsg(page, cmd()) }
}

func tagMsg(page int, msg tea.Msg) tea.Msg {
	if msg == nil {
		return nil
	}
	if _, already := msg.(pageMsg); already {
		return msg
	}
	if b, ok := msg.(tea.BatchMsg); ok {
		out := make(tea.BatchMsg, 0, len(b))
		for _, c := range b {
			out = append(out, tagCmd(page, c))
		}
		return out
	}
	rv := reflect.ValueOf(msg)
	if rv.Kind() == reflect.Slice && rv.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		// tea.Sequence's message: an unexported slice of commands, rebuilt
		// through tea.Sequence so the runtime still runs them in order.
		cmds := make([]tea.Cmd, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			c, _ := rv.Index(i).Interface().(tea.Cmd)
			cmds = append(cmds, tagCmd(page, c))
		}
		return tea.Sequence(cmds...)()
	}
	if reflect.TypeOf(msg).PkgPath() != ourPkg {
		return msg
	}
	return pageMsg{page: page, msg: msg}
}

// App holds the pages and shows one.
type App struct {
	pages   []Model
	agents  []string // the agent of each page
	unseen  []bool   // a background page whose turn ended
	cur     int
	program *tea.Program
	width   int
	height  int
	// newPage opens the page at index page for an agent: a fresh channel,
	// a poster bound to it, the model with its ops (cmd/cli builds it).
	newPage func(agent string, page int) (Model, error)
}

// NewApp starts with one page open. The factory is required: an App that
// cannot open its other pages is not an App.
func NewApp(first Model, newPage func(agent string, page int) (Model, error)) App {
	if newPage == nil {
		panic("NewApp: a page factory is required")
	}
	if !first.paged || first.page != 0 {
		panic("NewApp: the first page must be built with PageConfig{Paged: true, Page: 0}")
	}
	first.background = false
	return App{pages: []Model{first}, agents: []string{first.codeAgent}, unseen: []bool{false}, newPage: newPage}
}

// Current is the page showing (tests).
func (a App) Current() Model { return a.pages[a.cur] }

func (a App) Init() tea.Cmd { return tagCmd(0, a.pages[0].Init()) }

// pageSize is the size a page gets: the terminal less the strip's line.
func (a App) pageSize() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: a.width, Height: a.height - 1}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch t := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = t.Width, t.Height
		var cmds []tea.Cmd
		for i := range a.pages {
			cmds = append(cmds, a.updatePage(i, a.pageSize()))
		}
		return a, tea.Batch(cmds...)
	case SetProgramMsg:
		a.program = t.Program
		var cmds []tea.Cmd
		for i := range a.pages {
			cmds = append(cmds, a.updatePage(i, msg)) // each page tags its websocket with its index
		}
		return a, tea.Batch(cmds...)
	case pageMsg:
		if t.page < 0 || t.page >= len(a.pages) {
			return a, nil
		}
		if t.page != a.cur {
			switch t.msg.(type) {
			case runCompleteMsg, executionFailedMsg, questionMsg:
				a.unseen[t.page] = true
			}
		}
		return a, a.updatePage(t.page, t.msg)
	case tea.KeyMsg:
		if t.Type == tea.KeyCtrlT && a.pages[a.cur].pendingQuestion == nil {
			return a.switchPage((a.cur + 1) % len(pageAgents()))
		}
	}
	return a, a.updatePage(a.cur, msg)
}

// updatePage runs one page's Update and tags what it returns.
func (a *App) updatePage(i int, msg tea.Msg) tea.Cmd {
	next, cmd := a.pages[i].Update(msg)
	if nm, ok := next.(Model); ok {
		a.pages[i] = nm
	}
	return tagCmd(i, cmd)
}

// switchPage shows the page for pageAgents()[n], opening it the first time.
func (a App) switchPage(n int) (tea.Model, tea.Cmd) {
	agents := pageAgents()
	if n < 0 || n >= len(agents) {
		return a, nil
	}
	agent := agents[n]
	idx := -1
	for i, ag := range a.agents {
		if ag == agent {
			idx = i
			break
		}
	}
	var cmds []tea.Cmd
	if idx < 0 {
		m, err := a.newPage(agent, len(a.pages))
		if err != nil {
			a.pages[a.cur].note("Could not open the " + agent + " page: " + err.Error())
			return a, nil
		}
		if !m.paged || m.page != len(a.pages) {
			panic("App: the page factory must build with PageConfig{Paged: true, Page: <index given>}")
		}
		m.background = true
		a.pages = append(a.pages, m)
		a.agents = append(a.agents, agent)
		a.unseen = append(a.unseen, false)
		idx = len(a.pages) - 1
		cmds = append(cmds, tagCmd(idx, a.pages[idx].Init()))
		if a.width > 0 {
			cmds = append(cmds, a.updatePage(idx, a.pageSize()))
		}
		if a.program != nil {
			cmds = append(cmds, a.updatePage(idx, SetProgramMsg{Program: a.program}))
		}
	}
	if idx == a.cur {
		return a, tea.Batch(cmds...)
	}
	a.pages[a.cur].background = true
	a.cur = idx
	a.unseen[idx] = false
	a.pages[idx].background = false
	// The terminal's scrollback is one stream: a rule says whose lines
	// follow.
	cmds = append(cmds, tea.Println(pageRule(agent, a.width)))
	// The page redraws for the size it may have missed while away.
	if a.width > 0 {
		cmds = append(cmds, a.updatePage(idx, a.pageSize()))
	}
	return a, tea.Batch(cmds...)
}

func pageRule(agent string, width int) string {
	label := " " + agent + " "
	n := width - lipgloss.Width(label) - 2
	if n < 4 {
		n = 4
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Render("──" + label + strings.Repeat("─", n))
}

var (
	stripCurStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colSelFG)).Background(lipgloss.Color(colSelBG)).Padding(0, 1)
	stripStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Padding(0, 1)
	stripHint     = lipgloss.NewStyle().Foreground(lipgloss.Color(colDim))
)

// renderStrip is the page strip: every agent, the current one lit, a dot
// for a page whose turn is running, a tick for one that finished while
// you were away.
func (a App) renderStrip() string {
	var parts []string
	for _, agent := range pageAgents() {
		label := agent
		for i, ag := range a.agents {
			if ag != agent {
				continue
			}
			switch {
			case a.pages[i].busy():
				label += " ●"
			case a.unseen[i]:
				label += " ✓"
			}
		}
		if a.agents[a.cur] == agent {
			parts = append(parts, stripCurStyle.Render(label))
		} else {
			parts = append(parts, stripStyle.Render(label))
		}
	}
	if len(pageAgents()) < 2 {
		// One agent, nothing to switch to: the hint would be a key that does
		// nothing.
		return strings.Join(parts, "")
	}
	return strings.Join(parts, "") + stripHint.Render("  ctrl+t switches")
}

func (a App) View() string {
	return fmt.Sprintf("%s\n%s", a.renderStrip(), a.pages[a.cur].View())
}
