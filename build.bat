@echo off
cd /d "%~dp0"
set GO="C:\Program Files\Go\bin\go.exe"
echo Building sabre-seat-block.exe...
%GO% mod tidy
if %errorlevel% neq 0 (
    echo go mod tidy failed.
    pause
    exit /b %errorlevel%
)
%GO% build -o sabre-seat-block.exe -ldflags="-s -w" .
if %errorlevel% equ 0 (
    echo Done! Output: sabre-seat-block.exe
) else (
    echo Build failed.
)
pause
