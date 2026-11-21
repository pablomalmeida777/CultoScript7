//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// migrarNomeAntigo leva as configurações da versão "CultoScript" para
// "CultoScript7" e remove o antigo registro de inicialização com o Windows
// (que apontava para o executável com o nome antigo).
func migrarNomeAntigo() {
	if dir, err := os.UserConfigDir(); err == nil {
		antigo := filepath.Join(dir, "CultoScript")
		novo := filepath.Join(dir, nomeApp)
		if _, err := os.Stat(novo); os.IsNotExist(err) {
			if _, err := os.Stat(antigo); err == nil {
				_ = os.Rename(antigo, novo)
			}
		}
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE); err == nil {
		_ = k.DeleteValue("cultoscript")
		k.Close()
	}
}
