# Compila o CultoScript7 para Windows.
#   .\build.ps1                      -> bin\CultoScript7.exe
#   .\build.ps1 -Icones              -> regenera os ícones antes (a partir de assets-src\fonte-icone.ttf)
#   .\build.ps1 -Instalador          -> também gera bin\CultoScript7-Setup.exe (requer NSIS)
#   .\build.ps1 -Versao 1.2.0        -> define a versão (exe, instalador e build\windows\info.json)
# Sem -Versao, usa a versão que está em build\windows\info.json.
param([switch]$Icones, [switch]$Instalador, [string]$Versao = "")

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
$env:Path = (Join-Path (go env GOPATH) "bin") + ";" + $env:Path

function Exigir($nome) {
    if ($LASTEXITCODE -ne 0) { throw "$nome falhou (código $LASTEXITCODE)" }
}

# ── Versão ────────────────────────────────────────────────
$infoPath = "build\windows\info.json"
$info = Get-Content $infoPath -Raw -Encoding UTF8 | ConvertFrom-Json
if ($Versao) {
    $Versao = $Versao.TrimStart("v")
    if ($Versao -notmatch '^\d+\.\d+\.\d+$') { throw "Versão inválida: '$Versao' (use o formato 1.2.3)" }
    $info.fixed.file_version = $Versao
    $info.fixed.product_version = $Versao
    $info.info.'0409'.FileVersion = $Versao
    $info.info.'0409'.ProductVersion = $Versao
    [IO.File]::WriteAllText((Resolve-Path $infoPath), ($info | ConvertTo-Json -Depth 5), (New-Object Text.UTF8Encoding $false))
}
else {
    $Versao = $info.fixed.product_version
}
Write-Host "Versão $Versao"

# ── Ícones ────────────────────────────────────────────────
if ($Icones -or -not (Test-Path build\windows\icon.ico)) {
    go run ./tools/icongen -font assets-src/fonte-icone.ttf -text 7; Exigir "icongen"
    $mac = Join-Path $env:TEMP "CultoScript7-icon.icns"
    wails3 generate icons -input build/appicon.png -windowsfilename build/windows/icon.ico -macfilename $mac; Exigir "wails3 generate icons"
    # Os selos "7" do widget e do editor usam a mesma imagem do ícone do app.
    Copy-Item build/appicon.png frontend/img/sete.png -Force
}

# ── App ───────────────────────────────────────────────────
wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info $infoPath -out wails_windows_amd64.syso
Exigir "wails3 generate syso"
try {
    $env:CGO_ENABLED = "0"
    go build -tags production -trimpath -buildvcs=false -ldflags="-w -s -H windowsgui -X main.versao=$Versao" -o bin\CultoScript7.exe .
    Exigir "go build"
}
finally {
    Remove-Item wails_windows_amd64.syso -ErrorAction SilentlyContinue
}
Write-Host "OK -> bin\CultoScript7.exe"

# ── Instalador ────────────────────────────────────────────
if ($Instalador) {
    $makensis = "${env:ProgramFiles(x86)}\NSIS\makensis.exe"
    if (-not (Test-Path $makensis)) { throw "NSIS não encontrado. Instale com: winget install NSIS.NSIS" }
    if (-not (Test-Path build\windows\nsis\MicrosoftEdgeWebview2Setup.exe)) {
        wails3 generate webview2bootstrapper -dir build/windows/nsis; Exigir "wails3 generate webview2bootstrapper"
    }
    Push-Location build\windows\nsis
    try {
        & $makensis -V2 "-DINFO_PRODUCTVERSION=$Versao" "-DARG_WAILS_AMD64_BINARY=..\..\..\bin\CultoScript7.exe" project.nsi
        Exigir "makensis"
    }
    finally { Pop-Location }
    Write-Host "OK -> bin\CultoScript7-Setup.exe"
}
