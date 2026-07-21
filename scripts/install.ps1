# Caramel CLI Installer for Windows (PowerShell)
$ErrorActionPreference = "Stop"

$InstallDir = "$env:USERPROFILE\.caramel\bin"
$BinaryName = "caramel.exe"
$TargetPath = Join-Path $InstallDir $BinaryName

Write-Host "🍬 Instalando Caramel CLI para Windows..." -ForegroundColor Cyan

# Create install folder
If (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# Copy binary
If (Test-Path "dist\caramel-windows-amd64.exe") {
    Copy-Item -Path "dist\caramel-windows-amd64.exe" -Destination $TargetPath -Force
    Write-Host " └─ Binario copiado para: $TargetPath" -ForegroundColor Green
} Else {
    Write-Host " └─ Executavel dist\caramel-windows-amd64.exe nao encontrado. Execute 'scripts/build.sh' primeiro." -ForegroundColor Yellow
    Exit 1
}

# Update PATH environment variable if not already present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
If ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host " └─ Adicionado $InstallDir ao PATH de Usuario." -ForegroundColor Green
    Write-Host "   (Reinicie o terminal para atualizar o PATH)" -ForegroundColor Yellow
}

Write-Host "✅ Instalacao concluida com sucesso!" -ForegroundColor Green
