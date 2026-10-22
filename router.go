// Package router mantém uma pilha de telas para aplicações Bubble Tea.
//
// O [Router] é o modelo raiz do programa: delega os eventos à tela do topo,
// guarda o tamanho do terminal e o repassa a todas as telas da pilha, e expõe
// as operações de navegação [Router.Push], [Router.Pop], [Router.Replace] e
// [Router.Reset].
//
// O Router não é seguro para uso concorrente. O Bubble Tea processa mensagens
// em uma única goroutine, então essa restrição é intencional e evita locks no
// caminho quente de Update.
package router

import "charm.land/bubbletea/v2"

// Screen é uma tela navegável.
type Screen interface {
	// Init é chamado quando a tela entra na pilha e devolve um comando inicial.
	Init() tea.Cmd

	// Update processa uma mensagem e devolve a tela atualizada.
	//
	// A tela devolvida substitui a atual na pilha. Se a tela navegar durante o
	// Update (Push, Pop, Replace ou Reset), o valor devolvido é descartado:
	// vale a pilha resultante da navegação.
	Update(tea.Msg) (Screen, tea.Cmd)

	// View devolve o conteúdo renderizado da tela.
	View() tea.View

	// Title é o nome exibido em breadcrumbs/pilha.
	Title() string
}

// Guard impede a saída de uma tela (ex.: "alterações não salvas").
//
// Devolve (true, nil) para permitir a saída ou (false, cmd) para bloqueá-la.
// No caso bloqueado, cmd pode ser usado, por exemplo, para exibir um diálogo.
type Guard func() (bool, tea.Cmd)

// Router mantém a pilha de telas e delega os eventos à tela do topo.
//
// Use [New] para criar um Router e passe-o como modelo raiz do programa. As
// telas que precisam navegar guardam uma referência ao Router e chamam Push,
// Pop, Replace ou Reset de dentro do próprio Update.
type Router struct {
	stack   []entry
	width   int
	height  int
	hasSize bool

	// gen muda a cada operação que altera a pilha. Update usa o valor para
	// detectar navegação feita pela própria tela e não sobrescrever o topo.
	gen uint64
}

// entry é um item da pilha: a tela, os parâmetros do Push e o guard da tela.
type entry struct {
	screen Screen
	params any
	guard  Guard
}

// Router implementa tea.Model.
var _ tea.Model = (*Router)(nil)

// New cria um Router com root como tela base.
//
// New não chama Init: o Bubble Tea chama [Router.Init], que delega à tela atual.
// Se root for nil, o Router começa com a pilha vazia; nesse caso [Router.Current]
// devolve nil e as demais operações continuam seguras.
func New(root Screen) *Router {
	r := &Router{}
	if root != nil {
		r.stack = append(r.stack, entry{screen: root})
	}
	return r
}

// Push empilha s e devolve o comando de Init da nova tela.
//
// params fica disponível para a tela via [Router.Params] e [ParamsAs]. Se o
// tamanho do terminal já for conhecido, ele é aplicado a s antes de Init. Um
// Screen nil é ignorado e Push devolve nil.
func (r *Router) Push(s Screen, params any) tea.Cmd {
	if s == nil {
		return nil
	}

	r.stack = append(r.stack, entry{screen: s, params: params})
	r.gen++
	r.applySize(s)
	return s.Init()
}

// Pop desempilha a tela do topo e devolve o comando do guard quando ele bloqueia.
//
// A tela base nunca é removida: com profundidade 1, Pop não faz nada. O guard
// consultado é o da tela do topo (ver [Router.SetGuard]).
func (r *Router) Pop() tea.Cmd {
	if len(r.stack) <= 1 {
		return nil
	}

	var cmd tea.Cmd
	if guard := r.stack[len(r.stack)-1].guard; guard != nil {
		ok, guardCmd := guard()
		if !ok {
			return guardCmd
		}
		cmd = guardCmd
	}

	r.dropTop()
	return cmd
}

// dropTop remove a tela do topo, limpando a referência para permitir coleta.
func (r *Router) dropTop() {
	last := len(r.stack) - 1
	r.stack[last] = entry{}
	r.stack = r.stack[:last]
	r.gen++
}

// Replace substitui a tela do topo por s e devolve o comando de Init de s.
//
// Os parâmetros da posição são preservados e o guard da tela substituída é
// descartado. Com a pilha vazia, Replace empilha s como tela base. Um Screen nil
// é ignorado e Replace devolve nil.
func (r *Router) Replace(s Screen) tea.Cmd {
	if s == nil {
		return nil
	}

	if len(r.stack) == 0 {
		r.stack = append(r.stack, entry{screen: s})
	} else {
		last := len(r.stack) - 1
		r.stack[last] = entry{screen: s, params: r.stack[last].params}
	}
	r.gen++
	r.applySize(s)
	return s.Init()
}

// Reset limpa a pilha e deixa apenas a tela base.
//
// Reset não consulta o guard nem chama Init novamente: a tela base é a mesma
// instância que já estava na pilha, com o estado que tiver.
func (r *Router) Reset() tea.Cmd {
	if len(r.stack) <= 1 {
		return nil
	}

	for i := 1; i < len(r.stack); i++ {
		r.stack[i] = entry{}
	}
	r.stack = r.stack[:1]
	r.gen++
	return nil
}

// Current devolve a tela do topo, ou nil se a pilha estiver vazia.
func (r *Router) Current() Screen {
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1].screen
}

// Depth devolve o número de telas na pilha (mínimo 1 quando há tela base).
func (r *Router) Depth() int {
	return len(r.stack)
}

// Titles devolve os títulos das telas da base ao topo.
//
// O slice devolvido é uma cópia: alterá-lo não muda o Router.
func (r *Router) Titles() []string {
	titles := make([]string, len(r.stack))
	for i, e := range r.stack {
		if e.screen != nil {
			titles[i] = e.screen.Title()
		}
	}
	return titles
}

// SetGuard associa g à tela do topo. O guard é consultado apenas em Pop.
//
// Como o guard pertence à tela em que foi definido, empilhar outra tela não faz
// com que o guard anterior bloqueie a nova tela; ele volta a valer se a pilha
// retornar àquela tela. Com a pilha vazia, SetGuard não faz nada.
func (r *Router) SetGuard(g Guard) {
	if len(r.stack) == 0 {
		return
	}
	r.stack[len(r.stack)-1].guard = g
}

// ClearGuard remove o guard da tela do topo.
func (r *Router) ClearGuard() {
	if len(r.stack) == 0 {
		return
	}
	r.stack[len(r.stack)-1].guard = nil
}

// Params devolve os parâmetros passados no Push que criou a tela atual.
//
// A tela base não tem parâmetros: devolve nil.
func (r *Router) Params() any {
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1].params
}

// ParamsAs devolve os parâmetros da tela atual convertidos para T.
//
// O segundo valor é false quando não há parâmetros ou quando eles não são do
// tipo T. Um Router nil também devolve o valor zero de T e false.
func ParamsAs[T any](r *Router) (T, bool) {
	var zero T
	if r == nil {
		return zero, false
	}

	params := r.Params()
	if params == nil {
		return zero, false
	}

	value, ok := params.(T)
	if !ok {
		return zero, false
	}
	return value, true
}

// Init delega à tela atual, conforme o contrato de [tea.Model].
func (r *Router) Init() tea.Cmd {
	if current := r.Current(); current != nil {
		return current.Init()
	}
	return nil
}

// Update delega msg à tela do topo e devolve o próprio Router como modelo.
//
// WindowSizeMsg é guardado e repassado a todas as telas da pilha que implementam
// interface{ SetSize(int, int) } antes de chegar ao topo, para que uma tela já
// esteja dimensionada quando voltar a ser a atual.
func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		r.setSize(size.Width, size.Height)
	}

	current := r.Current()
	if current == nil {
		return r, nil
	}

	gen := r.gen
	next, cmd := current.Update(msg)
	if r.gen == gen && next != nil {
		r.stack[len(r.stack)-1].screen = next
	}
	return r, cmd
}

// View renderiza a tela do topo. Com a pilha vazia, devolve uma view vazia.
func (r *Router) View() tea.View {
	if current := r.Current(); current != nil {
		return current.View()
	}
	return tea.NewView("")
}

// SetSize guarda o tamanho do terminal e o aplica a todas as telas da pilha que
// implementam interface{ SetSize(int, int) }.
//
// Valores negativos são tratados como zero. O tamanho guardado também é aplicado
// às telas empilhadas depois dele (ver [Router.Push] e [Router.Replace]).
func (r *Router) SetSize(width, height int) {
	r.setSize(width, height)
}

// setSize é a implementação comum de SetSize e do tratamento de WindowSizeMsg.
func (r *Router) setSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	r.width, r.height = width, height
	r.hasSize = true

	for _, e := range r.stack {
		r.applySize(e.screen)
	}
}

// applySize aplica o tamanho guardado a s, se ele já for conhecido e se a tela
// implementar SetSize.
func (r *Router) applySize(s Screen) {
	if !r.hasSize || s == nil {
		return
	}
	if sized, ok := s.(interface{ SetSize(int, int) }); ok {
		sized.SetSize(r.width, r.height)
	}
}
