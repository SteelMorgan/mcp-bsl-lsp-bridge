# Тестирование BSL Language Server напрямую
# Автор: AI Assistant
# Дата: 26 октября 2025

Write-Host "🔧 Тестирование BSL Language Server напрямую" -ForegroundColor Cyan
Write-Host "=" * 50

# Пути
$bslJar = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\bsl-language-server.jar"
$testWorkspace = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\test-workspace"
$configFile = "D:\My Projects\FrameWork 1C\mcp-lsp-bridge\lsp_config.optimized.json"

# Проверяем наличие файлов
if (-not (Test-Path $bslJar)) {
    Write-Host "❌ BSL Language Server JAR не найден: $bslJar" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $testWorkspace)) {
    Write-Host "❌ Тестовая рабочая область не найдена: $testWorkspace" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Все файлы найдены" -ForegroundColor Green

# Переходим в рабочую область
Set-Location $testWorkspace

# Запускаем BSL Language Server в фоне
Write-Host "🚀 Запускаем BSL Language Server..." -ForegroundColor Yellow

$javaArgs = @(
    "-Xmx4g",
    "-Xms1g", 
    "-XX:+UseG1GC",
    "-XX:MaxGCPauseMillis=200",
    "-Dfile.encoding=UTF-8",
    "-Djava.awt.headless=true",
    "-jar",
    $bslJar,
    "--lsp"
)

$process = Start-Process -FilePath "java" -ArgumentList $javaArgs -RedirectStandardInput -RedirectStandardOutput -RedirectStandardError -PassThru -NoNewWindow

Write-Host "✅ BSL Language Server запущен (PID: $($process.Id))" -ForegroundColor Green

# Ждем запуска
Start-Sleep -Seconds 5

# Проверяем, что процесс еще работает
if ($process.HasExited) {
    Write-Host "❌ BSL Language Server завершился с кодом: $($process.ExitCode)" -ForegroundColor Red
    $error = $process.StandardError.ReadToEnd()
    Write-Host "Ошибка: $error" -ForegroundColor Red
    exit 1
}

Write-Host "✅ BSL Language Server работает" -ForegroundColor Green

# Отправляем LSP инициализацию
Write-Host "📡 Отправляем LSP инициализацию..." -ForegroundColor Yellow

$initRequest = @{
    jsonrpc = "2.0"
    id = 1
    method = "initialize"
    params = @{
        processId = $PID
        rootUri = "file:///$($testWorkspace.Replace('\', '/'))"
        capabilities = @{
            workspace = @{
                symbol = @{
                    dynamicRegistration = $true
                }
            }
        }
    }
} | ConvertTo-Json -Depth 10

$process.StandardInput.WriteLine($initRequest)
$process.StandardInput.Flush()

# Ждем ответ
Start-Sleep -Seconds 3

# Читаем ответ
$response = ""
$timeout = 10
$elapsed = 0

while ($elapsed -lt $timeout -and -not $process.HasExited) {
    if ($process.StandardOutput.Peek() -gt 0) {
        $response += $process.StandardOutput.ReadToEnd()
        break
    }
    Start-Sleep -Milliseconds 100
    $elapsed += 0.1
}

if ($response) {
    Write-Host "✅ Получен ответ от BSL Language Server:" -ForegroundColor Green
    Write-Host $response -ForegroundColor Cyan
} else {
    Write-Host "⚠️ Ответ не получен за $timeout секунд" -ForegroundColor Yellow
}

# Останавливаем процесс
Write-Host "🛑 Останавливаем BSL Language Server..." -ForegroundColor Yellow
$process.Kill()
$process.WaitForExit(5000)

Write-Host "🎯 Тестирование завершено!" -ForegroundColor Green
