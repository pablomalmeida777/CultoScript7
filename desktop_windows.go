//go:build windows

package main

import (
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Prende uma janela à área de trabalho:
//   - dono = Progman (janela do desktop), então ela não some com Win+D
//     e fica sempre logo acima do papel de parede;
//   - intercepta WM_WINDOWPOSCHANGING para que nunca suba na ordem Z,
//     mesmo quando clicada.

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	comctl32 = windows.NewLazySystemDLL("comctl32.dll")

	procFindWindowW       = user32.NewProc("FindWindowW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procSetWindowSubclass = comctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc   = comctl32.NewProc("DefSubclassProc")
)

const (
	gwlpHwndParent = ^uintptr(8 - 1) // -8
	gwlExStyle     = ^uintptr(20 - 1) // -20

	wsExToolWindow = 0x00000080
	wsExAppWindow  = 0x00040000

	wmWindowPosChanging = 0x0046
	wmMouseActivate     = 0x0021
	maNoActivate        = 3

	hwndBottom = 1

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

type windowPos struct {
	hwnd            uintptr
	hwndInsertAfter uintptr
	x, y, cx, cy    int32
	flags           uint32
}

var subclassProc = syscall.NewCallback(func(hwnd, msg, wParam, lParam, id, ref uintptr) uintptr {
	switch msg {
	case wmWindowPosChanging:
		wp := (*windowPos)(unsafe.Pointer(lParam))
		if wp.flags&swpNoZOrder == 0 {
			wp.hwndInsertAfter = hwndBottom
		}
	case wmMouseActivate:
		// Clicar no widget não rouba o foco da janela em uso.
		return maNoActivate
	}
	r, _, _ := procDefSubclassProc.Call(hwnd, msg, wParam, lParam)
	return r
})

// ── Menu da bandeja ─────────────────────────────────────────
// Menu nativo próprio (em vez do menu do Wails) para controlar:
//   - sem a coluna de ícones/marcação à esquerda (MNS_NOCHECK);
//   - itens com respiro nas laterais;
//   - um espaço entre o menu e a barra de tarefas, para parecer flutuante.

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procSetMenuInfo         = user32.NewProc("SetMenuInfo")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procSHAppBarMessage     = shell32.NewProc("SHAppBarMessage")
)

const (
	mfString    = 0x0000
	mfSeparator = 0x0800

	mimStyle   = 0x00000010
	mnsNoCheck = 0x80000000

	tpmCenterAlign  = 0x0004
	tpmRightAlign   = 0x0008
	tpmVCenterAlign = 0x0010
	tpmBottomAlign  = 0x0020
	tpmRightButton  = 0x0002
	tpmNoNotify     = 0x0080
	tpmReturnCmd    = 0x0100

	abmGetTaskbarPos = 0x00000005
	abeLeft          = 0
	abeTop           = 1
	abeRight         = 2

	wmNull = 0x0000

	// distância entre o menu e a barra de tarefas (px lógicos)
	folgaMenu = 10
	// respiro lateral dentro de cada item
	respiroItem = "   "
)

type menuInfo struct {
	cbSize          uint32
	fMask           uint32
	dwStyle         uint32
	cyMax           uint32
	hbrBack         uintptr
	dwContextHelpID uint32
	dwMenuData      uintptr
}

type ponto struct{ x, y int32 }

type retangulo struct{ left, top, right, bottom int32 }

type appBarData struct {
	cbSize           uint32
	hWnd             uintptr
	uCallbackMessage uint32
	uEdge            uint32
	rc               retangulo
	lParam           uintptr
}

// ItemMenu é uma opção do menu; Rotulo vazio vira separador.
type ItemMenu struct {
	Rotulo string
	Acao   func()
}

// mostrarMenu abre o menu perto do cursor, afastado da barra de tarefas, e
// devolve a ação escolhida (ou nil). Deve rodar na thread da UI.
func mostrarMenu(dono uintptr, itens []ItemMenu) func() {
	hmenu, _, _ := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return nil
	}
	defer procDestroyMenu.Call(hmenu)

	for i, it := range itens {
		if it.Rotulo == "" {
			procAppendMenuW.Call(hmenu, mfSeparator, 0, 0)
			continue
		}
		txt := windows.StringToUTF16Ptr(respiroItem + it.Rotulo + respiroItem)
		procAppendMenuW.Call(hmenu, mfString, uintptr(i+1), uintptr(unsafe.Pointer(txt)))
	}
	mi := menuInfo{fMask: mimStyle, dwStyle: mnsNoCheck}
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	procSetMenuInfo.Call(hmenu, uintptr(unsafe.Pointer(&mi)))

	// Posição: junto ao cursor, mas sempre com folga em relação à barra.
	var pt ponto
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	escala := 1.0
	if dpi, _, _ := procGetDpiForWindow.Call(dono); dpi > 0 {
		escala = float64(dpi) / 96
	}
	folga := int32(folgaMenu * escala)

	x, y := pt.x, pt.y
	flags := uintptr(tpmRightButton | tpmReturnCmd | tpmNoNotify)
	abd := appBarData{}
	abd.cbSize = uint32(unsafe.Sizeof(abd))
	if ok, _, _ := procSHAppBarMessage.Call(abmGetTaskbarPos, uintptr(unsafe.Pointer(&abd))); ok != 0 {
		switch abd.uEdge {
		case abeTop:
			y = abd.rc.bottom + folga
			flags |= tpmCenterAlign
		case abeLeft:
			x = abd.rc.right + folga
			flags |= tpmVCenterAlign
		case abeRight:
			x = abd.rc.left - folga
			flags |= tpmRightAlign | tpmVCenterAlign
		default: // embaixo (padrão do Windows 11)
			y = abd.rc.top - folga
			flags |= tpmCenterAlign | tpmBottomAlign
		}
	}

	// Necessário para o menu fechar ao clicar fora dele.
	procSetForegroundWindow.Call(dono)
	cmd, _, _ := procTrackPopupMenuEx.Call(hmenu, flags, uintptr(x), uintptr(y), dono, 0)
	procPostMessageW.Call(dono, wmNull, 0, 0)

	if cmd >= 1 && int(cmd) <= len(itens) {
		return itens[cmd-1].Acao
	}
	return nil
}

// ── Instalação pela Microsoft Store (MSIX) ──────────────────

var procGetCurrentPackageFullName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentPackageFullName")

// empacotado informa se o app roda como pacote MSIX (versão da Microsoft
// Store). Nesse caso a inicialização com o Windows é feita pela StartupTask
// declarada no AppxManifest, e não pela chave Run do registro.
func empacotado() bool {
	const appmodelErrorNoPackage = 15700
	var n uint32
	r, _, _ := procGetCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0)
	return r != appmodelErrorNoPackage
}

// iniciadoNoLogon diz se o app foi aberto logo após o login (inicialização
// automática). Na versão da Store a mesma entrada serve para o Menu Iniciar
// e para a StartupTask, sem argumentos; então usamos a hora em que o
// explorer.exe desta sessão começou (≈ hora do login) para diferenciar.
func iniciadoNoLogon() bool {
	inicio, ok := inicioDoExplorer()
	if !ok {
		return false
	}
	return time.Since(inicio) < 3*time.Minute
}

func inicioDoExplorer() (time.Time, bool) {
	var sessao uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessao); err != nil {
		return time.Time{}, false
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return time.Time{}, false
	}
	defer windows.CloseHandle(snap)

	var mais time.Time
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if !strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), "explorer.exe") {
			continue
		}
		var s uint32
		if windows.ProcessIdToSessionId(pe.ProcessID, &s) != nil || s != sessao {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pe.ProcessID)
		if err != nil {
			continue
		}
		var criado, saida, kernel, usuario windows.Filetime
		if windows.GetProcessTimes(h, &criado, &saida, &kernel, &usuario) == nil {
			t := time.Unix(0, criado.Nanoseconds())
			if mais.IsZero() || t.Before(mais) {
				mais = t // o explorer mais antigo da sessão = o do login
			}
		}
		windows.CloseHandle(h)
	}
	return mais, !mais.IsZero()
}

// fixarNaAreaDeTrabalho deve rodar na thread da UI (application.InvokeSync).
func fixarNaAreaDeTrabalho(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	// Garante que não aparece na barra de tarefas nem no Alt+Tab.
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	ex = (ex | wsExToolWindow) &^ wsExAppWindow
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex)

	progman, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Progman"))), 0)
	if progman != 0 {
		procSetWindowLongPtrW.Call(hwnd, gwlpHwndParent, progman)
	}
	procSetWindowSubclass.Call(hwnd, subclassProc, 0x7C5, 0)
	procSetWindowPos.Call(hwnd, hwndBottom, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
}
