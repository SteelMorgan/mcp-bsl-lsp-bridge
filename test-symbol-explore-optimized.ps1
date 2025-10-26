# Тестирование symbol_explore с оптимизированной конфигурацией
# Автор: AI Assistant
# Дата: 26 октября 2025

Write-Host "🔧 Тестирование symbol_explore с оптимизированной конфигурацией" -ForegroundColor Cyan
Write-Host "=" * 60

# Проверяем наличие Docker
$dockerRunning = docker ps 2>$null
if (-not $dockerRunning) {
    Write-Host "❌ Docker не запущен. Запустите Docker Desktop и повторите попытку." -ForegroundColor Red
    exit 1
}

Write-Host "✅ Docker запущен" -ForegroundColor Green

# Останавливаем существующий контейнер если есть
Write-Host "🛑 Останавливаем существующий контейнер..." -ForegroundColor Yellow
docker stop mcp-lsp-bridge 2>$null
docker rm mcp-lsp-bridge 2>$null

# Собираем образ с новой конфигурацией
Write-Host "🔨 Собираем Docker образ с оптимизированной конфигурацией..." -ForegroundColor Yellow
docker build -t mcp-lsp-bridge:optimized -f Dockerfile.bsl . 2>$null

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Ошибка сборки Docker образа" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Docker образ собран успешно" -ForegroundColor Green

# Запускаем контейнер с новой конфигурацией
Write-Host "🚀 Запускаем контейнер с оптимизированной конфигурацией..." -ForegroundColor Yellow
docker run -d --name mcp-lsp-bridge-optimized `
    -p 3001:3001 `
    -v "D:\My Projects\Projects 1C\temp:/workspace" `
    -v "${PWD}/lsp_config.docker.optimized.json:/home/user/.local/share/mcp-lsp-bridge/config/lsp_config.json" `
    mcp-lsp-bridge:optimized

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Ошибка запуска контейнера" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Контейнер запущен успешно" -ForegroundColor Green

# Ждем запуска сервера
Write-Host "⏳ Ждем запуска MCP сервера (30 секунд)..." -ForegroundColor Yellow
Start-Sleep -Seconds 30

# Проверяем статус контейнера
$containerStatus = docker ps --filter "name=mcp-lsp-bridge-optimized" --format "table {{.Status}}"
Write-Host "📊 Статус контейнера: $containerStatus" -ForegroundColor Cyan

# Проверяем логи
Write-Host "📋 Последние логи контейнера:" -ForegroundColor Cyan
docker logs --tail 20 mcp-lsp-bridge-optimized

Write-Host "`n🎯 Тестирование завершено!" -ForegroundColor Green
Write-Host "Теперь можно тестировать symbol_explore с оптимизированными настройками" -ForegroundColor Cyan
Write-Host "Используйте контейнер: mcp-lsp-bridge-optimized" -ForegroundColor Cyan
