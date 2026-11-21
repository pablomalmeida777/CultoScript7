package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend
var frontendFS embed.FS

//go:embed icons/tray-claro.png
var trayClaro []byte

//go:embed icons/tray-escuro.png
var trayEscuro []byte

const (
	nomeApp      = "CultoScript7"
	urlProjeto   = "https://github.com/pablomalmeida777/CultoScript7"
	margemPadrao = 32
)

// CultoApp concentra o estado do app e é exposto ao JavaScript como serviço.
type CultoApp struct {
	app    *application.App
	store  *Store
	widget *application.WebviewWindow

	mu        sync.Mutex
	editor    *application.WebviewWindow
	editando  bool
	altura    int
	moveTimer *time.Timer

	hwndWidget uintptr
}

func main() {
	migrarNomeAntigo()
	store, err := NovoStore()
	if err != nil {
		log.Fatal(err)
	}
	if f, err := os.OpenFile(filepath.Join(filepath.Dir(store.path), "cultoscript7.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
	}
	c := &CultoApp{store: store, altura: 480}

	assets, _ := fs.Sub(frontendFS, "frontend")
	c.app = application.New(application.Options{
		Name:        nomeApp,
		Description: "Programação do culto na área de trabalho",
		Services:    []application.Service{application.NewService(c)},
		Assets:      application.AssetOptions{Handler: application.BundledAssetFileServer(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "br.iasd.solarville.cultoscript",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				c.abrirEditor()
			},
		},
	})

	c.criarBandeja()

	// O widget só é criado depois que o app inicia: aí as telas já são
	// conhecidas e dá para abrir a janela direto na posição salva.
	c.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		x, y := c.posicaoInicial()
		c.criarWidget(x, y)
		go func() {
			c.prepararWidget(x, y)
			// Abre o editor quando:
			//  - pelo atalho do Menu Iniciar ("CultoScript7.exe --editar");
			//    se o app já estiver rodando, a 2ª instância cai em OnSecondInstanceLaunch;
			//  - na primeira execução (logo após instalar): o widget fica atrás de
			//    todas as janelas, então sem o editor pareceria que nada abriu.
			if temArg("--editar") || c.store.PrimeiraVez {
				c.abrirEditor()
			}
		}()
		c.garantirAutostart()
	})

	if err := c.app.Run(); err != nil {
		log.Fatal(err)
	}
}

func temArg(a string) bool {
	for _, x := range os.Args[1:] {
		if strings.EqualFold(x, a) {
			return true
		}
	}
	return false
}

// ───────────────────────── widget ─────────────────────────

func (c *CultoApp) criarWidget(x, y int) {
	cfg := c.store.Get()
	c.widget = c.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "widget",
		Title:                      nomeApp,
		Width:                      cfg.Largura,
		Height:                     c.altura,
		InitialPosition:            application.WindowXY,
		X:                          x,
		Y:                          y,
		Frameless:                  true,
		DisableResize:              true,
		Hidden:                     true,
		BackgroundType:             application.BackgroundTypeTransparent,
		BackgroundColour:           application.NewRGBA(0, 0, 0, 0),
		DefaultContextMenuDisabled: true,
		URL:                        "/widget.html",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
	})

	// Durante a edição o widget pode ser arrastado; salva a posição ao parar.
	c.widget.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.editando {
			return
		}
		if c.moveTimer != nil {
			c.moveTimer.Stop()
		}
		c.moveTimer = time.AfterFunc(400*time.Millisecond, c.salvarPosicao)
	})
}

// prepararWidget espera a janela nativa existir, prende-a à área de trabalho
// e só então a mostra (assim ela nunca aparece por cima das outras).
func (c *CultoApp) prepararWidget(x, y int) {
	var hwnd uintptr
	for i := 0; i < 200 && hwnd == 0; i++ {
		hwnd = uintptr(c.widget.NativeWindow())
		if hwnd == 0 {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if hwnd == 0 {
		log.Println("widget: janela nativa não foi criada")
		return
	}
	c.mu.Lock()
	c.hwndWidget = hwnd
	c.mu.Unlock()
	application.InvokeSync(func() { fixarNaAreaDeTrabalho(hwnd) })
	c.widget.SetPosition(x, y)
	c.widget.Show()
	px, py := c.widget.Position()
	log.Printf("widget hwnd=%#x pos=%d,%d", hwnd, px, py)
}

func (c *CultoApp) posicaoInicial() (int, int) {
	cfg := c.store.Get()
	x, y := cfg.X, cfg.Y
	if !cfg.PosicaoDefinida || !c.pontoVisivel(x, y) {
		if p := c.app.Screen.GetPrimary(); p != nil {
			wa := p.WorkArea
			x = wa.X + wa.Width - cfg.Largura - margemPadrao
			y = wa.Y + margemPadrao
		}
	}
	return x, y
}

func (c *CultoApp) posicionarWidget() {
	if c.widget != nil {
		c.widget.SetPosition(c.posicaoInicial())
	}
}

// pontoVisivel evita restaurar o widget num monitor que não existe mais.
func (c *CultoApp) pontoVisivel(x, y int) bool {
	for _, s := range c.app.Screen.GetAll() {
		b := s.Bounds
		if x >= b.X-40 && x < b.X+b.Width-40 && y >= b.Y-10 && y < b.Y+b.Height-40 {
			return true
		}
	}
	return false
}

func (c *CultoApp) salvarPosicao() {
	if c.widget == nil {
		return
	}
	x, y := c.widget.Position()
	_, err := c.store.Update(func(cfg *Config) {
		cfg.X, cfg.Y, cfg.PosicaoDefinida = x, y, true
	})
	if err != nil {
		log.Println("salvar posição:", err)
	}
}

func (c *CultoApp) aplicarTamanho() {
	if c.widget == nil {
		return
	}
	cfg := c.store.Get()
	c.mu.Lock()
	h := c.altura
	c.mu.Unlock()
	if s, err := c.widget.GetScreen(); err == nil && s != nil {
		if max := s.WorkArea.Height - 16; h > max {
			h = max
		}
	}
	c.widget.SetSize(cfg.Largura, h)
}

// ───────────────────────── editor ─────────────────────────

func (c *CultoApp) abrirEditor() {
	c.mu.Lock()
	if c.editor != nil {
		ed := c.editor
		c.mu.Unlock()
		ed.Show()
		ed.Restore()
		ed.Focus()
		return
	}
	ed := c.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:       "editor",
		Title:      nomeApp + " · Editar programação",
		StartState: application.WindowStateMaximised,
		Width:     1040,
		Height:    760,
		MinWidth:  820,
		MinHeight: 560,
		URL:       "/editor.html",
		BackgroundColour: corFundoEditor(c.store.Get().EditorTema),
		DefaultContextMenuDisabled: true,
	})
	c.editor = ed
	c.editando = true
	c.mu.Unlock()

	ed.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		c.mu.Lock()
		c.editor = nil
		c.editando = false
		c.mu.Unlock()
		c.salvarPosicao()
		c.app.Event.Emit("edicao", false)
	})
	c.app.Event.Emit("edicao", true)
	ed.Show()
	ed.Focus()
}

// corFundoEditor evita o "piscar" de fundo errado enquanto o editor carrega.
func corFundoEditor(tema string) application.RGBA {
	if tema == "claro" {
		return application.NewRGB(0xF4, 0xF5, 0xF8)
	}
	return application.NewRGB(0x1C, 0x1D, 0x22)
}

// ───────────────────────── bandeja ─────────────────────────

func (c *CultoApp) criarBandeja() {
	tray := c.app.SystemTray.New()
	tray.SetIcon(trayClaro)          // barra de tarefas clara → ícone escuro
	tray.SetDarkModeIcon(trayEscuro) // barra de tarefas escura → ícone branco
	tray.SetTooltip(nomeApp)

	tray.OnClick(c.menuBandeja)
	tray.OnRightClick(c.menuBandeja)
	tray.OnDoubleClick(func() { c.abrirEditor() })
}

// menuBandeja mostra o menu nativo próprio (ver mostrarMenu em desktop_windows.go).
// Os cliques da bandeja já chegam na thread da UI (wndProc), então o menu é
// aberto direto; a ação escolhida roda fora dela.
func (c *CultoApp) menuBandeja() {
	c.mu.Lock()
	dono := c.hwndWidget
	c.mu.Unlock()
	if dono == 0 {
		return
	}
	itens := []ItemMenu{
		{Rotulo: "Editar programação", Acao: c.abrirEditor},
		{},
		{Rotulo: "Fechar " + nomeApp, Acao: c.app.Quit},
	}
	if acao := mostrarMenu(dono, itens); acao != nil {
		go acao()
	}
}

// ───────────────────────── autostart ─────────────────────────

// Em builds de desenvolvimento (go run / wails3 dev) o executável fica numa
// pasta temporária; não faz sentido registrá-lo para iniciar com o Windows.
func executavelTemporario() bool {
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	exe = strings.ToLower(exe)
	return strings.Contains(exe, `\go-build`) || strings.Contains(exe, strings.ToLower(os.TempDir()))
}

// garantirAutostart registra o app para iniciar com o Windows a cada execução
// (assim o registro acompanha o executável se ele mudar de pasta).
func (c *CultoApp) garantirAutostart() {
	if executavelTemporario() {
		return
	}
	if err := c.app.Autostart.Enable(); err != nil {
		log.Println("autostart:", err)
	}
}

// ───────────────────── métodos expostos ao JS ─────────────────────

// Config devolve a configuração atual.
func (c *CultoApp) Config() Config { return c.store.Get() }

// ModoEdicao informa se o editor está aberto (widget arrastável).
func (c *CultoApp) ModoEdicao() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editando
}

// Salvar grava o conteúdo e a aparência vindos do editor.
func (c *CultoApp) Salvar(nova Config) (Config, error) {
	cfg, err := c.store.Update(func(cfg *Config) {
		cfg.Igreja = strings.TrimSpace(nova.Igreja)
		cfg.Cultos = nova.Cultos
		cfg.Largura = nova.Largura
		cfg.Escala = nova.Escala
		cfg.Opacidade = nova.Opacidade
		cfg.Modo = nova.Modo
		cfg.Claro = nova.Claro
		cfg.Escuro = nova.Escuro
		cfg.Colunas = nova.Colunas
		cfg.EditorTema = nova.EditorTema
	})
	if err != nil {
		return cfg, err
	}
	c.aplicarTamanho()
	c.app.Event.Emit("config", cfg)
	return cfg, nil
}

// AjustarAltura é chamado pelo widget sempre que o conteúdo muda de tamanho.
func (c *CultoApp) AjustarAltura(h int) {
	if h < 120 {
		h = 120
	}
	c.mu.Lock()
	c.altura = h
	c.mu.Unlock()
	c.aplicarTamanho()
}

// AbrirProjeto abre a página do projeto no GitHub no navegador padrão.
func (c *CultoApp) AbrirProjeto() error { return c.app.Browser.OpenURL(urlProjeto) }

// RestaurarPosicao volta o widget ao canto superior direito da tela principal.
func (c *CultoApp) RestaurarPosicao() {
	_, _ = c.store.Update(func(cfg *Config) { cfg.PosicaoDefinida = false })
	c.posicionarWidget()
}
