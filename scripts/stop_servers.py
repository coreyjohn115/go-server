"""Gracefully stop only server executables belonging to this checkout (Windows)."""

import ctypes
from ctypes import wintypes
from pathlib import Path
import subprocess
import sys
import time


kernel = ctypes.WinDLL("kernel32", use_last_error=True)
psapi = ctypes.WinDLL("psapi", use_last_error=True)
kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
kernel.OpenProcess.restype = wintypes.HANDLE
kernel.CloseHandle.argtypes = [wintypes.HANDLE]
kernel.QueryFullProcessImageNameW.argtypes = [
    wintypes.HANDLE, wintypes.DWORD, wintypes.LPWSTR, ctypes.POINTER(wintypes.DWORD)
]
kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
kernel.WaitForSingleObject.restype = wintypes.DWORD
psapi.EnumProcesses.argtypes = [ctypes.c_void_p, wintypes.DWORD, ctypes.POINTER(wintypes.DWORD)]
kernel.AttachConsole.argtypes = [wintypes.DWORD]
kernel.SetConsoleCtrlHandler.argtypes = [ctypes.c_void_p, wintypes.BOOL]
kernel.GenerateConsoleCtrlEvent.argtypes = [wintypes.DWORD, wintypes.DWORD]


def signal_console(pid):
    # Isolate console attachment and our Ctrl+C ignore flag in a short-lived helper.
    kernel.FreeConsole()
    if not kernel.AttachConsole(pid):
        return 1
    if not kernel.SetConsoleCtrlHandler(None, True):
        return 1
    sent = kernel.GenerateConsoleCtrlEvent(0, 0)
    time.sleep(0.2)
    kernel.FreeConsole()
    return 0 if sent else 1


def server_processes():
    targets = {
        str((Path(__file__).resolve().parents[1] / "bin" / f"{name}.exe")).casefold(): name
        for name in ("gate", "battle", "logic", "backend")
    }
    capacity = 4096
    while True:
        pids = (wintypes.DWORD * capacity)()
        used = wintypes.DWORD()
        if not psapi.EnumProcesses(pids, ctypes.sizeof(pids), ctypes.byref(used)):
            raise ctypes.WinError(ctypes.get_last_error())
        if used.value < ctypes.sizeof(pids):
            break
        capacity *= 2
    found = []
    for pid in pids[:used.value // ctypes.sizeof(wintypes.DWORD)]:
        handle = kernel.OpenProcess(0x1000 | 0x100000, False, pid)
        if not handle:
            continue
        path = ctypes.create_unicode_buffer(32768)
        size = wintypes.DWORD(len(path))
        if kernel.QueryFullProcessImageNameW(handle, 0, path, ctypes.byref(size)):
            name = targets.get(path.value.casefold())
            if name:
                found.append((name, pid, handle))
                continue
        kernel.CloseHandle(handle)
    return found


def main():
    processes = server_processes()
    try:
        # Stop ingress first, then stateful services, then backend.
        for name in ("gate", "battle", "logic", "backend"):
            group = [(pid, handle) for service, pid, handle in processes if service == name]
            for pid, handle in group:
                if kernel.WaitForSingleObject(handle, 0) == 0:
                    continue
                print(f"Stopping {name} (PID {pid})...", flush=True)
                result = subprocess.run([sys.executable, __file__, "--signal", str(pid)])
                if result.returncode and kernel.WaitForSingleObject(handle, 0) != 0:
                    print(f"Cannot signal PID {pid}. Stop it with Ctrl+C in its console.", file=sys.stderr)
                    return 1
            deadline = time.monotonic() + 40
            for pid, handle in group:
                remaining = max(0, int((deadline - time.monotonic()) * 1000))
                if kernel.WaitForSingleObject(handle, remaining) != 0:
                    print(f"PID {pid} did not exit within 40s; startup cancelled.", file=sys.stderr)
                    return 1
        print("Previous project servers stopped (or none running).")
        return 0
    finally:
        for _, _, handle in processes:
            kernel.CloseHandle(handle)


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--signal":
        sys.exit(signal_console(int(sys.argv[2])))
    sys.exit(main())
