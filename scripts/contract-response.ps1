function Assert-ContractResponse {
    param([string]$Output, [int]$ExitCode, [int]$ExpectedStatus)

    $responses = [regex]::Matches($Output, '(?m)^\s*HTTP/\S+\s+(\d{3})\b')
    if ($responses.Count -eq 0 -or [int]$responses[$responses.Count - 1].Groups[1].Value -ne $ExpectedStatus) {
        throw "Expected HTTP $ExpectedStatus; the service returned an unexpected response or transport failure."
    }
    if (($ExpectedStatus -lt 400 -and $ExitCode -ne 0) -or
        ($ExpectedStatus -ge 400 -and $ExitCode -notin @(1, 8))) {
        throw "Unexpected request exit code $ExitCode for HTTP $ExpectedStatus."
    }
}
