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
# Exercise the actual polling loop with a delayed Kafka assignment, without wall-clock waits.
$loop = $ast.Find({ param($node) $node -is [Management.Automation.Language.ForStatementAst] -and $node.Extent.Text.Contains('Query-Claim') }, $true)
function Start-Sleep { param($Milliseconds) }
$script:polls = 0
function Query-Claim { param($OrderId); $script:polls++; if ($script:polls -ge 50) { 'event-delayed|running' } else { '' } }
$claim = ''; $orderId = 'delayed'
Invoke-Expression $loop.Extent.Text
if ($claim -ne 'event-delayed|running') { throw 'Polling stopped before delayed Kafka assignment acquired the claim' }
'Kafka assignment polling regression passed.'
$script:polls = 0
function Query-Claim { param($OrderId); $script:polls++; '' }
$claim = ''
Invoke-Expression $loop.Extent.Text
if ($claim -ne '' -or $script:polls -ne 180) { throw 'Missing claims must exhaust the bounded polling budget' }
'Absent claims remain bounded and cannot pass crash replay.'
