# Caramel CLI Installer for Windows (PowerShell)
$ErrorActionPreference = "Stop"

$InstallDir = "$env:USERPROFILE\.caramel\bin"
$BinaryName = "caramel.exe"
$TargetPath = Join-Path $InstallDir $BinaryName
$SourcePath = "dist\caramel-windows-amd64.exe"

Write-Host "🍬 Instalando Caramel CLI para Windows..." -ForegroundColor Cyan

# Create install folder
If (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# Copy binary or build automatically
If (Test-Path $SourcePath) {
    Copy-Item -Path $SourcePath -Destination $TargetPath -Force
    Write-Host " └─ Binario copiado de $SourcePath para: $TargetPath" -ForegroundColor Green
} ElseIf (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host " └─ Executavel $SourcePath nao encontrado. Compilando via 'go build'..." -ForegroundColor Yellow
    go build -o $TargetPath ./cmd/caramel
    Write-Host " └─ Binario compilado e instalado em: $TargetPath" -ForegroundColor Green
} Else {
    Write-Host " └─ Executavel $SourcePath nao encontrado e Go nao esta instalado." -ForegroundColor Red
    Write-Host "   Execute 'scripts/build.sh' ou instale o Go." -ForegroundColor Yellow
    Exit 1
}

# Update PATH environment variable if not already present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
If ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host " └─ Adicionado $InstallDir ao PATH de Usuario." -ForegroundColor Green
}

# Update PATH in current session
If ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$env:Path;$InstallDir"
}

Write-Host "✅ Instalacao concluida com sucesso!" -ForegroundColor Green
Write-Host " Use 'caramel --help' para comecar." -ForegroundColor Cyan
