@echo off
cd /d "%~dp0"

REM Enable BuildKit for layer cache + mount cache (faster repeat builds)
set "DOCKER_BUILDKIT=1"

REM Proxy for docker build (change port if needed; leave empty to disable)
set "HTTP_PROXY=http://127.0.0.1:7897"
set "HTTPS_PROXY=http://127.0.0.1:7897"

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\docker-push.ps1"
set EXITCODE=%ERRORLEVEL%
if %EXITCODE% neq 0 pause
exit /b %EXITCODE%
