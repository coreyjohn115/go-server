@echo off
py "%~dp0scripts\generate.py" all
exit /b %errorlevel%
