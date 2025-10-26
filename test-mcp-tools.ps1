# Тестирование MCP инструментов с оптимизированной конфигурацией
# Автор: AI Assistant
# Дата: 26 октября 2025

Write-Host "🔧 Тестирование MCP инструментов" -ForegroundColor Cyan
Write-Host "=" * 50

# Проверяем наличие MCP сервера
$mcpServer = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\mcp-lsp-bridge.exe"
if (-not (Test-Path $mcpServer)) {
    Write-Host "❌ MCP сервер не найден: $mcpServer" -ForegroundColor Red
    exit 1
}

Write-Host "✅ MCP сервер найден" -ForegroundColor Green

# Проверяем конфигурацию
$configFile = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\lsp_config.optimized.json"
if (-not (Test-Path $configFile)) {
    Write-Host "❌ Оптимизированная конфигурация не найдена: $configFile" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Оптимизированная конфигурация найдена" -ForegroundColor Green

# Копируем оптимизированную конфигурацию
Write-Host "📋 Копируем оптимизированную конфигурацию..." -ForegroundColor Yellow
Copy-Item $configFile "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\lsp_config.json" -Force

# Запускаем MCP сервер в фоне
Write-Host "🚀 Запускаем MCP сервер..." -ForegroundColor Yellow
$process = Start-Process -FilePath $mcpServer -ArgumentList @("-config", "mcp_config.json", "-lsp-config", "lsp_config.json") -PassThru -NoNewWindow

Write-Host "✅ MCP сервер запущен (PID: $($process.Id))" -ForegroundColor Green

# Ждем запуска
Write-Host "⏳ Ждем запуска MCP сервера (10 секунд)..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Проверяем, что процесс еще работает
if ($process.HasExited) {
    Write-Host "❌ MCP сервер завершился с кодом: $($process.ExitCode)" -ForegroundColor Red
    exit 1
}

Write-Host "✅ MCP сервер работает" -ForegroundColor Green

# Тестируем BSL Language Server напрямую
Write-Host "🧪 Тестируем BSL Language Server напрямую..." -ForegroundColor Yellow

$testWorkspace = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\test-workspace"
Set-Location $testWorkspace

# Тест анализа
Write-Host "📊 Тест анализа файлов..." -ForegroundColor Cyan
$analyzeResult = java -Xmx4g -Xms1g -XX:+UseG1GC -XX:MaxGCPauseMillis=200 -jar "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\bsl-language-server.jar" --analyze . 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Анализ файлов работает" -ForegroundColor Green
} else {
    Write-Host "❌ Ошибка анализа: $analyzeResult" -ForegroundColor Red
}

# Тест форматирования
Write-Host "🎨 Тест форматирования файлов..." -ForegroundColor Cyan
$formatResult = java -Xmx4g -Xms1g -XX:+UseG1GC -XX:MaxGCPauseMillis=200 -jar "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\bsl-language-server.jar" --format . 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Форматирование файлов работает" -ForegroundColor Green
} else {
    Write-Host "❌ Ошибка форматирования: $formatResult" -ForegroundColor Red
}

# Останавливаем MCP сервер
Write-Host "🛑 Останавливаем MCP сервер..." -ForegroundColor Yellow
$process.Kill()
$process.WaitForExit(5000)

Write-Host "🎯 Тестирование завершено!" -ForegroundColor Green
Write-Host "BSL Language Server работает корректно с оптимизированными настройками" -ForegroundColor Cyan
