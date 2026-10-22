package router

import (
	"reflect"
	"testing"

	"charm.land/bubbletea/v2"
)

// size registra uma chamada de SetSize.
type size struct{ width, height int }

// fakeScreen é uma tela de teste que registra tudo o que recebe.
type fakeScreen struct {
	title     string
	view      string
	initCmd   tea.Cmd
	initCalls int
	updates   []tea.Msg
	sizes     []size
	width     int
	height    int
	onUpdate  func(*fakeScreen, tea.Msg) (Screen, tea.Cmd)
}

func (s *fakeScreen) Init() tea.Cmd {
	s.initCalls++
	return s.initCmd
}

func (s *fakeScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	s.updates = append(s.updates, msg)
	if s.onUpdate != nil {
		return s.onUpdate(s, msg)
	}
	return s, nil
}

func (s *fakeScreen) View() tea.View { return tea.NewView(s.view) }

func (s *fakeScreen) Title() string { return s.title }

func (s *fakeScreen) SetSize(width, height int) {
	s.width, s.height = width, height
	s.sizes = append(s.sizes, size{width: width, height: height})
}

// plainScreen não implementa SetSize: serve para garantir que o Router tolera
// telas sem a interface opcional.
type plainScreen struct{ title string }

func (s *plainScreen) Init() tea.Cmd                    { return nil }
func (s *plainScreen) Update(tea.Msg) (Screen, tea.Cmd) { return s, nil }
func (s *plainScreen) View() tea.View                   { return tea.NewView(s.title) }
func (s *plainScreen) Title() string                    { return s.title }

// pingMsg é uma mensagem qualquer para exercitar a delegação de Update.
type pingMsg struct{}

func TestNew(t *testing.T) {
	root := &fakeScreen{title: "raiz"}

	r := New(root)
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
	if r.Current() != root {
		t.Fatalf("Current() não é a tela base")
	}
	if got := r.Titles(); !reflect.DeepEqual(got, []string{"raiz"}) {
		t.Fatalf("Titles() = %v, esperado [raiz]", got)
	}
	if got := r.Params(); got != nil {
		t.Fatalf("Params() da base = %v, esperado nil", got)
	}
	if root.initCalls != 0 {
		t.Fatalf("New chamou Init da base %d vez(es), esperado 0", root.initCalls)
	}
	if got := r.Init(); got != nil {
		t.Fatalf("Init() da base = %v, esperado nil", got)
	}
	if root.initCalls != 1 {
		t.Fatalf("Init da base chamado %d vez(es), esperado 1", root.initCalls)
	}
}

func TestPushPopOrder(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}
	b := &fakeScreen{title: "b"}

	r := New(root)
	r.Push(a, nil)
	r.Push(b, nil)

	if got := r.Depth(); got != 3 {
		t.Fatalf("Depth() = %d, esperado 3", got)
	}
	if got := r.Titles(); !reflect.DeepEqual(got, []string{"raiz", "a", "b"}) {
		t.Fatalf("Titles() = %v, esperado [raiz a b]", got)
	}
	if r.Current() != b {
		t.Fatalf("Current() não é a tela do topo")
	}

	r.Pop()
	if got := r.Depth(); got != 2 {
		t.Fatalf("Depth() após Pop = %d, esperado 2", got)
	}
	if r.Current() != a {
		t.Fatalf("Current() após Pop não é a tela anterior")
	}
	if got := r.Titles(); !reflect.DeepEqual(got, []string{"raiz", "a"}) {
		t.Fatalf("Titles() após Pop = %v, esperado [raiz a]", got)
	}

	r.Pop()
	if r.Current() != root {
		t.Fatalf("Current() após segundo Pop não é a base")
	}
}

func TestPushChamaInitDaNovaTela(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a", initCmd: func() tea.Msg { return "init-a" }}

	r := New(root)
	cmd := r.Push(a, nil)

	if a.initCalls != 1 {
		t.Fatalf("Init da tela empilhada chamado %d vez(es), esperado 1", a.initCalls)
	}
	if root.initCalls != 0 {
		t.Fatalf("Init da base chamado %d vez(es), esperado 0", root.initCalls)
	}
	if cmd == nil {
		t.Fatal("Push devolveu nil, esperado o comando de Init")
	}
	if got := cmd(); got != "init-a" {
		t.Fatalf("comando de Init = %v, esperado init-a", got)
	}
}

func TestPopNaRaizNaoFazNada(t *testing.T) {
	root := &fakeScreen{title: "raiz"}

	r := New(root)
	r.SetGuard(func() (bool, tea.Cmd) {
		t.Error("guard da raiz foi consultado, mas Pop na raiz não deveria fazer nada")
		return false, nil
	})
	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop na raiz devolveu %v, esperado nil", cmd)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
	if r.Current() != root {
		t.Fatal("Pop na raiz removeu a tela base")
	}
}

func TestPushEReplaceIgnoramTelaNil(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	r := New(root)

	if cmd := r.Push(nil, nil); cmd != nil {
		t.Fatalf("Push(nil) devolveu %v, esperado nil", cmd)
	}
	if cmd := r.Replace(nil); cmd != nil {
		t.Fatalf("Replace(nil) devolveu %v, esperado nil", cmd)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
}

func TestGuardBloqueiaELibera(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	editor := &fakeScreen{title: "editor"}

	r := New(root)
	r.Push(editor, nil)

	blockCmd := func() tea.Msg { return "bloqueado" }
	calls := 0
	r.SetGuard(func() (bool, tea.Cmd) {
		calls++
		if calls == 1 {
			return false, blockCmd
		}
		return true, nil
	})

	cmd := r.Pop()
	if calls != 1 {
		t.Fatalf("guard chamado %d vez(es), esperado 1", calls)
	}
	if cmd == nil {
		t.Fatal("Pop bloqueado devolveu nil, esperado o comando do guard")
	}
	if got := cmd(); got != "bloqueado" {
		t.Fatalf("comando do guard = %v, esperado bloqueado", got)
	}
	if got := r.Depth(); got != 2 {
		t.Fatalf("Depth() após Pop bloqueado = %d, esperado 2", got)
	}
	if r.Current() != editor {
		t.Fatal("Pop bloqueado removeu a tela")
	}

	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop liberado devolveu %v, esperado nil", cmd)
	}
	if calls != 2 {
		t.Fatalf("guard chamado %d vez(es), esperado 2", calls)
	}
	if r.Current() != root {
		t.Fatal("Pop liberado não voltou para a tela anterior")
	}
}

func TestGuardAprovadoComComando(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, nil)

	saved := func() tea.Msg { return "salvo" }
	r.SetGuard(func() (bool, tea.Cmd) { return true, saved })

	cmd := r.Pop()
	if cmd == nil {
		t.Fatal("Pop liberado devolveu nil, esperado o comando do guard")
	}
	if got := cmd(); got != "salvo" {
		t.Fatalf("comando do guard = %v, esperado salvo", got)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
}

func TestGuardNaoEConsultadoEmPushEReplace(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}
	b := &fakeScreen{title: "b"}

	r := New(root)
	calls := 0
	r.SetGuard(func() (bool, tea.Cmd) {
		calls++
		return false, nil
	})

	r.Push(a, nil)
	if calls != 0 {
		t.Fatalf("guard consultado em Push %d vez(es), esperado 0", calls)
	}

	r.Replace(b)
	if calls != 0 {
		t.Fatalf("guard consultado em Replace %d vez(es), esperado 0", calls)
	}

	r.Reset()
	if calls != 0 {
		t.Fatalf("guard consultado em Reset %d vez(es), esperado 0", calls)
	}
}

func TestGuardPertenceATelaAtual(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}
	b := &fakeScreen{title: "b"}

	r := New(root)
	r.Push(a, nil)

	calls := 0
	r.SetGuard(func() (bool, tea.Cmd) {
		calls++
		return false, nil
	})

	r.Push(b, nil)
	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop devolveu %v; o guard de %q não deveria bloquear outra tela", cmd, "a")
	}
	if calls != 0 {
		t.Fatalf("guard consultado %d vez(es) ao desempilhar outra tela", calls)
	}

	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop devolveu %v, esperado nil", cmd)
	}
	if calls != 1 {
		t.Fatalf("guard consultado %d vez(es) ao voltar para a tela que o definiu, esperado 1", calls)
	}
	if got := r.Depth(); got != 2 {
		t.Fatalf("Depth() = %d, esperado 2: o guard deve continuar bloqueando a saída", got)
	}
}

func TestClearGuard(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, nil)
	r.SetGuard(func() (bool, tea.Cmd) { return false, nil })
	r.ClearGuard()

	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop devolveu %v, esperado nil após ClearGuard", cmd)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
}

func TestParams(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	if got := r.Params(); got != nil {
		t.Fatalf("Params() na base = %v, esperado nil", got)
	}

	r.Push(a, "item-42")
	if got := r.Params(); got != "item-42" {
		t.Fatalf("Params() = %v, esperado item-42", got)
	}

	r.Pop()
	if got := r.Params(); got != nil {
		t.Fatalf("Params() após Pop = %v, esperado nil", got)
	}
}

func TestParamsAs(t *testing.T) {
	type detalhe struct{ ID int }

	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, detalhe{ID: 7})

	got, ok := ParamsAs[detalhe](r)
	if !ok {
		t.Fatal("ParamsAs[detalhe] devolveu ok = false")
	}
	if got.ID != 7 {
		t.Fatalf("ParamsAs[detalhe].ID = %d, esperado 7", got.ID)
	}

	if _, ok := ParamsAs[string](r); ok {
		t.Fatal("ParamsAs[string] devolveu ok = true para params de outro tipo")
	}
	if _, ok := ParamsAs[detalhe](New(nil)); ok {
		t.Fatal("ParamsAs em pilha vazia devolveu ok = true")
	}
	if _, ok := ParamsAs[detalhe](nil); ok {
		t.Fatal("ParamsAs(nil) devolveu ok = true")
	}
}

func TestWindowSizeBroadcast(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	middle := &fakeScreen{title: "meio"}
	top := &fakeScreen{title: "topo"}

	r := New(root)
	r.Push(middle, nil)
	r.Push(top, nil)

	r.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	for _, s := range []*fakeScreen{root, middle, top} {
		if s.width != 120 || s.height != 40 {
			t.Fatalf("%q ficou %dx%d, esperado 120x40", s.title, s.width, s.height)
		}
		if len(s.sizes) != 1 {
			t.Fatalf("%q recebeu SetSize %d vez(es), esperado 1", s.title, len(s.sizes))
		}
	}

	if len(root.updates) != 0 || len(middle.updates) != 0 {
		t.Fatal("WindowSizeMsg foi entregue a uma tela que não é o topo")
	}
	if len(top.updates) != 1 {
		t.Fatalf("topo recebeu %d mensagem(ns), esperado 1", len(top.updates))
	}
	if _, ok := top.updates[0].(tea.WindowSizeMsg); !ok {
		t.Fatalf("topo recebeu %T, esperado tea.WindowSizeMsg", top.updates[0])
	}
}

func TestWindowSizeReaplicadoAoVoltar(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, nil)
	r.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	r.Pop()
	if r.Current() != root {
		t.Fatal("Pop não voltou para a base")
	}
	if root.width != 100 || root.height != 30 {
		t.Fatalf("base ficou %dx%d após voltar, esperado 100x30", root.width, root.height)
	}

	// A tela empilhada depois do resize também nasce dimensionada.
	late := &fakeScreen{title: "tardia"}
	r.Push(late, nil)
	if late.width != 100 || late.height != 30 {
		t.Fatalf("tela tardia nasceu %dx%d, esperado 100x30", late.width, late.height)
	}
}

func TestTamanhoAplicadoAntesDoInit(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	r := New(root)
	r.SetSize(80, 24)

	var alturaNoInit [2]int
	tela := &initObserverScreen{fakeScreen: &fakeScreen{title: "tardia"}}
	tela.onInit = func() {
		alturaNoInit = [2]int{tela.width, tela.height}
	}

	r.Push(tela, nil)
	if alturaNoInit != [2]int{80, 24} {
		t.Fatalf("tamanho no Init = %v, esperado [80 24]", alturaNoInit)
	}
}

// initObserverScreen observa o tamanho no momento em que Init é chamado.
type initObserverScreen struct {
	*fakeScreen
	onInit func()
}

func (s *initObserverScreen) Init() tea.Cmd {
	if s.onInit != nil {
		s.onInit()
	}
	return s.fakeScreen.Init()
}

func TestSetSizeNegativoViraZero(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	r := New(root)

	r.SetSize(-10, -4)
	if root.width != 0 || root.height != 0 {
		t.Fatalf("base ficou %dx%d, esperado 0x0", root.width, root.height)
	}
}

func TestSetSizeToleraTelaSemInterface(t *testing.T) {
	root := &plainScreen{title: "simples"}
	r := New(root)

	r.SetSize(80, 24)
	r.Update(tea.WindowSizeMsg{Width: 90, Height: 25})

	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
}

func TestUpdateDelegaSomenteAoTopo(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, nil)

	model, _ := r.Update(pingMsg{})
	if model != r {
		t.Fatal("Update não devolveu o próprio Router")
	}
	if len(root.updates) != 0 {
		t.Fatalf("base recebeu %d mensagem(ns), esperado 0", len(root.updates))
	}
	if len(a.updates) != 1 {
		t.Fatalf("topo recebeu %d mensagem(ns), esperado 1", len(a.updates))
	}
}

func TestUpdateGuardaTelaDevolvida(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}

	r := New(root)
	r.Push(a, nil)

	a.onUpdate = func(s *fakeScreen, _ tea.Msg) (Screen, tea.Cmd) {
		clone := *s
		clone.title = "a-atualizada"
		return &clone, nil
	}

	r.Update(pingMsg{})
	if got := r.Current().Title(); got != "a-atualizada" {
		t.Fatalf("Current().Title() = %q, esperado a-atualizada", got)
	}
	if got := r.Depth(); got != 2 {
		t.Fatalf("Depth() = %d, esperado 2", got)
	}
}

func TestUpdateDescartaTelaDevolvidaQuandoTelaNavega(t *testing.T) {
	t.Run("push", func(t *testing.T) {
		root := &fakeScreen{title: "raiz"}
		detail := &fakeScreen{title: "detalhe"}
		pushed := &fakeScreen{title: "empilhada"}

		var r *Router
		detail.onUpdate = func(*fakeScreen, tea.Msg) (Screen, tea.Cmd) {
			r.Push(pushed, nil)
			return &fakeScreen{title: "descartada"}, nil
		}

		r = New(root)
		r.Push(detail, nil)
		r.Update(pingMsg{})

		if r.Current() != pushed {
			t.Fatalf("Current() = %q, esperado empilhada", r.Current().Title())
		}
		if got := r.Depth(); got != 3 {
			t.Fatalf("Depth() = %d, esperado 3", got)
		}
	})

	t.Run("pop", func(t *testing.T) {
		root := &fakeScreen{title: "raiz"}
		detail := &fakeScreen{title: "detalhe"}

		var r *Router
		detail.onUpdate = func(*fakeScreen, tea.Msg) (Screen, tea.Cmd) {
			r.Pop()
			return &fakeScreen{title: "descartada"}, nil
		}

		r = New(root)
		r.Push(detail, nil)
		r.Update(pingMsg{})

		if r.Current() != root {
			t.Fatalf("Current() = %q, esperado raiz", r.Current().Title())
		}
		if got := r.Depth(); got != 1 {
			t.Fatalf("Depth() = %d, esperado 1", got)
		}
	})
}

func TestReplace(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}
	b := &fakeScreen{title: "b", initCmd: func() tea.Msg { return "init-b" }}

	r := New(root)
	r.Push(a, "params-da-posicao")
	r.SetGuard(func() (bool, tea.Cmd) { return false, nil })
	r.SetSize(80, 24)

	cmd := r.Replace(b)
	if r.Current() != b {
		t.Fatal("Replace não trocou a tela do topo")
	}
	if b.width != 80 || b.height != 24 {
		t.Fatalf("tela nova ficou %dx%d, esperado 80x24", b.width, b.height)
	}
	if got := r.Depth(); got != 2 {
		t.Fatalf("Depth() = %d, esperado 2", got)
	}
	if b.initCalls != 1 {
		t.Fatalf("Init da tela nova chamado %d vez(es), esperado 1", b.initCalls)
	}
	if cmd == nil || cmd() != "init-b" {
		t.Fatal("Replace não devolveu o comando de Init da tela nova")
	}
	if got := r.Params(); got != "params-da-posicao" {
		t.Fatalf("Params() após Replace = %v, esperado params-da-posicao", got)
	}
	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop devolveu %v: o guard da tela substituída deveria ter sido descartado", cmd)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
}

func TestReplaceComPilhaVazia(t *testing.T) {
	r := New(nil)
	a := &fakeScreen{title: "a"}

	r.Replace(a)
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
	if r.Current() != a {
		t.Fatal("Replace com pilha vazia não definiu a tela base")
	}
}

func TestReset(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	a := &fakeScreen{title: "a"}
	b := &fakeScreen{title: "b"}

	r := New(root)
	r.Push(a, "a")
	r.Push(b, "b")
	r.SetSize(80, 24)
	guardCalls := 0
	r.SetGuard(func() (bool, tea.Cmd) {
		guardCalls++
		return false, nil
	})

	if cmd := r.Reset(); cmd != nil {
		t.Fatalf("Reset devolveu %v, esperado nil", cmd)
	}
	if got := r.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, esperado 1", got)
	}
	if root.initCalls != 0 {
		t.Fatalf("Reset chamou Init da base %d vez(es), esperado 0", root.initCalls)
	}
	if r.Current() != root {
		t.Fatal("Reset não deixou a tela base")
	}
	if got := r.Params(); got != nil {
		t.Fatalf("Params() após Reset = %v, esperado nil", got)
	}
	if guardCalls != 0 {
		t.Fatalf("Reset consultou o guard %d vez(es)", guardCalls)
	}
	if root.width != 80 || root.height != 24 {
		t.Fatalf("base ficou %dx%d após Reset, esperado 80x24", root.width, root.height)
	}
}

func TestNilRoot(t *testing.T) {
	r := New(nil)
	if got := r.Depth(); got != 0 {
		t.Fatalf("Depth() = %d, esperado 0", got)
	}
	if r.Current() != nil {
		t.Fatal("Current() não é nil com pilha vazia")
	}
	if cmd := r.Init(); cmd != nil {
		t.Fatalf("Init() = %v, esperado nil", cmd)
	}
	if got := r.View().Content; got != "" {
		t.Fatalf("View() = %q, esperado vazio", got)
	}
	if got := r.Breadcrumb(80); got != "" {
		t.Fatalf("Breadcrumb() = %q, esperado vazio", got)
	}
	if cmd := r.Pop(); cmd != nil {
		t.Fatalf("Pop() = %v, esperado nil", cmd)
	}
	if r.Depth() != 0 {
		t.Fatal("Pop alterou a pilha vazia")
	}

	a := &fakeScreen{title: "a"}
	r.Push(a, nil)
	if r.Depth() != 1 || r.Current() != a {
		t.Fatal("Push com pilha vazia não definiu a tela base")
	}
	r.Pop()
	if r.Depth() != 1 {
		t.Fatal("Pop removeu a única tela da pilha")
	}
}

func TestViewDelegaAoTopo(t *testing.T) {
	root := &fakeScreen{title: "raiz", view: "view-raiz"}
	a := &fakeScreen{title: "a", view: "view-a"}

	r := New(root)
	if got := r.View().Content; got != "view-raiz" {
		t.Fatalf("View() = %q, esperado view-raiz", got)
	}

	r.Push(a, nil)
	if got := r.View().Content; got != "view-a" {
		t.Fatalf("View() = %q, esperado view-a", got)
	}
}

func TestTitlesDevolveCopia(t *testing.T) {
	root := &fakeScreen{title: "raiz"}
	r := New(root)

	titles := r.Titles()
	titles[0] = "alterado"
	if got := r.Titles()[0]; got != "raiz" {
		t.Fatalf("Titles() compartilha o slice interno: %q", got)
	}
}

func TestInitDelegaAoTelaAtual(t *testing.T) {
	root := &fakeScreen{title: "raiz", initCmd: func() tea.Msg { return "init-raiz" }}
	r := New(root)

	cmd := r.Init()
	if cmd == nil || cmd() != "init-raiz" {
		t.Fatal("Init não delegou à tela base")
	}
}
