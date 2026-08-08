#!/usr/bin/env sh

set -eu

repo_root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' 0 1 2 15

fake_bin="$test_root/bin"
install_dir="$test_root/install"
mkdir -p "$fake_bin" "$install_dir"

cat > "$fake_bin/curl" <<'EOF'
#!/usr/bin/env sh
set -eu
output=""
url=""
write_format=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        -o)
            output="$2"
            shift 2
            ;;
        --proto|--tlsv1.2)
            if [ "$1" = "--proto" ]; then shift 2; else shift; fi
            ;;
        -w)
            write_format="$2"
            shift 2
            ;;
        -*) shift ;;
        *) url="$1"; shift ;;
    esac
done
case "$url:$write_format" in
    */releases/latest:*url_effective*)
        printf 'https://github.com/lznauy/NekoCode/releases/tag/vtest'
        exit 0
        ;;
esac
if [ -z "$output" ]; then
    exit 2
fi
case "$url" in
    */SHA256SUMS)
        [ "${MOCK_SUMS_AVAILABLE:-true}" = "true" ] || exit 22
        printf '%s  nekocode-tui-linux-amd64\n' "$MOCK_CHECKSUM" > "$output"
        ;;
    *) printf 'test binary\n' > "$output" ;;
esac
EOF
chmod +x "$fake_bin/curl"

payload="$test_root/payload"
printf 'test binary\n' > "$payload"
checksum="$(sha256sum "$payload" | awk '{ print $1 }')"

PATH="$fake_bin:$PATH" MOCK_CHECKSUM="$checksum" \
    sh "$repo_root/scripts/install.sh" --version vtest --dir "$install_dir" >/dev/null

cmp "$payload" "$install_dir/nekocode-tui"

latest_dir="$test_root/latest"
mkdir -p "$latest_dir"
PATH="$fake_bin:$PATH" MOCK_CHECKSUM="$checksum" \
    sh "$repo_root/scripts/install.sh" --dir "$latest_dir" >/dev/null
cmp "$payload" "$latest_dir/nekocode-tui"

expected="$test_root/expected"
printf 'existing binary\n' > "$expected"

unsupported_dir="$test_root/unsupported"
mkdir -p "$unsupported_dir"
printf 'existing binary\n' > "$unsupported_dir/nekocode-tui"
if PATH="$fake_bin:$PATH" MOCK_CHECKSUM="$checksum" MOCK_SUMS_AVAILABLE=false \
    sh "$repo_root/scripts/install.sh" --version vtest --dir "$unsupported_dir" >/dev/null 2>&1; then
    echo "release without a trusted checksum unexpectedly succeeded" >&2
    exit 1
fi
cmp "$expected" "$unsupported_dir/nekocode-tui"

pinned_dir="$test_root/pinned"
mkdir -p "$pinned_dir"
printf 'existing binary\n' > "$pinned_dir/nekocode-tui"
if PATH="$fake_bin:$PATH" MOCK_CHECKSUM="$checksum" \
    sh "$repo_root/scripts/install.sh" --version v0.4.2 --dir "$pinned_dir" >/dev/null 2>&1; then
    echo "remote checksum unexpectedly overrode the pinned v0.4.2 checksum" >&2
    exit 1
fi
cmp "$expected" "$pinned_dir/nekocode-tui"

printf 'existing binary\n' > "$install_dir/nekocode-tui"
if PATH="$fake_bin:$PATH" MOCK_CHECKSUM="0000000000000000000000000000000000000000000000000000000000000000" \
    sh "$repo_root/scripts/install.sh" --version vtest --dir "$install_dir" >/dev/null 2>&1; then
    echo "checksum mismatch unexpectedly succeeded" >&2
    exit 1
fi

cmp "$expected" "$install_dir/nekocode-tui"

echo "installer tests passed"
