package main

import (
	"encoding/json"
	"testing"
)

func TestLerCultosExportacao(t *testing.T) {
	raw, _ := json.Marshal(exportacao{App: nomeApp, Formato: 1, Cultos: configPadrao().Cultos})
	cultos, err := lerCultos(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cultos) != 1 || cultos[0].Aba != "Sábado" || len(cultos[0].Secoes) != 2 {
		t.Fatalf("conteúdo inesperado: %+v", cultos)
	}
}

func TestLerCultosAceitaConfigJSON(t *testing.T) {
	raw, _ := json.Marshal(configPadrao())
	if _, err := lerCultos(raw); err != nil {
		t.Fatalf("config.json deveria ser aceito: %v", err)
	}
}

func TestLerCultosRecusaArquivoInvalido(t *testing.T) {
	for nome, raw := range map[string]string{
		"não é json":  "isto não é json",
		"sem cultos":  `{"app":"outro"}`,
		"lista vazia": `{"cultos":[]}`,
	} {
		if _, err := lerCultos([]byte(raw)); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
}

func TestLerCultosCorrigeInconsistencias(t *testing.T) {
	raw := `{"cultos":[
		{"id":"a","aba":"  Domingo ","dia":9,"secoes":[{"titulo":"X","itens":["  um ","", "  "]}]},
		{"id":"a","aba":"","titulo":"","dia":3},
		{"aba":"Sem id","dia":-1}
	]}`
	cultos, err := lerCultos([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cultos[0].Aba != "Domingo" || cultos[0].Dia != -1 {
		t.Errorf("aba/dia não normalizados: %+v", cultos[0])
	}
	if it := cultos[0].Secoes[0].Itens; len(it) != 1 || it[0] != "um" {
		t.Errorf("itens vazios não removidos: %q", it)
	}
	if cultos[1].ID == "a" || cultos[2].ID == "" {
		t.Errorf("ids duplicados/vazios não corrigidos: %q %q", cultos[1].ID, cultos[2].ID)
	}
	if cultos[1].Aba != "Culto 2" || cultos[1].Secoes == nil {
		t.Errorf("culto sem nome não recebeu padrão: %+v", cultos[1])
	}
}
