# Compila o CultoScript7 para Windows.
#   .\build.ps1            -> bin\CultoScript7.exe
#   .\build.ps1 -Icones    -> regenera os ícones antes (a partir de assets-src\fonte-icone.ttf)
#   .\build.ps1 -Instalador -> também gera bin\CultoScript7-Setup.exe (requer NSIS)
param([switch]$Icones, [switch]$Instalador)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
$env:Path = "C:\Program Files\Go\bin;$env:USERPROFILE\go\bin;" + $env:Path

if ($Icones -or -not (Test-Path build\windows\icon.ico)) {
    go run ./tools/icongen -font assets-src/fonte-icone.ttf -text 7
    $mac = Join-Path $env:TEMP "CultoScript7-icon.icns"
    wails3 generate icons -input build/appicon.png -windowsfilename build/windows/icon.ico -macfilename $mac
    # Os selos "7" do widget e do editor usam a mesma imagem do ícone do app.
    Copy-Item build/appicon.png frontend/img/sete.png -Force
}

wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out wails_windows_amd64.syso
try {
    $env:CGO_ENABLED = "0"
    go build -tags production -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui" -o bin\CultoScript7.exe .
}
finally {
    Remove-Item wails_windows_amd64.syso -ErrorAction SilentlyContinue
}
Write-Host "OK -> bin\CultoScript7.exe"

if ($Instalador) {
    $makensis = "${env:ProgramFiles(x86)}\NSIS\makensis.exe"
    if (-not (Test-Path $makensis)) { throw "NSIS não encontrado. Instale com: winget install NSIS.NSIS" }
    if (-not (Test-Path build\windows\nsis\MicrosoftEdgeWebview2Setup.exe)) {
        wails3 generate webview2bootstrapper -dir build/windows/nsis
    }
    Push-Location build\windows\nsis
    try {
        & $makensis -V2 "-DARG_WAILS_AMD64_BINARY=..\..\..\bin\CultoScript7.exe" project.nsi
        if ($LASTEXITCODE -ne 0) { throw "makensis falhou ($LASTEXITCODE)" }
    }
    finally { Pop-Location }
    Write-Host "OK -> bin\CultoScript7-Setup.exe"
}
