"""Generate protobuf protocols and Luban configuration from the repository root."""
from pathlib import Path
import argparse
import hashlib
import os
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / '.codex-work' / 'tools'

def run(*args):
    subprocess.run([str(x) for x in args], cwd=ROOT, check=True)

def download(url, path, digest=None):
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists():
        partial = path.with_suffix(path.suffix + '.download')
        try:
            run('curl.exe', '-fL', '--retry', '2', '--max-time', '180', '-o', partial, url)
            partial.replace(path)
        finally:
            partial.unlink(missing_ok=True)
    if digest and hashlib.sha256(path.read_bytes()).hexdigest() != digest:
        raise RuntimeError(f'Checksum mismatch: {path}; remove it and retry')

def protobuf():
    compiler = Path(os.environ.get('PROTOC', TOOLS / 'protoc-33.0/bin/protoc.exe'))
    if not compiler.exists():
        archive = TOOLS / 'protoc-33.0.zip'
        download('https://github.com/protocolbuffers/protobuf/releases/download/v33.0/protoc-33.0-win64.zip', archive)
        with zipfile.ZipFile(archive) as z:
            z.extractall(TOOLS / 'protoc-33.0')
    plugin = TOOLS / 'protoc-gen-go.exe'
    plugin.parent.mkdir(parents=True, exist_ok=True)
    # Build the generator from the version pinned by go.mod, without global installation.
    run('go', 'build', '-o', plugin, 'google.golang.org/protobuf/cmd/protoc-gen-go')
    sources = [p.relative_to(ROOT) for p in sorted((ROOT / 'proto').glob('*.proto'))]
    if not sources:
        raise RuntimeError('No proto/*.proto sources found')
    run(compiler, '-I', 'proto', f'--plugin=protoc-gen-go={plugin}', '--go_out=.',
        '--go_opt=module=due-server', *sources)

def config():
    dll = Path(os.environ.get('LUBAN_DLL', TOOLS / 'luban-5.1.0/Luban/Luban.dll'))
    if not dll.exists():
        archive = TOOLS / 'Luban-5.1.0.7z'
        download('https://github.com/focus-creative-games/luban/releases/download/v5.1.0/Luban.7z', archive,
                 'bac9a1b8d69cfeaa7ef1d8c67b34a007324d1cd158e0b14294521b3879ba5213')
        extractor = TOOLS / '7zr.exe'
        download('https://www.7-zip.org/a/7zr.exe', extractor)
        run(extractor, 'x', archive, f'-o{TOOLS / "luban-5.1.0"}', '-y')
    run('dotnet', dll, '--conf', 'Excel/luban.conf', '-t', 'server', '-c', 'go-json', '-d', 'json',
        '--strict', '-x', 'outputCodeDir=internal/common/gamedata/generated',
        '-x', 'outputDataDir=config/generated', '-x', 'lubanGoModule=due-server/internal/common/luban')
    run('gofmt', '-w', *sorted((ROOT / 'internal/common/gamedata/generated').glob('*.go')))

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('target', choices=['proto', 'config', 'all'])
    args = parser.parse_args()
    if args.target in ('proto', 'all'): protobuf()
    if args.target in ('config', 'all'): config()

if __name__ == '__main__':
    try:
        main()
    except (subprocess.CalledProcessError, OSError, RuntimeError) as e:
        print(f'Generation failed: {e}', file=sys.stderr)
        sys.exit(1)
