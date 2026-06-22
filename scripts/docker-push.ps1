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

function Test-DockerImageExists {
    param([string]$Tag)
    docker image inspect $Tag *> $null
    return ($LASTEXITCODE -eq 0)
}

function Stop-WedgedBuildProcesses {
    # The post-build hang lives in docker-buildx.exe (the BuildKit CLI plugin),
    # NOT in the docker.exe we launched - docker.exe forks buildx and may exit
    # first, leaving buildx wedged. So kill buildx by NAME to actually clear it.
    param([System.Diagnostics.Process]$TrackedProc)
    if ($TrackedProc -and -not $TrackedProc.HasExited) {
        try { $TrackedProc.Kill() } catch { }
    }
    Get-Process -Name 'docker-buildx' -ErrorAction SilentlyContinue | ForEach-Object {
        try { $_.Kill() } catch { }
    }
}

function Invoke-ResilientBuild {
    # Runs `docker build` but stays immune to the Docker Desktop + containerd
    # post-build hang: docker finishes the image ("#44 DONE", image is already
    # in the local store) but docker-buildx.exe sometimes wedges on its
    # build-record / "View build details" step and never returns. We start
    # docker with live console output, then poll: once the target image is
    # present in the store the build is genuinely done, so if the process has
    # not exited within a short grace window we kill the wedged buildx and go on.
    param(
        [string[]]$DockerArgs,
        [string]$ReadyTag,            # image tag that signals the build finished
        [int]$GraceSecondsAfterReady = 20,
        [int]$HardTimeoutSeconds = 1800
    )
    $proc = Start-Process -FilePath 'docker' -ArgumentList $DockerArgs `
        -NoNewWindow -PassThru
    $start = Get-Date
    $readyAt = $null
    while ($true) {
        $exited = $false
        try { $exited = $proc.HasExited } catch { $exited = $true }
        if ($exited -and -not $readyAt) {
            # Process returned on its own (normal, no hang). Use its exit code.
            $code = 0
            try { $code = $proc.ExitCode } catch { $code = 0 }
            return $code
        }
        if (-not $readyAt) {
            if (Test-DockerImageExists -Tag $ReadyTag) {
                $readyAt = Get-Date
                Write-StepLog "image $ReadyTag present in local store; grace ${GraceSecondsAfterReady}s for docker to exit"
            }
        }
        else {
            if ($exited) { return 0 }   # image ready AND process exited cleanly
            if (((Get-Date) - $readyAt).TotalSeconds -ge $GraceSecondsAfterReady) {
                Write-Host "[WARN] image built but docker/buildx did not return (known Docker Desktop hang); killing wedged buildx and continuing." -ForegroundColor Yellow
                Stop-WedgedBuildProcesses -TrackedProc $proc
                return 0
            }
        }
        if (((Get-Date) - $start).TotalSeconds -ge $HardTimeoutSeconds) {
            Write-Host "[ERROR] docker build exceeded $HardTimeoutSeconds s hard timeout." -ForegroundColor Red
            Stop-WedgedBuildProcesses -TrackedProc $proc
            return 1
        }
        Start-Sleep -Seconds 2
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
Write-Host "[INFO] Local tag:  $LocalTag (built locally, removed after push)"
Write-Host "[INFO] Remote tag: $RemoteTag"
$BuildDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
Write-Host "[INFO] DATE=$BuildDate"
Write-Host ''

Write-StepLog ">>> STEP: docker build ($RemoteTag)"
# Use classic `docker build` + `docker push` instead of `buildx build --push`.
# Why: on Docker Desktop with the containerd image store, the build finishes
# ("#44 DONE", image already unpacked into the local store) but the docker CLI
# can wedge on its post-build "View build details" / build-record step and never
# return - so push & cleanup never run. Invoke-ResilientBuild detects that the
# image is already in the store and stops waiting on the wedged CLI.
# `docker build` is still BuildKit under DOCKER_BUILDKIT=1, so RUN --mount caches
# keep working. No --provenance: that is a buildx-only flag.
$buildArgs = @(
    'build',
    '-t', $RemoteTag,
    '-t', $LocalTag,
    '--progress=plain',
    '--build-arg', "VERSION=$Version",
    '--build-arg', "COMMIT=$Commit",
    '--build-arg', "DATE=$BuildDate",
    '--build-arg', 'GOPROXY=https://goproxy.cn,direct',
    '--build-arg', 'GOSUMDB=sum.golang.google.cn',
    '--build-arg', 'NPM_CONFIG_REGISTRY=https://registry.npmmirror.com',
    '-f', 'Dockerfile', '.'
)
$buildExit = Invoke-ResilientBuild -DockerArgs $buildArgs -ReadyTag $RemoteTag
Write-StepLog "<<< STEP: docker build done (exit=$buildExit)"
Assert-DockerOk -ExitCode $buildExit -ErrorMessage '[ERROR] docker build failed.'

# Guard: make sure the image really exists before pushing (covers the case where
# the CLI was killed - it should still exist, but verify rather than assume).
if (-not (Test-DockerImageExists -Tag $RemoteTag)) {
    Write-Host "[ERROR] image $RemoteTag not found in local store after build." -ForegroundColor Red
    exit 1
}

Write-StepLog ">>> STEP: docker push ($RemoteTag)"
& docker push $RemoteTag
$pushExit = $LASTEXITCODE
Write-StepLog "<<< STEP: docker push done (exit=$pushExit)"
Assert-DockerOk -ExitCode $pushExit -ErrorMessage '[ERROR] docker push failed. Re-run to login again if unauthorized.'

Write-StepLog '>>> STEP: docker builder prune (clean build cache)'
docker builder prune -f
Write-StepLog '<<< STEP: build cache cleaned'

Write-StepLog '>>> STEP: remove all local sub2api images'
$localImages = docker images --format '{{.Repository}}:{{.Tag}}' | Where-Object { $_ -match 'sub2api' }
if ($localImages) {
    foreach ($img in $localImages) {
        docker rmi $img 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[INFO] Removed: $img"
        } else {
            Write-Host "[WARN] Could not remove: $img (in use?)"
        }
    }
} else {
    Write-Host '[INFO] No local sub2api images found.'
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
