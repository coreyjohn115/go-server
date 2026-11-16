@echo off
py "%~dp0..\scripts\generate.py" config
exit /b %errorlevel%
