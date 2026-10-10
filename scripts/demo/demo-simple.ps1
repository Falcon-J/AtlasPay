# The full Compose demo owns setup, saga completion, compensation, and internal replay assertions.
param([string]$ApiUrl = 'http://localhost:8080')
$ErrorActionPreference = 'Stop'
& "$PSScriptRoot/../demo-smoke.ps1" -BaseUrl $ApiUrl
