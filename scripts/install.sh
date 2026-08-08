#!/usr/bin/env sh
#
# NekoCode 一键安装脚本
#
# 用法:
#   curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/lznauy/NekoCode/master/scripts/install.sh | sh
#   curl --proto '=https' --tlsv1.2 -fsSL ... | sh -s -- --version v0.3.3   # 安装指定版本
#   curl --proto '=https' --tlsv1.2 -fsSL ... | sh -s -- --dir /custom/path  # 安装到自定义目录
#
# 支持平台: Linux / macOS (amd64 / arm64)
# 安装位置: ~/.local/bin（免 sudo），可用 --dir 指定其他目录

set -eu
umask 077

REPO="lznauy/NekoCode"
BINARY="nekocode-tui"
DEFAULT_VERSION="latest"

version="$DEFAULT_VERSION"
install_dir=""

# ---------- 参数解析 ----------
while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            [ "$#" -ge 2 ] || { echo "错误: --version 需要一个版本号，例如 v0.3.3"; exit 1; }
            version="$2"
            shift 2
            ;;
        --dir)
            [ "$#" -ge 2 ] || { echo "错误: --dir 需要一个目录路径"; exit 1; }
            install_dir="$2"
            shift 2
            ;;
        -h|--help)
            echo "NekoCode 安装脚本"
            echo ""
            echo "选项:"
            echo "  --version <vX.Y.Z>   安装指定版本（默认: latest）"
            echo "  --dir <路径>         安装到指定目录（默认: ~/.local/bin）"
            echo "  -h, --help           显示帮助"
            exit 0
            ;;
        *)
            echo "未知参数: $1 （可用 --help 查看帮助）" >&2
            exit 1
            ;;
    esac
done

# ---------- 检测系统与架构 ----------
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
        echo "错误: 不支持的架构: $arch" >&2
        echo "NekoCode 目前支持 amd64 和 arm64。" >&2
        exit 1
        ;;
esac

case "$os" in
    linux|darwin) ;;
    *)
        echo "错误: 不支持的平台: $os" >&2
        echo "NekoCode 目前支持 Linux 和 macOS。Windows 用户请通过 WSL 运行。" >&2
        exit 1
        ;;
esac

# ---------- 解析最新版本号 ----------
resolve_version() {
    if [ "$version" = "latest" ]; then
        echo "正在获取最新版本号..." >&2
        latest_url="$(curl --proto '=https' --tlsv1.2 -fsSLI -o /dev/null \
            -w '%{url_effective}' "https://github.com/$REPO/releases/latest")"
        tag="${latest_url##*/}"
        if [ -z "$tag" ]; then
            echo "错误: 无法获取最新版本号，请检查网络或改用 --version 指定版本。" >&2
            exit 1
        fi
    else
        tag="$version"
    fi
    case "$tag" in
        *[!A-Za-z0-9._+-]*)
            echo "错误: 版本号包含不支持的字符: $tag" >&2
            exit 1
            ;;
    esac
}

legacy_checksum() {
    case "$tag:$asset" in
        v0.4.2:nekocode-tui-darwin-amd64) echo "208de4dcde64bfd9d335c5521f90496b3974a8872f6e662bf8a977a2462f6e54" ;;
        v0.4.2:nekocode-tui-darwin-arm64) echo "437345f5c27d3265c3ce9de2c67a89f72732c12ae51fa2205d1ffd10c5002355" ;;
        v0.4.2:nekocode-tui-linux-amd64) echo "9108d43194a2a5ede5ba6339934c71c9ad9ee1cabb00331d49aab9731b17f62e" ;;
        v0.4.2:nekocode-tui-linux-arm64) echo "f2ab078d9557bbc3a9f20446fb906bf4d583176a0cce7da1b43882638db050eb" ;;
        *) return 1 ;;
    esac
}

# ---------- 下载并安装 ----------
install_binary() {
    asset="nekocode-tui-$os-$arch"
    url="https://github.com/$REPO/releases/download/$tag/$asset"
    checksum_url="https://github.com/$REPO/releases/download/$tag/SHA256SUMS"
    target="$1/$BINARY"
    tmp="$(mktemp "$1/.nekocode-install.XXXXXX")" || {
        echo "错误: 无法在安装目录创建临时文件。" >&2
        exit 1
    }
    sums="$tmp.sha256sums"
    trap 'rm -f "$tmp" "$sums"' 0 1 2 15

    echo "下载: $url" >&2
    echo "版本: $tag" >&2

    curl --proto '=https' --tlsv1.2 -fsSL "$url" -o "$tmp" || {
        echo "错误: 下载失败。请检查: 版本号是否正确、网络是否可达。" >&2
        exit 1
    }
    if expected="$(legacy_checksum)"; then
        echo "使用安装器内置的 $tag 校验值。" >&2
    elif curl --proto '=https' --tlsv1.2 -fsSL "$checksum_url" -o "$sums" 2>/dev/null; then
        expected="$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$sums")"
    else
        expected=""
    fi
    if [ -z "$expected" ]; then
        echo "错误: 无法获取 $asset 的可信 SHA-256，已取消安装。" >&2
        exit 1
    fi
    if [ "${#expected}" -ne 64 ] || [ -n "$(printf '%s' "$expected" | tr -d '0-9A-Fa-f')" ]; then
        echo "错误: $asset 的 SHA-256 格式无效，已取消安装。" >&2
        exit 1
    fi
    expected="$(printf '%s' "$expected" | tr 'A-F' 'a-f')"
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$tmp" | awk '{ print $1 }')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$tmp" | awk '{ print $1 }')"
    else
        echo "错误: 系统缺少 sha256sum 或 shasum，无法校验下载文件。" >&2
        exit 1
    fi
    if [ "$actual" != "$expected" ]; then
        echo "错误: 下载文件的 SHA-256 校验失败，已取消安装。" >&2
        exit 1
    fi

    chmod 0755 "$tmp"
    mv "$tmp" "$target"
    rm -f "$sums"
    trap - 0 1 2 15

    echo ""
    echo "已安装并通过 SHA-256 校验: $target" >&2
}

# ---------- 验证 ----------
verify_binary() {
    target="$1/$BINARY"
    if [ -x "$target" ] && [ -s "$target" ]; then
        echo "安装成功，文件已就绪。" >&2
    else
        echo "警告: 文件存在但检查未通过，请手动确认: $target" >&2
    fi
}

# ---------- 主流程 ----------
resolve_version

# 安装目录: 默认 ~/.local/bin（免 sudo），目录不可写时提示用户处理。
[ -n "$install_dir" ] || install_dir="$HOME/.local/bin"
case "$install_dir" in
    /*) ;;
    *) install_dir="$(pwd)/$install_dir" ;;
esac
mkdir -p "$install_dir" 2>/dev/null || true
if [ ! -d "$install_dir" ] || [ ! -w "$install_dir" ]; then
    echo "错误: 目录 $install_dir 不存在或不可写。" >&2
    echo "请检查权限，或用 --dir 指定其他目录:" >&2
    echo "  curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/$REPO/master/scripts/install.sh | sh -s -- --dir /你的/目录" >&2
    exit 1
fi

install_binary "$install_dir"
verify_binary "$install_dir"

echo ""
case ":$PATH:" in
    *":$install_dir:"*) echo "运行 nekocode-tui 即可启动。" >&2 ;;
    *) echo "将 $install_dir 加入 PATH 后，运行 nekocode-tui 即可启动。" >&2 ;;
esac
