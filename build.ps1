# LogAnalyzer - Build Script

param(
    [switch]$Test,
    [switch]$Clean
)

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "     LogAnalyzer - Build Script               " -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host ""

if ($Clean) {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Yellow
    Remove-Item -Path "loganalyzer.exe" -Force -ErrorAction SilentlyContinue
    Write-Host "Clean complete!" -ForegroundColor Green
    Write-Host ""
}

if ($Test) {
    Write-Host "Running tests..." -ForegroundColor Yellow
    go test ./...
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Tests failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "All tests passed!" -ForegroundColor Green
    Write-Host ""
}

Write-Host "Building LogAnalyzer..." -ForegroundColor Yellow
go build -o loganalyzer.exe cmd/main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Binary created: loganalyzer.exe" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor Cyan
    Write-Host "  .\loganalyzer.exe -config examples\loganalyzer.yaml" -ForegroundColor White
    Write-Host ""
} else {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}
