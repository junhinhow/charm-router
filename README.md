# charm-router

A screen stack for Bubble Tea v2: push/pop navigation, exit guards, typed params and a resize-safe breadcrumb.

## Install

```bash
go get github.com/junhinhow/charm-router
```

Requires Go 1.25 or newer and Bubble Tea v2.

## Usage

The router is the root model of the program. Screens that need to navigate keep a
reference to the router and call `Push`, `Pop`, `Replace` or `Reset` from inside
their own `Update`.

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/junhinhow/charm-router"
)

// listScreen is the base screen: it shows the list and opens the detail.
type listScreen struct {
	router *router.Router
	width  int
	height int
}

func (s *listScreen) Init() tea.Cmd { return nil }

func (s *listScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	switch key.String() {
	case "enter":
		return s, s.router.Push(&detailScreen{router: s.router}, "item-42")
	case "q", "ctrl+c":
		return s, tea.Quit
	}
	return s, nil
}

func (s *listScreen) View() tea.View {
	lines := []string{
		fmt.Sprintf("List (%dx%d)", s.width, s.height),
		"",
		"enter  open detail",
		"q      quit",
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (s *listScreen) Title() string { return "List" }

func (s *listScreen) SetSize(width, height int) {
	s.width, s.height = width, height
}

// detailScreen receives a param from Push and protects the exit with a guard.
type detailScreen struct {
	router *router.Router
	item   string
	dirty  bool
	notice string
	width  int
	height int
}

func (s *detailScreen) Init() tea.Cmd {
	// Push already stacked the screen, so params are available here.
	if item, ok := router.ParamsAs[string](s.router); ok {
		s.item = item
	}

	s.router.SetGuard(func() (bool, tea.Cmd) {
		if !s.dirty {
			return true, nil
		}
		s.notice = "unsaved changes: press s to save"
		return false, nil
	})

	return nil
}

func (s *detailScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	switch key.String() {
	case "d":
		s.dirty = true
		s.notice = "change pending"
	case "s":
		s.dirty = false
		s.notice = "saved"
	case "esc":
		return s, s.router.Pop()
	case "q", "ctrl+c":
		return s, tea.Quit
	}
	return s, nil
}

func (s *detailScreen) View() tea.View {
	return tea.NewView(fmt.Sprintf(
		"Detail of %s\n\n%s\n\n%s\n\nd  mark change\ns  save\nesc  back\nq  quit",
		s.item, s.router.Breadcrumb(s.width), s.notice,
	))
}

func (s *detailScreen) Title() string { return "Detail" }

func (s *detailScreen) SetSize(width, height int) {
	s.width, s.height = width, height
}

func main() {
	list := &listScreen{}
	r := router.New(list)
	list.router = r

	if _, err := tea.NewProgram(r).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

Every screen implements `router.Screen`. `SetSize` is optional: the router calls it
on every screen in the stack that has the method.

## API

| Symbol | Description |
|---|---|
| `type Screen` | `Init`, `Update`, `View`, `Title` — a navigable screen |
| `New(root Screen) *Router` | creates the stack with `root` at the base |
| `(*Router) Push(s Screen, params any) tea.Cmd` | stacks `s`, stores `params`, applies the known size and returns `s.Init()` |
| `(*Router) Pop() tea.Cmd` | pops the top; asks the top's guard first; does nothing at depth 1 |
| `(*Router) Replace(s Screen) tea.Cmd` | replaces the top, keeping the entry params and dropping its guard |
| `(*Router) Reset() tea.Cmd` | drops every screen above the base |
| `(*Router) Current() Screen` | top screen, or `nil` with an empty stack |
| `(*Router) Depth() int` | number of screens in the stack |
| `(*Router) Titles() []string` | titles from base to top (a copy) |
| `type Guard func() (bool, tea.Cmd)` | `(true, nil)` allows leaving, `(false, cmd)` blocks it |
| `(*Router) SetGuard(g Guard)` / `ClearGuard()` | attaches or removes the guard of the current screen |
| `(*Router) Breadcrumb(width int) string` | renders `A › B › C`, eliding the middle when needed |
| `(*Router) Params() any` | params passed to the `Push` that created the current screen |
| `ParamsAs[T any](r *Router) (T, bool)` | typed helper for `Params` |
| `(*Router) Init/Update/View` | `tea.Model` implementation, delegating to the top |
| `(*Router) SetSize(width, height int)` | stores the size and forwards it to every screen that implements `SetSize` |

### Breadcrumb

`Breadcrumb(width)` styles the current screen and never exceeds `width` display
columns:

| Situation | Result |
|---|---|
| fits | `Home › Settings › Profile` |
| does not fit | `Home › … › Profile` |
| base + ellipsis still does not fit | `… › Profile` |
| only the current title fits | `Profile` (truncated with `ansi.Truncate`) |
| `width <= 0` | `""` |

Titles are normalized to a single line (tabs and line breaks become spaces).

## Design notes

- **No locks, by design.** Bubble Tea processes messages on a single goroutine, so
  the router is not safe for concurrent use. Do not share a router between
  programs.
- **The guard belongs to the screen that set it.** Pushing another screen does not
  make the previous guard block the new screen; it is consulted again if the stack
  returns to that screen. A guard set on the base screen is never consulted,
  because `Pop` at depth 1 is a no-op.
- **`Replace` keeps the entry params.** There is no params argument in `Replace`,
  so the position keeps what the original `Push` stored.
- **`Reset` does not re-run `Init`.** The base screen is the same instance that was
  already in the stack, with its current state.
- **`New(nil)` is tolerated.** The stack starts empty (`Depth` 0) and `Current`
  returns nil. The first `Push` becomes the base and, like any base, cannot be
  popped.
- **Resize is sticky.** `WindowSizeMsg` is stored, broadcast to every screen in the
  stack that implements `SetSize`, and then delivered to the top screen's `Update`.
  Screens pushed later receive the stored size before `Init` runs.
- **Navigation inside `Update` wins.** If a screen calls `Push`, `Pop`, `Replace` or
  `Reset` from its own `Update`, the `Screen` it returns is discarded; the resulting
  stack is what matters. Batch the returned command with your own if needed.
- **Only the top screen receives messages.** Screens below the top are not updated
  while hidden — the exception is the size broadcast above.

## Limitations

- No screen transition or animation hooks.
- No teardown/dispose callback; use a guard plus explicit cleanup if a screen owns
  resources.
- No focus stack: background screens cannot react to messages.
- The breadcrumb collapses the whole middle into a single `…`; it does not try to
  fit as many intermediate titles as possible.

## License

MIT
