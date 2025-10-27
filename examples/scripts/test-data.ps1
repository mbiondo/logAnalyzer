# Test Data Generator for LogAnalyzer
param([int]$MessageCount = 5)

Write-Host "🚀 LogAnalyzer Test Data Generator" -ForegroundColor Green
Write-Host "Sending $MessageCount messages to Kafka and HTTP..." -ForegroundColor Yellow

# Sample messages
$messages = @(
    '{"level":"info","message":"User login successful","user_id":12345,"service":"auth"}',
    '{"level":"error","message":"Database connection failed","error":"timeout","service":"auth"}',
    '{"level":"warn","message":"High memory usage detected","usage":85,"service":"web"}'
)

$successCount = 0
$totalMessages = $MessageCount * 2

for ($i = 0; $i -lt $MessageCount; $i++) {
    $msg = $messages[$i % $messages.Length]

    Write-Host "`n📤 Message $($i + 1)/$MessageCount" -ForegroundColor Magenta

    # Send to Kafka
    $msg | docker exec -i loganalyzer-kafka kafka-console-producer --bootstrap-server localhost:9092 --topic application-logs 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Kafka: Sent successfully" -ForegroundColor Green
        $successCount++
    } else {
        Write-Host "❌ Kafka: Failed" -ForegroundColor Red
    }

    # Send to HTTP
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/logs" -Method POST -Body $msg -ContentType "application/json" -TimeoutSec 5
        if ($response.StatusCode -eq 200) {
            Write-Host "✅ HTTP: Sent successfully" -ForegroundColor Cyan
            $successCount++
        } else {
            Write-Host "❌ HTTP: Status $($response.StatusCode)" -ForegroundColor Red
        }
    } catch {
        Write-Host "❌ HTTP: Error" -ForegroundColor Red
    }

    Start-Sleep -Milliseconds 500
}

Write-Host "`n🎉 Complete! $successCount/$totalMessages successful" -ForegroundColor Green
Write-Host "📊 Check Grafana dashboards for the logs" -ForegroundColor Cyan
