#!/bin/sh
# Isolated installer integration test. Python serves local release fixtures only.
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
server_pid=
cleanup() {
    test_status=$?
    if [ "$test_status" -ne 0 ]; then
        printf 'Installer test failed (exit %s); captured output:\n' "$test_status" >&2
        for log in output failure latest-output server.log; do
            if [ -f "$tmp/$log" ]; then printf '\n%s:\n' "$log" >&2; cat "$tmp/$log" >&2; fi
        done
    fi
    if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || :; wait "$server_pid" 2>/dev/null || :; fi
    rm -rf "$tmp"
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
case $(uname -s) in Linux) os=linux ;; Darwin) os=darwin ;; *) exit 0 ;; esac
case $(uname -m) in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) exit 0 ;; esac
asset=qwe-$os-$arch
mkdir -p "$tmp/releases/download/v0.0.0" "$tmp/releases/latest/download"
cat > "$tmp/releases/download/v0.0.0/$asset" <<'BINARY'
#!/bin/sh
[ "$1" = --version ] || exit 1
printf 'qwe v0.0.0\n'
BINARY
if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "$tmp/releases/download/v0.0.0/$asset" | awk '{print $1}')
else
    digest=$(shasum -a 256 "$tmp/releases/download/v0.0.0/$asset" | awk '{print $1}')
fi
printf '%s  %s\n' "$digest" "$asset" > "$tmp/releases/download/v0.0.0/checksums.txt"
cp "$tmp/releases/download/v0.0.0/"* "$tmp/releases/latest/download/"
python3 - "$tmp" <<'PY' > "$tmp/server.log" 2>&1 &
import http.server, pathlib, sys
root = pathlib.Path(sys.argv[1])
handler = lambda *args, **kwargs: http.server.SimpleHTTPRequestHandler(*args, directory=str(root), **kwargs)
server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), handler)
(root / 'port').write_text(str(server.server_port))
server.serve_forever()
PY
server_pid=$!
attempt=0
while [ ! -f "$tmp/port" ]; do
    attempt=$((attempt + 1))
    [ "$attempt" -lt 50 ] || { cat "$tmp/server.log" >&2; exit 1; }
    sleep 0.1
done
base=http://127.0.0.1:$(cat "$tmp/port")/releases
QWE_INSTALL_DIR="$tmp/custom bin" QWE_VERSION=v0.0.0 QWE_RELEASE_BASE_URL="$base" sh "$repo/scripts/install.sh" > "$tmp/output" 2>&1
[ "$("$tmp/custom bin/qwe" --version)" = 'qwe v0.0.0' ]
[ -x "$tmp/custom bin/qwe" ]
cp "$tmp/custom bin/qwe" "$tmp/existing"
printf '%064d  %s\n' 0 "$asset" > "$tmp/releases/download/v0.0.0/checksums.txt"
if QWE_INSTALL_DIR="$tmp/custom bin" QWE_VERSION=v0.0.0 QWE_RELEASE_BASE_URL="$base" sh "$repo/scripts/install.sh" > "$tmp/failure" 2>&1; then
    printf 'FAIL: invalid checksum was accepted\n' >&2
    exit 1
fi
cmp "$tmp/existing" "$tmp/custom bin/qwe"
awk '/checksum mismatch/ { found=1 } END { exit !found }' "$tmp/failure"
QWE_INSTALL_DIR="$tmp/latest bin" QWE_RELEASE_BASE_URL="$base" sh "$repo/scripts/install.sh" > "$tmp/latest-output" 2>&1
[ "$("$tmp/latest bin/qwe" --version)" = 'qwe v0.0.0' ]
[ -z "$(find "$tmp/custom bin" "$tmp/latest bin" -name '.qwe-install.*' -print)" ]
printf 'Installer tests passed (pinned release, latest, spaces, checksum rejection, atomic preservation, cleanup).\n'
