package router

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// breadcrumbSeparator separa os títulos no breadcrumb.
const breadcrumbSeparator = " › "

// breadcrumbEllipsis marca as telas omitidas no meio do breadcrumb.
const breadcrumbEllipsis = "…"

var (
	// breadcrumbInactiveStyle é usado nos títulos das telas anteriores.
	breadcrumbInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C6C6C"))

	// breadcrumbActiveStyle destaca a tela atual.
	breadcrumbActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))

	// titleSanitizer mantém cada título em uma única linha.
	titleSanitizer = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
)

// Breadcrumb renderiza a trilha de telas como "A › B › C".
//
// A tela atual (topo) recebe destaque. Quando a trilha não cabe em width, as
// telas do meio são substituídas por "…" (A › … › Z); se ainda assim não couber,
// o título atual é truncado com [ansi.Truncate]. Larguras menores ou iguais a
// zero devolvem string vazia.
//
// Títulos são normalizados para uma única linha antes da renderização.
func (r *Router) Breadcrumb(width int) string {
	if width <= 0 {
		return ""
	}

	titles := r.Titles()
	if len(titles) == 0 {
		return ""
	}
	for i, title := range titles {
		titles[i] = titleSanitizer.Replace(title)
	}

	last := len(titles) - 1
	render := func(index int, text string) string {
		if index == last {
			return breadcrumbActiveStyle.Render(text)
		}
		return breadcrumbInactiveStyle.Render(text)
	}

	parts := make([]string, len(titles))
	for i, title := range titles {
		parts[i] = render(i, title)
	}

	full := strings.Join(parts, breadcrumbSeparator)
	if lipgloss.Width(full) <= width {
		return full
	}

	if len(titles) == 1 {
		return render(last, ansi.Truncate(titles[last], width, breadcrumbEllipsis))
	}

	if len(titles) == 2 {
		prefix := parts[0] + breadcrumbSeparator
		if available := width - lipgloss.Width(prefix); available > 0 {
			return prefix + render(last, ansi.Truncate(titles[last], available, breadcrumbEllipsis))
		}
		return render(last, ansi.Truncate(titles[last], width, breadcrumbEllipsis))
	}

	ellipsis := breadcrumbInactiveStyle.Render(breadcrumbEllipsis)
	collapsed := parts[0] + breadcrumbSeparator + ellipsis + breadcrumbSeparator + parts[last]
	if lipgloss.Width(collapsed) <= width {
		return collapsed
	}

	prefix := parts[0] + breadcrumbSeparator + ellipsis + breadcrumbSeparator
	if available := width - lipgloss.Width(prefix); available > 0 {
		return prefix + render(last, ansi.Truncate(titles[last], available, breadcrumbEllipsis))
	}

	// Nem a tela base cabe junto do indicador de omissão: preserva a tela atual.
	short := ellipsis + breadcrumbSeparator + parts[last]
	if lipgloss.Width(short) <= width {
		return short
	}

	return render(last, ansi.Truncate(titles[last], width, breadcrumbEllipsis))
}
