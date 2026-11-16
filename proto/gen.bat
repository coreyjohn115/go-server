@echo off
py "%~dp0..\scripts\generate.py" proto
exit /b %errorlevel%
