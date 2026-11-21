package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Arquivo de exportação dos cultos (backup / levar para outro PC / compartilhar).
type exportacao struct {
	App         string  `json:"app"`
	Formato     int     `json:"formato"`
	Versao      string  `json:"versao"`
	ExportadoEm string  `json:"exportadoEm"`
	Igreja      string  `json:"igreja"`
	Cultos      []Culto `json:"cultos"`
}

const formatoExportacao = 1

// ExportarCultos abre o "Salvar como" e grava os cultos num .json.
// Devolve o caminho salvo, ou "" se a pessoa cancelou.
func (c *CultoApp) ExportarCultos() (string, error) {
	cfg := c.store.Get()
	nome := fmt.Sprintf("%s-cultos-%s.json", nomeApp, time.Now().Format("2006-01-02"))

	dlg := c.app.Dialog.SaveFile().
		SetFilename(nome).
		AddFilter("Programação do CultoScript7 (*.json)", "*.json").
		CanCreateDirectories(true)
	if docs, err := os.UserHomeDir(); err == nil {
		dlg.SetDirectory(filepath.Join(docs, "Documents"))
	}
	if ed := c.janelaEditor(); ed != nil {
		dlg.AttachToWindow(ed)
	}
	caminho, err := dlg.PromptForSingleSelection()
	if err != nil || caminho == "" {
		return "", nil // cancelado
	}
	if !strings.EqualFold(filepath.Ext(caminho), ".json") {
		caminho += ".json"
	}

	dados, err := json.MarshalIndent(exportacao{
		App:         nomeApp,
		Formato:     formatoExportacao,
		Versao:      versao,
		ExportadoEm: time.Now().Format(time.RFC3339),
		Igreja:      cfg.Igreja,
		Cultos:      cfg.Cultos,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(caminho, dados, 0o644); err != nil {
		return "", fmt.Errorf("não foi possível salvar o arquivo: %w", err)
	}
	return caminho, nil
}

// ImportarCultos abre um .json exportado e SUBSTITUI os cultos atuais.
// Nome da igreja, cores e posição não mudam. Devolve a configuração nova,
// ou nil se a pessoa cancelou.
func (c *CultoApp) ImportarCultos() (*Config, error) {
	dlg := c.app.Dialog.OpenFile().
		SetTitle("Importar cultos").
		CanChooseFiles(true).
		AddFilter("Programação do CultoScript7 (*.json)", "*.json")
	if ed := c.janelaEditor(); ed != nil {
		dlg.AttachToWindow(ed)
	}
	caminho, err := dlg.PromptForSingleSelection()
	if err != nil || caminho == "" {
		return nil, nil // cancelado
	}

	raw, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o arquivo: %w", err)
	}
	cultos, err := lerCultos(raw)
	if err != nil {
		return nil, err
	}

	cfg, err := c.store.Update(func(cfg *Config) { cfg.Cultos = cultos })
	if err != nil {
		return nil, err
	}
	c.aplicarTamanho()
	c.app.Event.Emit("config", cfg)
	return &cfg, nil
}

// lerCultos aceita o arquivo exportado e também o config.json do app
// (os dois têm a lista "cultos"), e corrige o que vier inconsistente.
func lerCultos(raw []byte) ([]Culto, error) {
	var arq struct {
		Cultos *[]Culto `json:"cultos"`
	}
	if err := json.Unmarshal(raw, &arq); err != nil || arq.Cultos == nil {
		return nil, errors.New("este arquivo não é uma programação do CultoScript7")
	}
	cultos := *arq.Cultos
	if len(cultos) == 0 {
		return nil, errors.New("o arquivo não tem nenhum culto")
	}

	usados := map[string]bool{}
	base := time.Now().UnixNano()
	for i := range cultos {
		cu := &cultos[i]
		cu.Aba = strings.TrimSpace(cu.Aba)
		cu.Titulo = strings.TrimSpace(cu.Titulo)
		if cu.Aba == "" && cu.Titulo == "" {
			cu.Aba = "Culto " + strconv.Itoa(i+1)
		}
		if cu.ID == "" || usados[cu.ID] {
			cu.ID = strconv.FormatInt(base+int64(i), 36)
		}
		usados[cu.ID] = true
		if cu.Dia < -1 || cu.Dia > 6 {
			cu.Dia = -1
		}
		if cu.Secoes == nil {
			cu.Secoes = []Secao{}
		}
		for j := range cu.Secoes {
			itens := cu.Secoes[j].Itens[:0]
			for _, it := range cu.Secoes[j].Itens {
				if it = strings.TrimSpace(it); it != "" {
					itens = append(itens, it)
				}
			}
			cu.Secoes[j].Itens = itens
			if cu.Secoes[j].Itens == nil {
				cu.Secoes[j].Itens = []string{}
			}
		}
	}
	return cultos, nil
}

func (c *CultoApp) janelaEditor() *application.WebviewWindow {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor
}
