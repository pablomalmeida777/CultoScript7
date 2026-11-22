//go:build windows

package main

import (
	"testing"
	"time"
)

func TestInicioDoExplorer(t *testing.T) {
	inicio, ok := inicioDoExplorer()
	if !ok {
		t.Skip("sem explorer.exe nesta sessão (ex.: CI sem área de trabalho)")
	}
	if inicio.After(time.Now()) {
		t.Fatalf("início no futuro: %v", inicio)
	}
	t.Logf("explorer.exe (login) começou há %v", time.Since(inicio).Round(time.Second))
}

func TestNaoEmpacotadoNosTestes(t *testing.T) {
	if empacotado() {
		t.Fatal("os testes não rodam como pacote MSIX")
	}
}
