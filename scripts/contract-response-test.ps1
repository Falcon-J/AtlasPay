$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/contract-response.ps1"
Assert-ContractResponse -Output '  HTTP/1.1 200 OK' -ExitCode 0 -ExpectedStatus 200
Assert-ContractResponse -Output '  HTTP/1.1 401 Unauthorized' -ExitCode 1 -ExpectedStatus 401
Assert-ContractResponse -Output '  HTTP/1.1 401 Unauthorized' -ExitCode 8 -ExpectedStatus 401
foreach ($case in @(
    @{ Output = 'connection refused'; ExitCode = 1 },
    @{ Output = '  HTTP/1.1 404 Not Found'; ExitCode = 1 },
    @{ Output = '  HTTP/1.1 500 Internal Server Error'; ExitCode = 1 },
    @{ Output = '  HTTP/1.1 401 Unauthorized'; ExitCode = 2 },
    @{ Output = '  HTTP/1.1 401 Unauthorized'; ExitCode = 0 }
)) {
    $rejected = $false
    try { Assert-ContractResponse @case -ExpectedStatus 401 } catch { $rejected = $true }
    if (-not $rejected) { throw 'Contract assertion accepted an invalid response.' }
}
Write-Host 'Contract response assertions passed.'
