@echo off
setlocal
where my-codex >nul 2>nul
if %ERRORLEVEL% EQU 0 (
  my-codex pull %*
  exit /b %ERRORLEVEL%
)

go run .\cmd\my-codex pull %*
