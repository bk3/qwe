#!/bin/sh
# Install a checksum-verified GitHub release without changing shell configuration.
set -eu

fail() { printf 'qwe installer: %s\n' "$*" >&2; exit 1; }
for tool in uname mktemp awk chmod mv; do
    command -v "$tool" >/dev/null 2>&1 || fail "required tool not found: $tool"
done
case $(uname -s) in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail 'supported operating systems are Linux and macOS' ;;
esac
case $(uname -m) in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) fail 'supported architectures are amd64 and arm64' ;;
esac
version=${QWE_VERSION:-latest}
case "$version" in
    latest) release_path=latest/download ;;
    v[0-9]*)
        case "$version" in *[!a-zA-Z0-9._-]*) fail 'QWE_VERSION must be a release tag such as v0.1.0' ;; esac
        release_path=download/$version ;;
    *) fail 'QWE_VERSION must be latest or a release tag such as v0.1.0' ;;
esac
base=${QWE_RELEASE_BASE_URL:-https://github.com/bk3/qwe/releases}
base=${base%/}
install_dir=${QWE_INSTALL_DIR:-${HOME:?HOME must be set, or specify QWE_INSTALL_DIR}/.local/bin}
if command -v sha256sum >/dev/null 2>&1; then
    hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
    hash_tool=shasum
else
    fail 'SHA-256 verification requires sha256sum or shasum'
fi
if command -v curl >/dev/null 2>&1; then
    download() { curl -fsSL --retry 3 --connect-timeout 15 --max-time 120 "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
    download() { wget -q -T 120 -O "$2" "$1"; }
else
    fail 'downloads require curl or wget'
fi
mkdir -p "$install_dir"
install_dir=$(cd "$install_dir" && pwd)
[ ! -d "$install_dir/qwe" ] || fail "$install_dir/qwe is a directory"
stage=$(mktemp -d "$install_dir/.qwe-install.XXXXXXXX")
trap 'rm -rf "$stage"' 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
asset=qwe-$os-$arch
url=$base/$release_path
printf 'Downloading %s (%s)\n' "$asset" "$version"
download "$url/$asset" "$stage/qwe" || fail 'binary download failed; check that the release exists'
download "$url/checksums.txt" "$stage/checksums.txt" || fail 'checksum download failed'
expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { count++; hash=$1 } END { if (count == 1) print hash }' "$stage/checksums.txt")
[ ${#expected} -eq 64 ] || fail 'release checksum is missing, malformed, or duplicated'
case "$expected" in *[!0-9a-fA-F]*) fail 'release checksum is malformed' ;; esac
if [ "$hash_tool" = sha256sum ]; then
    actual=$(sha256sum "$stage/qwe" | awk '{print $1}')
else
    actual=$(shasum -a 256 "$stage/qwe" | awk '{print $1}')
fi
expected=$(printf '%s' "$expected" | tr 'A-F' 'a-f')
[ "$actual" = "$expected" ] || fail 'checksum mismatch; existing installation was preserved'
chmod 755 "$stage/qwe"
"$stage/qwe" --version || fail 'downloaded binary could not run; existing installation was preserved'
mv -f "$stage/qwe" "$install_dir/qwe"
printf 'Installed qwe to %s/qwe\n' "$install_dir"
case :${PATH:-}: in
    *:"$install_dir":*) ;;
    *) printf 'Add %s to your PATH, for example:\n  export PATH="%s:$PATH"\n' "$install_dir" "$install_dir" ;;
esac
