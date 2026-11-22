# Compila o CultoScript7 para Windows.
#   .\build.ps1                      -> bin\CultoScript7.exe
#   .\build.ps1 -Icones              -> regenera os ícones antes (a partir de assets-src\fonte-icone.ttf)
#   .\build.ps1 -Instalador          -> também gera bin\CultoScript7-Setup.exe (requer NSIS)
#   .\build.ps1 -Loja                -> também gera bin\CultoScript7.msix (Microsoft Store)
#   .\build.ps1 -Versao 1.2.0        -> define a versão (exe, instalador e build\windows\info.json)
# Sem -Versao, usa a versão que está em build\windows\info.json.
param([switch]$Icones, [switch]$Instalador, [switch]$Loja, [string]$Versao = "")

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

# ── Pacote da Microsoft Store (MSIX) ──────────────────────
if ($Loja) {
    # MakeAppx: do Windows SDK, se instalado; senão, do pacote NuGet (só as ferramentas).
    $makeappx = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\makeappx.exe" -ErrorAction SilentlyContinue |
        Sort-Object FullName | Select-Object -Last 1 -ExpandProperty FullName
    if (-not $makeappx) {
        $sdkVer = "10.0.28000.2705"
        $sdkDir = Join-Path $env:LOCALAPPDATA "CultoScript7-build\sdk-buildtools\$sdkVer"
        if (-not (Test-Path $sdkDir)) {
            New-Item -ItemType Directory -Force $sdkDir | Out-Null
            $zip = "$sdkDir.zip"
            Invoke-WebRequest "https://api.nuget.org/v3-flatcontainer/microsoft.windows.sdk.buildtools/$sdkVer/microsoft.windows.sdk.buildtools.$sdkVer.nupkg" -OutFile $zip -UseBasicParsing
            Expand-Archive $zip $sdkDir -Force
            Remove-Item $zip
        }
        $makeappx = Get-ChildItem $sdkDir -Recurse -Filter makeappx.exe | Where-Object FullName -match '\\x64\\' |
            Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not $makeappx) { throw "makeappx.exe não encontrado" }

    $pacote = "bin\msix-pacote"
    Remove-Item $pacote -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force "$pacote\Assets" | Out-Null
    Copy-Item bin\CultoScript7.exe $pacote

    # Manifesto com a versão.
    $manifesto = [IO.File]::ReadAllText((Resolve-Path build\windows\msix\AppxManifest.xml), [Text.Encoding]::UTF8)
    [IO.File]::WriteAllText((Join-Path (Resolve-Path $pacote) "AppxManifest.xml"), $manifesto.Replace("__VERSAO__", $Versao), (New-Object Text.UTF8Encoding $false))

    # Ícones do pacote, a partir do ícone do app.
    Add-Type -AssemblyName System.Drawing
    $origem = [Drawing.Image]::FromFile((Resolve-Path build\appicon.png))
    function Salvar-Icone($nome, $w, $h, $lado) {
        $bmp = New-Object Drawing.Bitmap $w, $h
        $g = [Drawing.Graphics]::FromImage($bmp)
        $g.InterpolationMode = "HighQualityBicubic"; $g.SmoothingMode = "HighQuality"; $g.PixelOffsetMode = "HighQuality"
        $g.DrawImage($origem, [int](($w - $lado) / 2), [int](($h - $lado) / 2), $lado, $lado)
        $bmp.Save((Join-Path (Resolve-Path "$pacote\Assets") $nome), [Drawing.Imaging.ImageFormat]::Png)
        $g.Dispose(); $bmp.Dispose()
    }
    try {
        Salvar-Icone "StoreLogo.png" 50 50 50
        Salvar-Icone "Square44x44Logo.png" 44 44 44
        Salvar-Icone "Square150x150Logo.png" 150 150 150
        Salvar-Icone "Wide310x150Logo.png" 310 150 150
        foreach ($t in 16, 24, 32, 48, 256) {
            Salvar-Icone "Square44x44Logo.targetsize-$t.png" $t $t $t
            Salvar-Icone "Square44x44Logo.targetsize-${t}_altform-unplated.png" $t $t $t
        }
    }
    finally { $origem.Dispose() }

    # resources.pri: índice que faz o Windows usar os ícones "targetsize" e
    # "altform-unplated" (sem placa de fundo) na barra de tarefas e no Menu Iniciar.
    $makepri = Join-Path (Split-Path $makeappx) "makepri.exe"
    if (-not (Test-Path $makepri)) { throw "makepri.exe não encontrado ao lado do makeappx" }
    $priconfig = "bin\priconfig.xml"
    $saida = & $makepri createconfig /cf $priconfig /dq pt-BR /pv 10.0.0 /o
    if ($LASTEXITCODE -ne 0) { $saida | Write-Host; throw "makepri createconfig falhou (código $LASTEXITCODE)" }
    $saida = & $makepri new /pr $pacote /cf $priconfig /mn "$pacote\AppxManifest.xml" /of "$pacote\resources.pri" /o
    if ($LASTEXITCODE -ne 0) { $saida | Write-Host; throw "makepri new falhou (código $LASTEXITCODE)" }
    Remove-Item $priconfig

    $saida = & $makeappx pack /o /d $pacote /p bin\CultoScript7.msix
    if ($LASTEXITCODE -ne 0) { $saida | Write-Host; throw "makeappx falhou (código $LASTEXITCODE)" }
    Remove-Item $pacote -Recurse -Force
    Write-Host "OK -> bin\CultoScript7.msix"
}
