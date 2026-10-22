# charm-router

Uma pilha de telas para Bubble Tea v2: navegação push/pop, guards de saída, parâmetros tipados e breadcrumb que respeita o redimensionamento.

> [Read in English](README.md)

## Instalação

```bash
go get github.com/junhinhow/charm-router
```

Requer Go 1.25 ou superior e Bubble Tea v2.

## Uso

O router é o modelo raiz do programa. Telas que precisam navegar guardam uma
referência ao router e chamam `Push`, `Pop`, `Replace` ou `Reset` de dentro do
próprio `Update`.

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/junhinhow/charm-router"
)

// listaScreen é a tela base: mostra a lista e abre o detalhe.
type listaScreen struct {
	router *router.Router
	width  int
	height int
}

func (s *listaScreen) Init() tea.Cmd { return nil }

func (s *listaScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	switch key.String() {
	case "enter":
		return s, s.router.Push(&detalheScreen{router: s.router}, "item-42")
	case "q", "ctrl+c":
		return s, tea.Quit
	}
	return s, nil
}

func (s *listaScreen) View() tea.View {
	linhas := []string{
		fmt.Sprintf("Lista (%dx%d)", s.width, s.height),
		"",
		"enter  abrir detalhe",
		"q      sair",
	}
	return tea.NewView(strings.Join(linhas, "\n"))
}

func (s *listaScreen) Title() string { return "Lista" }

func (s *listaScreen) SetSize(width, height int) {
	s.width, s.height = width, height
}

// detalheScreen recebe um parâmetro no Push e protege a saída com um guard.
type detalheScreen struct {
	router *router.Router
	item   string
	dirty  bool
	aviso  string
	width  int
	height int
}

func (s *detalheScreen) Init() tea.Cmd {
	// O Push já empilhou a tela, então os parâmetros já estão disponíveis.
	if item, ok := router.ParamsAs[string](s.router); ok {
		s.item = item
	}

	s.router.SetGuard(func() (bool, tea.Cmd) {
		if !s.dirty {
			return true, nil
		}
		s.aviso = "há alterações não salvas: pressione s para salvar"
		return false, nil
	})

	return nil
}

func (s *detalheScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	switch key.String() {
	case "d":
		s.dirty = true
		s.aviso = "alteração pendente"
	case "s":
		s.dirty = false
		s.aviso = "salvo"
	case "esc":
		return s, s.router.Pop()
	case "q", "ctrl+c":
		return s, tea.Quit
	}
	return s, nil
}

func (s *detalheScreen) View() tea.View {
	return tea.NewView(fmt.Sprintf(
		"Detalhe de %s\n\n%s\n\n%s\n\nd  marcar alteração\ns  salvar\nesc  voltar\nq  sair",
		s.item, s.router.Breadcrumb(s.width), s.aviso,
	))
}

func (s *detalheScreen) Title() string { return "Detalhe" }

func (s *detalheScreen) SetSize(width, height int) {
	s.width, s.height = width, height
}

func main() {
	lista := &listaScreen{}
	r := router.New(lista)
	lista.router = r

	if _, err := tea.NewProgram(r).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
```

Toda tela implementa `router.Screen`. `SetSize` é opcional: o router o chama em
todas as telas da pilha que tiverem o método.

## API

| Símbolo | Descrição |
|---|---|
| `type Screen` | `Init`, `Update`, `View`, `Title` — uma tela navegável |
| `New(root Screen) *Router` | cria a pilha com `root` na base |
| `(*Router) Push(s Screen, params any) tea.Cmd` | empilha `s`, guarda `params`, aplica o tamanho conhecido e devolve `s.Init()` |
| `(*Router) Pop() tea.Cmd` | desempilha o topo; consulta o guard do topo antes; não faz nada na profundidade 1 |
| `(*Router) Replace(s Screen) tea.Cmd` | substitui o topo, mantendo os params da posição e descartando o guard |
| `(*Router) Reset() tea.Cmd` | remove todas as telas acima da base |
| `(*Router) Current() Screen` | tela do topo, ou `nil` com a pilha vazia |
| `(*Router) Depth() int` | número de telas na pilha |
| `(*Router) Titles() []string` | títulos da base ao topo (uma cópia) |
| `type Guard func() (bool, tea.Cmd)` | `(true, nil)` permite sair, `(false, cmd)` bloqueia |
| `(*Router) SetGuard(g Guard)` / `ClearGuard()` | associa ou remove o guard da tela atual |
| `(*Router) Breadcrumb(width int) string` | renderiza `A › B › C`, elidindo o meio quando necessário |
| `(*Router) Params() any` | params passados ao `Push` que criou a tela atual |
| `ParamsAs[T any](r *Router) (T, bool)` | helper tipado para `Params` |
| `(*Router) Init/Update/View` | implementação de `tea.Model`, delegando ao topo |
| `(*Router) SetSize(width, height int)` | guarda o tamanho e o repassa a todas as telas que implementam `SetSize` |

### Breadcrumb

`Breadcrumb(width)` estiliza a tela atual e nunca ultrapassa `width` colunas de
exibição:

| Situação | Resultado |
|---|---|
| cabe | `Início › Configurações › Perfil` |
| não cabe | `Início › … › Perfil` |
| base + reticências ainda não cabem | `… › Perfil` |
| só o título atual cabe | `Perfil` (truncado com `ansi.Truncate`) |
| `width <= 0` | `""` |

Os títulos são normalizados para uma única linha (tabs e quebras de linha viram
espaços).

## Notas de projeto

- **Sem locks, por decisão.** O Bubble Tea processa mensagens em uma única
  goroutine, então o router não é seguro para uso concorrente. Não compartilhe um
  router entre programas.
- **O guard pertence à tela que o definiu.** Empilhar outra tela não faz o guard
  anterior bloquear a nova tela; ele volta a ser consultado se a pilha retornar
  àquela tela. Um guard definido na tela base nunca é consultado, porque `Pop` na
  profundidade 1 é um no-op.
- **`Replace` mantém os params da posição.** `Replace` não recebe params, então a
  posição continua com o que o `Push` original guardou.
- **`Reset` não roda `Init` de novo.** A tela base é a mesma instância que já
  estava na pilha, com o estado atual.
- **O tamanho é pegajoso.** `WindowSizeMsg` é guardado, repassado a todas as telas
  da pilha que implementam `SetSize` e, só então, entregue ao `Update` do topo.
  Telas empilhadas depois recebem o tamanho guardado antes do `Init`.
- **Navegação dentro do `Update` vence.** Se uma tela chamar `Push`, `Pop`,
  `Replace` ou `Reset` no próprio `Update`, o `Screen` devolvido por ela é
  descartado; vale a pilha resultante da navegação. Combine o comando devolvido
  com os seus próprios usando `tea.Batch` quando precisar.
- **Só o topo recebe mensagens.** As telas abaixo do topo não são atualizadas
  enquanto estão escondidas — a exceção é o broadcast de tamanho acima.

## Limitações

- Sem hooks de transição ou animação entre telas.
- Sem callback de descarte; use um guard e limpeza explícita se a tela possuir
  recursos.
- Sem pilha de foco: telas em segundo plano não reagem a mensagens.
- O breadcrumb colapsa todo o meio em um único `…`; ele não tenta encaixar o maior
  número possível de títulos intermediários.

## Licença

MIT
