package router_test

import (
	"fmt"

	"charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/junhinhow/charm-router"
)

// exampleScreen é uma tela mínima usada nos exemplos.
type exampleScreen struct{ title string }

func (s exampleScreen) Init() tea.Cmd                           { return nil }
func (s exampleScreen) Update(tea.Msg) (router.Screen, tea.Cmd) { return s, nil }
func (s exampleScreen) View() tea.View                          { return tea.NewView(s.title) }
func (s exampleScreen) Title() string                           { return s.title }

func ExampleRouter() {
	r := router.New(exampleScreen{title: "Início"})
	r.Push(exampleScreen{title: "Configurações"}, nil)

	fmt.Println(r.Depth())
	fmt.Println(r.Titles())
	fmt.Println(ansi.Strip(r.Breadcrumb(80)))

	// Output:
	// 2
	// [Início Configurações]
	// Início › Configurações
}

func ExampleRouter_Breadcrumb() {
	r := router.New(exampleScreen{title: "Início"})
	for _, title := range []string{"Configurações", "Usuários", "Detalhes"} {
		r.Push(exampleScreen{title: title}, nil)
	}

	fmt.Println(ansi.Strip(r.Breadcrumb(80)))
	fmt.Println(ansi.Strip(r.Breadcrumb(12)))

	// Output:
	// Início › Configurações › Usuários › Detalhes
	// … › Detalhes
}

func ExampleRouter_SetGuard() {
	r := router.New(exampleScreen{title: "Editor"})
	r.Push(exampleScreen{title: "Ajuda"}, nil)

	// Há alterações não salvas: a saída fica bloqueada.
	r.SetGuard(func() (bool, tea.Cmd) { return false, nil })
	r.Pop()
	fmt.Println(r.Depth())

	// Depois de salvar, a saída é liberada.
	r.ClearGuard()
	r.Pop()
	fmt.Println(r.Depth())

	// Output:
	// 2
	// 1
}

func ExampleParamsAs() {
	r := router.New(exampleScreen{title: "Início"})
	r.Push(exampleScreen{title: "Detalhe"}, struct{ ID int }{ID: 42})

	params, ok := router.ParamsAs[struct{ ID int }](r)
	fmt.Println(params.ID, ok)

	// Output:
	// 42 true
}
