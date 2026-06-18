# Sub2API - build image and push to Aliyun Container Registry
# Called by docker-push.bat in repo root

# Do NOT use Stop: docker CLI logs to stderr and would abort the script after build
$ErrorActionPreference = 'Continue'

$env:DOCKER_BUILDKIT = '1'
$env:BUILDKIT_PROGRESS = 'plain'
$env:BUILDX_NO_DEFAULT_ATTESTATIONS = '1'

$Registry    = 'registry.cn-chengdu.aliyuncs.com'
$ImageRepo   = "$Registry/mz-andy/sub2api"
$DockerUser  = '517013774@qq.com'
$Commit      = 'prod'
$VersionFile = Join-Path $PSScriptRoot '..\deploy\.version'
$RepoRoot    = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

function Write-StepLog {
    param([string]$Message)
    $ts = Get-Date -Format 'HH:mm:ss'
    Write-Host "[$ts] $Message"
}

function Assert-DockerOk {
    param(
        [int]$ExitCode,
        [string]$ErrorMessage
    )
    Write-StepLog "exit code = $ExitCode"
    if ($ExitCode -ne 0) {
        Write-Host $ErrorMessage -ForegroundColor Red
        exit 1
    }
}

if ($env:HTTP_PROXY)  { Write-Host "[INFO] HTTP_PROXY=$($env:HTTP_PROXY)" }
if ($env:HTTPS_PROXY) { Write-Host "[INFO] HTTPS_PROXY=$($env:HTTPS_PROXY)" }

Set-Location $RepoRoot

Write-Host '========================================'
Write-Host ' Sub2API Docker Build and Push'
Write-Host '========================================'
Write-Host ''

Write-StepLog 'Checking Docker engine...'
docker version *> $null
Assert-DockerOk -ExitCode $LASTEXITCODE -ErrorMessage '[ERROR] Docker is not running. Please start Docker Desktop.'

function Test-DockerRegistryLogin {
    param([string]$RegistryHost)
    $cfgPath = Join-Path $env:USERPROFILE '.docker\config.json'
    if (-not (Test-Path $cfgPath)) { return $false }
    $cfg = Get-Content $cfgPath -Raw | ConvertFrom-Json
    $inAuths = $cfg.auths -and ($cfg.auths.PSObject.Properties.Name -contains $RegistryHost)
    $inHelpers = $cfg.credHelpers -and ($cfg.credHelpers.PSObject.Properties.Name -contains $RegistryHost)
    return ($inAuths -or $inHelpers)
}

if (-not (Test-DockerRegistryLogin -RegistryHost $Registry)) {
    Write-Host "[INFO] Not logged in to $Registry. Please enter password:"
    Write-Host ''
    docker login --username=$DockerUser $Registry
    Assert-DockerOk -ExitCode $LASTEXITCODE -ErrorMessage '[ERROR] docker login failed.'
    Write-Host ''
}
else {
    Write-Host "[INFO] Already logged in to $Registry, skip docker login."
    Write-Host ''
}

if (-not (Test-Path $VersionFile)) {
    Write-Host "[ERROR] Version file not found: $VersionFile" -ForegroundColor Red
    exit 1
}

$lines = Get-Content $VersionFile -Encoding UTF8
$found = $false
$newVersion = 1

for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match '^BUILD_VERSION=(\d+)\s*(#.*)?$') {
        $found = $true
        $newVersion = [int]$Matches[1] + 1
        $suffix = ''
        if ($Matches[2]) { $suffix = ' ' + $Matches[2].Trim() }
        $lines[$i] = "BUILD_VERSION=$newVersion$suffix"
        break
    }
}

if (-not $found) {
    $lines += ''
    $lines += '# Docker Build Version (docker-push.bat auto increment)'
    $lines += 'BUILD_VERSION=1'
    $newVersion = 1
}

$lines | Set-Content $VersionFile -Encoding UTF8
$Version = $newVersion
$LocalTag = "sub2api:$Version"
$RemoteTag = "${ImageRepo}:$Version"

Write-Host "[INFO] BUILD_VERSION=$Version"
Write-Host "[INFO] Local tag:  $LocalTag (not loaded during push build)"
Write-Host "[INFO] Remote tag: $RemoteTag"
$BuildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
Write-Host "[INFO] DATE=$BuildDate"
Write-Host ''

Write-StepLog ">>> STEP: docker buildx build --push ($RemoteTag)"
docker buildx build `
    --push `
    --provenance=false `
    -t $RemoteTag `
    --progress=plain `
    --build-arg "VERSION=$Version" `
    --build-arg "COMMIT=$Commit" `
    --build-arg "DATE=$BuildDate" `
    --build-arg "GOPROXY=https://goproxy.cn,direct" `
    --build-arg "GOSUMDB=sum.golang.google.cn" `
    --build-arg "NPM_CONFIG_REGISTRY=https://registry.npmmirror.com" `
    -f Dockerfile .


$buildExit = $LASTEXITCODE
Write-StepLog '<<< STEP: docker buildx build --push done'
Assert-DockerOk -ExitCode $buildExit -ErrorMessage '[ERROR] docker build/push failed. Re-run to login again if unauthorized.'

Write-StepLog '>>> STEP: docker builder prune (clean build cache)'
docker builder prune -f
Write-StepLog '<<< STEP: build cache cleaned'

Write-StepLog '>>> STEP: remove local images'
foreach ($tag in @($RemoteTag, $LocalTag)) {
    $exists = docker image inspect $tag 2>$null
    if ($LASTEXITCODE -eq 0) {
        docker rmi $tag | Out-Null
        Write-Host "[INFO] Removed local image: $tag"
    }
}
Write-StepLog '<<< STEP: local images removed'

Write-Host ''
Write-Host '========================================'
Write-Host ' Done'
Write-Host " Image: $RemoteTag"
Write-Host " Version: $Version (saved to deploy\.version)"
Write-Host " Date: $BuildDate"
Write-Host '========================================'

exit 0
