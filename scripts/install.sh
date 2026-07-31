#!/usr/bin/env sh
#
# NekoCode 一键安装脚本
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/lznauy/NekoCode/master/scripts/install.sh | sh
#   curl -fsSL ... | sh -s -- --version v0.3.3   # 安装指定版本
#   curl -fsSL ... | sh -s -- --dir /custom/path  # 安装到自定义目录
#
# 支持平台: Linux / macOS (amd64 / arm64)
# 安装位置: 优先 ~/.local/bin，无法写入时尝试 /usr/local/bin（可能需要 sudo）

set -eu

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
        tag="$(
            curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
                | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
        )"
        if [ -z "$tag" ]; then
            echo "错误: 无法获取最新版本号，请检查网络或改用 --version 指定版本。" >&2
            exit 1
        fi
    else
        tag="$version"
    fi
    # 去掉可能的前缀 "v"
    ver="${tag#v}"
}

# ---------- 确定安装目录 ----------
resolve_install_dir() {
    if [ -n "$install_dir" ]; then
        mkdir -p "$install_dir" 2>/dev/null || true
        echo "$install_dir"
        return
    fi

    # 优先用户目录，无需 sudo
    user_bin="$HOME/.local/bin"
    if [ -d "$HOME/.local" ] && [ -w "$HOME/.local" ]; then
        mkdir -p "$user_bin"
        echo "$user_bin"
        return
    fi

    # 兜底系统目录
    echo "/usr/local/bin"
}

# ---------- 下载并安装 ----------
install_binary() {
    asset="nekocode-tui-$os-$arch"
    url="https://github.com/$REPO/releases/download/$tag/$asset"
    target="$1/$BINARY"

    echo "下载: $url" >&2
    echo "版本: $tag" >&2

    curl -fsSL "$url" -o "$target.tmp" || {
        echo "错误: 下载失败。请检查: 版本号是否正确、网络是否可达。" >&2
        rm -f "$target.tmp"
        exit 1
    }

    chmod +x "$target.tmp"
    mv "$target.tmp" "$target"

    echo ""
    echo "✅ 已安装到: $target" >&2
}

# ---------- 验证 ----------
verify_binary() {
    target="$1/$BINARY"
    if [ -x "$target" ] && [ -s "$target" ]; then
        echo "✅ 安装成功！文件已就绪。" >&2
    else
        echo "⚠️  文件存在但检查未通过，请手动确认: $target" >&2
    fi
}

# ---------- 主流程 ----------
resolve_version
install_dir="$(resolve_install_dir)"

if [ -n "$install_dir" ] && [ ! -w "$install_dir" ]; then
    echo "提示: 目录 $install_dir 需要写权限，请手动运行:" >&2
    echo "  sudo $0 $*" >&2
    exit 1
fi

install_binary "$install_dir"
verify_binary "$install_dir"

echo ""
echo "将 $install_dir 加入 PATH 后，运行: nekocode-tui" >&2
