$ErrorActionPreference = 'Stop'
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile("$PSScriptRoot/saga-crash-replay-smoke.ps1", [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw 'Crash replay script does not parse' }
$query = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Query-Claim' }, $true)
Invoke-Expression $query.Extent.Text
function docker { $global:LASTEXITCODE = 0 }
if ((Query-Claim -OrderId 'missing') -ne '') { throw 'Absent claim must allow polling to continue' }
function docker { $global:LASTEXITCODE = 0; 'event-1|running' }
if ((Query-Claim -OrderId 'present') -ne 'event-1|running') { throw 'Claim response changed' }
function docker { $global:LASTEXITCODE = 1 }
$rejected = $false
try { Query-Claim -OrderId 'failed' } catch { $rejected = $true }
if (-not $rejected) { throw 'Database failure must stop the smoke' }
$global:LASTEXITCODE = 0
Write-Host 'Saga claim polling self-check passed.'
