$ErrorActionPreference = "Stop"

$envPath = Join-Path $PSScriptRoot ".env"
if (!(Test-Path $envPath)) {
    throw ".env file not found: $envPath"
}

Get-Content $envPath | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#") -or !$line.Contains("=")) {
        return
    }
    $parts = $line.Split("=", 2)
    [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), "Process")
}

if ([string]::IsNullOrWhiteSpace($env:TELEGRAM_API_ID)) {
    throw "TELEGRAM_API_ID is empty. Get it from https://my.telegram.org and put it in .env"
}
if ([string]::IsNullOrWhiteSpace($env:TELEGRAM_API_HASH)) {
    throw "TELEGRAM_API_HASH is empty. Get it from https://my.telegram.org and put it in .env"
}

$bin = Get-Command telegram-bot-api -ErrorAction SilentlyContinue
if ($null -eq $bin) {
    throw "telegram-bot-api was not found in PATH. Install it or run the executable with full path."
}

& $bin.Source --api-id=$env:TELEGRAM_API_ID --api-hash=$env:TELEGRAM_API_HASH --local --http-port=8081
