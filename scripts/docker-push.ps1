# Sub2API - build image and push to Aliyun Container Registry
# Called by docker-push.bat in repo root

$ErrorActionPreference = 'Stop'

$Registry   = 'registry.cn-chengdu.aliyuncs.com'
$ImageRepo  = "$Registry/mz-andy/sub2api"
$DockerUser = '517013774@qq.com'
$Commit     = 'prod'
$EnvFile    = Join-Path $PSScriptRoot '..\deploy\.version'
$RepoRoot   = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$LocalTag   = 'sub2api:latest'

# Inherit HTTP_PROXY / HTTPS_PROXY from parent bat (for faster docker build)
if ($env:HTTP_PROXY)  { Write-Host "[INFO] HTTP_PROXY=$($env:HTTP_PROXY)" }
if ($env:HTTPS_PROXY) { Write-Host "[INFO] HTTPS_PROXY=$($env:HTTPS_PROXY)" }

Set-Location $RepoRoot

Write-Host '========================================'
Write-Host ' Sub2API Docker Build and Push'
Write-Host '========================================'
Write-Host ''

# 1. Check Docker engine
$prevEap = $ErrorActionPreference
$ErrorActionPreference = 'SilentlyContinue'
docker info *> $null
$dockerOk = ($LASTEXITCODE -eq 0)
$ErrorActionPreference = $prevEap
if (-not $dockerOk) {
    Write-Host '[ERROR] Docker is not running. Please start Docker Desktop.' -ForegroundColor Red
    exit 1
}

# 2. Check registry login; interactive login if needed
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
    if ($LASTEXITCODE -ne 0) {
        Write-Host '[ERROR] docker login failed.' -ForegroundColor Red
        exit 1
    }
    Write-Host ''
}
else {
    Write-Host "[INFO] Already logged in to $Registry, skip docker login."
    Write-Host ''
}

# 3. Read BUILD_VERSION from deploy\.env and increment by 1
if (-not (Test-Path $EnvFile)) {
    Write-Host "[ERROR] Env file not found: $EnvFile" -ForegroundColor Red
    exit 1
}

$lines = Get-Content $EnvFile -Encoding UTF8
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

$lines | Set-Content $EnvFile -Encoding UTF8
$Version = $newVersion

Write-Host "[INFO] BUILD_VERSION=$Version"
$BuildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
Write-Host "[INFO] DATE=$BuildDate"
Write-Host ''

# 4. docker build
Write-Host '[INFO] Starting docker build...'
docker build -t $LocalTag `
    --build-arg "VERSION=$Version" `
    --build-arg "COMMIT=$Commit" `
    --build-arg "DATE=$BuildDate" `
    --build-arg "GOPROXY=https://goproxy.cn,direct" `
    --build-arg "GOSUMDB=sum.golang.google.cn" `
    --build-arg "NPM_CONFIG_REGISTRY=https://registry.npmmirror.com" `
    -f Dockerfile .

if ($LASTEXITCODE -ne 0) {
    Write-Host '[ERROR] docker build failed.' -ForegroundColor Red
    exit 1
}

Write-Host ''
Write-Host '[INFO] docker build succeeded.'
Write-Host ''

# 5. tag and push
$RemoteTag = "${ImageRepo}:$Version"
Write-Host "[INFO] Tagging: $RemoteTag"
docker tag $LocalTag $RemoteTag
if ($LASTEXITCODE -ne 0) {
    Write-Host '[ERROR] docker tag failed.' -ForegroundColor Red
    exit 1
}

Write-Host "[INFO] Pushing: $RemoteTag"
docker push $RemoteTag
if ($LASTEXITCODE -ne 0) {
    Write-Host '[ERROR] docker push failed. Re-run to login again if unauthorized.' -ForegroundColor Red
    exit 1
}

Write-Host ''
Write-Host '========================================'
Write-Host ' Done'
Write-Host " Image: $RemoteTag"
Write-Host " Version: $Version (saved to deploy\.env)"
Write-Host " Date: $BuildDate"
Write-Host '========================================'

exit 0
