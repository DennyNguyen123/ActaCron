param (
    [string]$Version = "v1.0.0"
)

$ErrorActionPreference = "Stop"
Write-Host "Building ActaCron Production Release ($Version)..." -ForegroundColor Cyan

$Commit = ""
try {
    $Commit = git rev-parse --short HEAD
} catch {
    $Commit = "none"
}
$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

# Clean dist
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue "dist/ActaCron-${Version}-windows-amd64"
New-Item -ItemType Directory -Path "dist/ActaCron-${Version}-windows-amd64" -Force | Out-Null
New-Item -ItemType Directory -Path "dist/ActaCron-${Version}-windows-amd64/packages/demo-pack" -Force | Out-Null

$ldflags = "-H windowsgui -s -w -X 'actacron/internal/version.Version=$Version' -X 'actacron/internal/version.GitCommit=$Commit' -X 'actacron/internal/version.BuildDate=$BuildDate'"

# Compile with GUI flag (no black console window) and stripped symbols
Write-Host "Compiling Go binary..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
go build -ldflags "$ldflags" -o "dist/ActaCron-${Version}-windows-amd64/actacron.exe" .

# Copy documentation and sample files
Copy-Item ".env.example" "dist/ActaCron-${Version}-windows-amd64/.env.example"
Copy-Item "README.md" "dist/ActaCron-${Version}-windows-amd64/README.md" -ErrorAction SilentlyContinue
Copy-Item -Recurse "packages/demo-pack/*" "dist/ActaCron-${Version}-windows-amd64/packages/demo-pack/"

# Compress into portable zip
Write-Host "Compressing portable release ZIP..." -ForegroundColor Yellow
$zipPath = "dist/ActaCron-${Version}-windows-amd64.zip"
Remove-Item -Force -ErrorAction SilentlyContinue $zipPath
Compress-Archive -Path "dist/ActaCron-${Version}-windows-amd64" -DestinationPath $zipPath

Write-Host "Portable ZIP created: $zipPath" -ForegroundColor Green

# Check if Inno Setup Compiler (iscc) is available locally
$isccPath = Get-Command "iscc" -ErrorAction SilentlyContinue
if (-not $isccPath) {
    # Check default Inno Setup installation directories
    $commonPaths = @(
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles}\Inno Setup 6\ISCC.exe",
        "${env:LocalAppData}\Programs\Inno Setup 6\ISCC.exe"
    )
    foreach ($p in $commonPaths) {
        if (Test-Path $p) {
            $isccPath = $p
            break
        }
    }
}

if ($isccPath) {
    Write-Host "Compiling Inno Setup Installer with $isccPath..." -ForegroundColor Yellow
    & $isccPath "scripts/installer.iss" "/DMyAppVersion=$Version"
    Write-Host "Installer created successfully in dist/" -ForegroundColor Green
} else {
    Write-Host "Note: Inno Setup compiler (iscc) not found in PATH or standard locations. Skipping local installer generation (installer will be generated on GitHub Actions runner)." -ForegroundColor DarkGray
}
