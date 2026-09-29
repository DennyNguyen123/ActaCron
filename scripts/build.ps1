$ErrorActionPreference = "Stop"
Write-Host "Building ActaCron Production Release..." -ForegroundColor Cyan

# Clean dist
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue dist
New-Item -ItemType Directory -Path "dist/ActaCron-v1.0.0-windows-amd64" | Out-Null
New-Item -ItemType Directory -Path "dist/ActaCron-v1.0.0-windows-amd64/packages/demo-pack" | Out-Null

# Compile with GUI flag (no black console window) and stripped symbols
go build -ldflags "-H windowsgui -s -w" -o "dist/ActaCron-v1.0.0-windows-amd64/actacron.exe" .

# Copy documentation and sample files
Copy-Item ".env.example" "dist/ActaCron-v1.0.0-windows-amd64/.env.example"
Copy-Item "README.md" "dist/ActaCron-v1.0.0-windows-amd64/README.md" -ErrorAction SilentlyContinue
Copy-Item -Recurse "packages/demo-pack/*" "dist/ActaCron-v1.0.0-windows-amd64/packages/demo-pack/"

# Compress into portable zip
Compress-Archive -Path "dist/ActaCron-v1.0.0-windows-amd64" -DestinationPath "dist/ActaCron-v1.0.0-windows-amd64.zip"

Write-Host "Release created successfully: dist/ActaCron-v1.0.0-windows-amd64.zip" -ForegroundColor Green
