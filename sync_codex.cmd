@echo off
setlocal
where my-codex >nul 2>nul
if %ERRORLEVEL% EQU 0 (
  my-codex sync %*
  exit /b %ERRORLEVEL%
)

go install .\cmd\my-codex
if %ERRORLEVEL% NEQ 0 exit /b %ERRORLEVEL%

for /f "delims=" %%i in ('go env GOBIN') do set "GOBIN=%%i"
if not defined GOBIN (
  for /f "delims=" %%i in ('go env GOPATH') do set "GOBIN=%%i\bin"
)

"%GOBIN%\my-codex.exe" sync %*
exit /b %ERRORLEVEL%
