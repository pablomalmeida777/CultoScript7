Unicode true

####
## Instalador do CultoScript7.
## Gerado por:  .\build.ps1 -Instalador   (saída: bin\CultoScript7-Setup.exe)
## Manualmente: makensis -DARG_WAILS_AMD64_BINARY=..\..\..\bin\CultoScript7.exe project.nsi
####

!define INFO_PROJECTNAME    "CultoScript7"
!define INFO_COMPANYNAME    "Pablo Almeida"
!define INFO_PRODUCTNAME    "CultoScript7"
!ifndef INFO_PRODUCTVERSION
    !define INFO_PRODUCTVERSION "1.2.0"  # o build.ps1 passa a versão real (-DINFO_PRODUCTVERSION=...)
!endif
!define INFO_COPYRIGHT      "© 2026 Pablo Almeida · Licença MIT"
!define PRODUCT_EXECUTABLE  "CultoScript7.exe"
!define UNINST_KEY_NAME     "CultoScript7"
!define WAILS_INSTALL_SCOPE "machine"

!define WAILS_WIN10_REQUIRED "O CultoScript7 precisa do Windows 10 ou mais recente."
!define WAILS_ARCHITECTURE_NOT_SUPPORTED "Este computador não é compatível com o CultoScript7 (requer Windows 64 bits)."
!define WAILS_INSTALL_WEBVIEW_DETAILPRINT "Instalando o Microsoft Edge WebView2..."

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "Instalador do ${INFO_PRODUCTNAME}"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI2.nsh"

!define MUI_ICON   "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING

!define MUI_WELCOMEPAGE_TITLE "Instalar o CultoScript7"
!define MUI_WELCOMEPAGE_TEXT "A programação do culto sempre à vista, direto na área de trabalho.$\r$\n$\r$\nO CultoScript7 será instalado e passará a iniciar junto com o Windows. Para editar a programação, use o atalho CultoScript7 no Menu Iniciar ou o ícone 7 perto do relógio.$\r$\n$\r$\nClique em Avançar para continuar."

!define MUI_FINISHPAGE_TITLE "Tudo pronto!"
!define MUI_FINISHPAGE_TEXT "O CultoScript7 foi instalado.$\r$\n$\r$\nDesenvolvido por Pablo Almeida, para honra e glória de Deus!"
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Abrir o CultoScript7 agora"
!define MUI_FINISHPAGE_RUN_FUNCTION AbrirApp
# Segunda caixa da tela final (usa o slot "readme" do MUI): atalho na área de trabalho.
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Criar atalho na área de trabalho"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CriarAtalhoAreaDeTrabalho
!define MUI_FINISHPAGE_LINK "Página do projeto no GitHub"
!define MUI_FINISHPAGE_LINK_LOCATION "https://github.com/pablomalmeida777/CultoScript7"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "PortugueseBR"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-Setup.exe"
InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
ShowInstDetails nevershow
ShowUninstDetails nevershow

Function .onInit
    !insertmacro wails.checkArchitecture
    SetRegView 64
FunctionEnd

# Abre o app SEM privilégios de administrador (via Explorer), para que ele
# rode como o usuário normal e se registre para iniciar com o Windows.
Function AbrirApp
    Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
FunctionEnd

Function CriarAtalhoAreaDeTrabalho
    SetShellVarContext all
    CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "--editar" \
        "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 SW_SHOWNORMAL "" "Editar a programação do culto"
FunctionEnd

!macro FecharApp
    # Fecha uma versão em execução para permitir atualizar/remover os arquivos.
    nsExec::Exec 'taskkill /F /IM "${PRODUCT_EXECUTABLE}"'
    Pop $0
    Sleep 800
!macroend

Section
    !insertmacro wails.setShellContext
    !insertmacro FecharApp

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files
    File "/oname=LICENSE.txt" "..\..\..\LICENSE"

    # Menu Iniciar: abre o editor da programação.
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "--editar" \
        "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 SW_SHOWNORMAL "" "Editar a programação do culto"

    !insertmacro wails.writeUninstaller
    WriteRegStr HKLM "${UNINST_KEY}" "URLInfoAbout" "https://github.com/pablomalmeida777/CultoScript7"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext
    !insertmacro FecharApp

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    RMDir /r $INSTDIR

    # Dados do usuário (pasta do perfil de quem está desinstalando).
    SetShellVarContext current
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"   # caso tenha sido criado só para o usuário
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "cultoscript7"
    RMDir /r "$APPDATA\${PRODUCT_EXECUTABLE}"   # cache do WebView2

    MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 \
        "Apagar também a programação e as configurações salvas?$\r$\n$\r$\nEscolha Não se pretende reinstalar o CultoScript7." \
        /SD IDNO IDNO manter
        RMDir /r "$APPDATA\${INFO_PRODUCTNAME}"
    manter:

    SetShellVarContext all
    !insertmacro wails.deleteUninstaller
SectionEnd
