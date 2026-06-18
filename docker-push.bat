@echo off
cd /d "%~dp0"

REM Proxy for docker build (change port if needed; leave empty to disable)
set "HTTP_PROXY=http://127.0.0.1:7897"
set "HTTPS_PROXY=http://127.0.0.1:7897"

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\docker-push.ps1"
if errorlevel 1 pause
exit /b %errorlevel%
