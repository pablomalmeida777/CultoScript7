package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// Secao é um bloco do culto (ex.: "Escola Sabatina") com suas etapas.
type Secao struct {
	Titulo string   `json:"titulo"`
	Itens  []string `json:"itens"`
}

// Paleta são as três cores personalizáveis do widget; o resto
// (texto secundário, linhas, bordas) é derivado delas no CSS.
type Paleta struct {
	Fundo    string `json:"fundo"`
	Texto    string `json:"texto"`
	Destaque string `json:"destaque"`
}

var (
	paletaClaroPadrao  = Paleta{Fundo: "#F7F8FC", Texto: "#18203A", Destaque: "#1D5FD6"}
	paletaEscuroPadrao = Paleta{Fundo: "#0B1630", Texto: "#EEF2FB", Destaque: "#DCBB72"}
	reHex              = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

func (p *Paleta) normalizar(padrao Paleta) {
	if !reHex.MatchString(p.Fundo) {
		p.Fundo = padrao.Fundo
	}
	if !reHex.MatchString(p.Texto) {
		p.Texto = padrao.Texto
	}
	if !reHex.MatchString(p.Destaque) {
		p.Destaque = padrao.Destaque
	}
}

// Culto vira uma aba no widget.
type Culto struct {
	ID     string  `json:"id"`
	Aba    string  `json:"aba"`    // rótulo curto da aba: "Sábado"
	Titulo string  `json:"titulo"` // título exibido: "Programação de sábado"
	Dia    int     `json:"dia"`    // 0=domingo … 6=sábado; -1 = sem dia fixo
	Secoes []Secao `json:"secoes"`
}

// Config é tudo o que o app persiste.
type Config struct {
	Igreja  string  `json:"igreja"`
	Cultos  []Culto `json:"cultos"`
	Largura int     `json:"largura"` // largura do widget em px
	Escala  float64 `json:"escala"`  // tamanho do texto (1 = padrão)
	Opacidade float64 `json:"opacidade"` // opacidade do fundo do cartão (0.4–1)
	Modo    string  `json:"modo"`    // "auto" (segue o Windows) | "claro" | "escuro"
	Claro   Paleta  `json:"claro"`   // cores usadas no modo claro
	Escuro  Paleta  `json:"escuro"`  // cores usadas no modo escuro
	Colunas int     `json:"colunas"` // 1 ou 2 (seções lado a lado)

	EditorTema string `json:"editorTema"` // tema da janela do editor: "escuro" (padrão) | "claro"

	// Posição do widget (em px lógicos). PosicaoDefinida=false usa o padrão.
	X               int  `json:"x"`
	Y               int  `json:"y"`
	PosicaoDefinida bool `json:"posicaoDefinida"`
}

func configPadrao() Config {
	return Config{
		Igreja:    "Minha Igreja",
		Largura:   400,
		Escala:    1,
		Opacidade: 0.88,
		Modo:      "auto",
		Claro:     paletaClaroPadrao,
		Escuro:    paletaEscuroPadrao,
		Colunas:   1,
		Cultos: []Culto{{
			ID:     "sabado",
			Aba:    "Sábado",
			Titulo: "Programação de Sábado",
			Dia:    6,
			Secoes: []Secao{
				{Titulo: "Escola Sabatina", Itens: []string{
					"Momento de louvor",
					"Louvor Inicial",
					"Oração",
					"Informativo Mundial das Missões",
					"Louvor Especial",
					"Apontamentos",
					"Recapitulação da lição",
					"Hino Final",
				}},
				{Titulo: "Culto Divino", Itens: []string{
					"Louvor",
					"Entrada da plataforma",
					"Anúncios",
					"Hino inicial",
					"Oração de joelhos",
					"Provai e vede",
					"Ofertório",
					"Adoração infantil",
					"Louvor especial",
					"Pregação",
					"Hino e oração final",
				}},
			},
		}},
	}
}

// normalizar mantém os valores dentro de faixas seguras.
func (c *Config) normalizar() {
	if c.Largura < 280 || c.Largura > 900 {
		c.Largura = 400
	}
	if c.Colunas != 2 {
		c.Colunas = 1
	}
	if c.EditorTema != "claro" {
		c.EditorTema = "escuro"
	}
	if c.Escala < 0.8 || c.Escala > 1.5 {
		c.Escala = 1
	}
	if c.Opacidade < 0.3 || c.Opacidade > 1 {
		c.Opacidade = 0.88
	}
	switch c.Modo {
	case "auto", "claro", "escuro":
	default:
		c.Modo = "auto"
	}
	c.Claro.normalizar(paletaClaroPadrao)
	c.Escuro.normalizar(paletaEscuroPadrao)
	for i := range c.Cultos {
		if c.Cultos[i].Dia < -1 || c.Cultos[i].Dia > 6 {
			c.Cultos[i].Dia = -1
		}
		if c.Cultos[i].Secoes == nil {
			c.Cultos[i].Secoes = []Secao{}
		}
		for j := range c.Cultos[i].Secoes {
			if c.Cultos[i].Secoes[j].Itens == nil {
				c.Cultos[i].Secoes[j].Itens = []string{}
			}
		}
	}
	if c.Cultos == nil {
		c.Cultos = []Culto{}
	}
}

// Store guarda a configuração em %APPDATA%\CultoScript7\config.json.
type Store struct {
	mu   sync.Mutex
	path string
	cfg  Config

	// PrimeiraVez indica que não havia configuração salva (app recém-instalado).
	PrimeiraVez bool
}

func NovoStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, nomeApp, "config.json")}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		s.cfg = configPadrao()
		s.PrimeiraVez = true
		return s, s.gravar()
	case err != nil:
		return nil, err
	}
	cfg := configPadrao()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		// Arquivo corrompido: guarda uma cópia e recomeça do padrão.
		_ = os.WriteFile(s.path+".corrompido", raw, 0o644)
		cfg = configPadrao()
	}
	cfg.normalizar()
	s.cfg = cfg
	return s, nil
}

func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Update aplica fn sobre a configuração e grava em disco.
func (s *Store) Update(fn func(*Config)) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	s.cfg.normalizar()
	return s.cfg, s.gravar()
}

func (s *Store) gravar() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
