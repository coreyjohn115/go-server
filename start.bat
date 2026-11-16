@echo off
setlocal
cd /d "%~dp0"
if /i "%~1"=="--help" goto help
if not "%~1"=="" if /i not "%~1"=="--compose" goto help
py scripts\stop_servers.py
if errorlevel 1 exit /b 1
if /i "%~1"=="--compose" (
    docker compose up -d
    if errorlevel 1 exit /b 1
)
if not exist bin mkdir bin
for %%S in (backend logic battle gate robot) do (
    echo Building %%S...
    go build -o "bin\%%S.exe" ./cmd/%%S
    if errorlevel 1 exit /b 1
)
for %%S in (backend logic battle gate) do (
    start "due %%S" /D "%~dp0" "%~dp0bin\%%S.exe"
)
echo Servers launched in separate consoles. Check each console for startup errors.
echo Run bin\robot.exe after all servers are ready. Press Ctrl+C in each console to stop.
exit /b 0
:help
echo Usage: start.bat [--compose]
echo   Default: stop previous project servers, then build and start them.
echo   Requires Go and Python launcher (py). Graceful stop timeout: 40s per service.
echo   --compose: also start etcd, Redis and MongoDB with Docker Compose.
echo   Docker MongoDB uses port 27018; local MongoDB may keep using port 27017.
exit /b 0
