@echo off
cd /d "%~dp0"
set GO="C:\Program Files\Go\bin\go.exe"
echo Building sabre-monitor.exe...
%GO% mod tidy
if %errorlevel% neq 0 (
    echo go mod tidy failed.
    pause
    exit /b %errorlevel%
)
%GO% build -o sabre-monitor.exe -ldflags="-s -w" .
if %errorlevel% equ 0 (
    echo Done! Output: sabre-monitor.exe
) else (
    echo Build failed.
)
pause
