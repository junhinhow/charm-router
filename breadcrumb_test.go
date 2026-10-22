package router

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// pilhaComTitulos monta um Router com as telas na ordem informada.
func pilhaComTitulos(titles ...string) *Router {
	r := New(&fakeScreen{title: titles[0]})
	for _, title := range titles[1:] {
		r.Push(&fakeScreen{title: title}, nil)
	}
	return r
}

func TestBreadcrumb(t *testing.T) {
	tests := []struct {
		name   string
		titles []string
		width  int
		want   string
	}{
		{
			name:   "uma tela",
			titles: []string{"Início"},
			width:  80,
			want:   "Início",
		},
		{
			name:   "duas telas",
			titles: []string{"Início", "Configurações"},
			width:  80,
			want:   "Início › Configurações",
		},
		{
			name:   "cinco telas",
			titles: []string{"A", "B", "C", "D", "E"},
			width:  80,
			want:   "A › B › C › D › E",
		},
		{
			name:   "cinco telas com o meio elidido",
			titles: []string{"A", "B", "C", "D", "E"},
			width:  14,
			want:   "A › … › E",
		},
		{
			name:   "meio elidido no limite exato",
			titles: []string{"A", "B", "C", "D", "E"},
			width:  9,
			want:   "A › … › E",
		},
		{
			name:   "preserva a tela atual quando a base não cabe",
			titles: []string{"A", "B", "C", "D", "E"},
			width:  8,
			want:   "… › E",
		},
		{
			name:   "trunca a tela atual quando só ela cabe",
			titles: []string{"A", "B", "C", "D", "E"},
			width:  4,
			want:   "E",
		},
		{
			name:   "trunca a tela atual longa",
			titles: []string{"Início", "Configurações", "Perfil"},
			width:  12,
			want:   "… › Perfil",
		},
		{
			name:   "trunca a última tela em duas telas",
			titles: []string{"Início", "Configurações"},
			width:  12,
			want:   "Início › Co…",
		},
		{
			name:   "trunca uma única tela",
			titles: []string{"Início"},
			width:  5,
			want:   "Iníc…",
		},
		{
			name:   "largura zero",
			titles: []string{"A", "B"},
			width:  0,
			want:   "",
		},
		{
			name:   "largura negativa",
			titles: []string{"A"},
			width:  -3,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := pilhaComTitulos(tt.titles...)
			got := ansi.Strip(r.Breadcrumb(tt.width))
			if got != tt.want {
				t.Fatalf("Breadcrumb(%d) = %q, esperado %q", tt.width, got, tt.want)
			}
		})
	}
}

func TestBreadcrumbVazioSemTelas(t *testing.T) {
	if got := New(nil).Breadcrumb(80); got != "" {
		t.Fatalf("Breadcrumb() = %q, esperado vazio", got)
	}
}

func TestBreadcrumbNuncaExcedeALargura(t *testing.T) {
	r := pilhaComTitulos("Início", "Configurações", "Usuários", "Detalhes", "Editar")

	for width := 1; width <= 60; width++ {
		got := r.Breadcrumb(width)
		if got == "" {
			t.Fatalf("Breadcrumb(%d) devolveu string vazia", width)
		}
		if w := lipgloss.Width(got); w > width {
			t.Fatalf("Breadcrumb(%d) tem largura %d: %q", width, w, ansi.Strip(got))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("Breadcrumb(%d) produziu UTF-8 inválido: %q", width, got)
		}
	}
}

func TestBreadcrumbTruncaCaracteresLargos(t *testing.T) {
	r := pilhaComTitulos("日本語")

	got := r.Breadcrumb(2)
	if w := lipgloss.Width(got); w > 2 {
		t.Fatalf("Breadcrumb(2) tem largura %d: %q", w, ansi.Strip(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("Breadcrumb(2) produziu UTF-8 inválido: %q", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("Breadcrumb(2) cortou um caractere no meio: %q", got)
	}
}

func TestBreadcrumbNormalizaTitulosEmUmaLinha(t *testing.T) {
	r := New(&fakeScreen{title: "Linha\nDuas\tColunas"})

	got := ansi.Strip(r.Breadcrumb(80))
	if got != "Linha Duas Colunas" {
		t.Fatalf("Breadcrumb() = %q, esperado %q", got, "Linha Duas Colunas")
	}
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("Breadcrumb() manteve quebra de linha: %q", got)
	}
}

func TestBreadcrumbAcompanhaAPilha(t *testing.T) {
	r := pilhaComTitulos("A", "B")

	r.Pop()
	if got := ansi.Strip(r.Breadcrumb(80)); got != "A" {
		t.Fatalf("Breadcrumb() após Pop = %q, esperado A", got)
	}

	r.Push(&fakeScreen{title: "C"}, nil)
	if got := ansi.Strip(r.Breadcrumb(80)); got != "A › C" {
		t.Fatalf("Breadcrumb() após Push = %q, esperado A › C", got)
	}
}
