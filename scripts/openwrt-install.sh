#!/bin/sh

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检测架构
check_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        armv7l|armv7)
            echo "arm"
            ;;
        mips|mips64)
            echo "mips"
            ;;
        mipsel|mips64el)
            echo "mipsle"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# 下载文件
download_file() {
    local url="$1"
    local output="$2"
    
    if command -v curl >/dev/null 2>&1; then
        curl -sSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        echo -e "${RED}错误: 未找到 curl 或 wget${NC}"
        return 1
    fi
}

# 获取当前版本
get_current_version() {
    if [ -f "/usr/bin/go-dns-proxy" ]; then
        /usr/bin/go-dns-proxy --version 2>/dev/null || echo ""
    else
        echo ""
    fi
}

# 主安装函数
main() {
    # 检查是否为 root
    if [ "$(id -u)" != "0" ]; then
        echo -e "${RED}错误: 请使用 root 权限运行此脚本${NC}"
        exit 1
    fi

    # 获取系统架构
    ARCH=$(check_arch)
    if [ "$ARCH" = "unknown" ]; then
        echo -e "${RED}错误: 不支持的系统架构${NC}"
        exit 1
    fi

    echo -e "${GREEN}检查版本信息...${NC}"
    
    # 获取当前版本
    CURRENT_VERSION=$(get_current_version)
    if [ -n "$CURRENT_VERSION" ]; then
        echo -e "当前版本: ${YELLOW}$CURRENT_VERSION${NC}"
    fi

    # 获取最新版本
    LATEST_VERSION=$(curl -sSL https://api.github.com/repos/chareice/go-dns-proxy/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${RED}错误: 无法获取最新版本信息${NC}"
        exit 1
    fi
    echo -e "最新版本: ${YELLOW}$LATEST_VERSION${NC}"

    # 检查是否需要升级
    if [ -n "$CURRENT_VERSION" ] && [ "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
        echo -e "${GREEN}已经是最新版本，无需升级${NC}"
        exit 0
    fi

    # 如果已安装，先停止服务
    if [ -f "/etc/init.d/go-dns-proxy" ]; then
        echo -e "${YELLOW}停止当前服务...${NC}"
        /etc/init.d/go-dns-proxy stop
    fi

    echo -e "${GREEN}开始${CURRENT_VERSION:+升}安装 go-dns-proxy...${NC}"
    echo -e "系统架构: ${YELLOW}$ARCH${NC}"

    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    if [ ! -d "$TMP_DIR" ]; then
        echo -e "${RED}错误: 无法创建临时目录${NC}"
        exit 1
    fi

    # 下载并解压
    echo -e "${GREEN}下载程序...${NC}"
    DOWNLOAD_URL="https://github.com/chareice/go-dns-proxy/releases/download/$LATEST_VERSION/go-dns-proxy-$ARCH.tar.gz"
    if ! download_file "$DOWNLOAD_URL" "$TMP_DIR/go-dns-proxy.tar.gz"; then
        echo -e "${RED}错误: 下载失败${NC}"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    cd "$TMP_DIR" || exit 1
    tar xzf go-dns-proxy.tar.gz

    # 安装程序
    echo -e "${GREEN}安装程序...${NC}"
    install -m 755 go-dns-proxy /usr/bin/
    mkdir -p /etc/go-dns-proxy/data

    # 下载配置文件（仅在首次安装时）
    if [ -z "$CURRENT_VERSION" ]; then
        echo -e "${GREEN}安装配置文件...${NC}"
        mkdir -p /etc/config
        download_file "$url/openwrt-config" /etc/config/go-dns-proxy
        download_file "$url/openwrt-init.d" /etc/init.d/go-dns-proxy
        chmod 755 /etc/init.d/go-dns-proxy

        # 首次安装时启用服务
        echo -e "${GREEN}启用服务...${NC}"
        /etc/init.d/go-dns-proxy enable
    fi

    # 启动服务
    echo -e "${GREEN}启动服务...${NC}"
    /etc/init.d/go-dns-proxy start

    # 清理
    cd / || exit 1
    rm -rf "$TMP_DIR"

    echo -e "${GREEN}${CURRENT_VERSION:+升}安装完成！${NC}"
    if [ -z "$CURRENT_VERSION" ]; then
        echo -e "配置文件位置: ${YELLOW}/etc/config/go-dns-proxy${NC}"
        echo -e "数据目录: ${YELLOW}/etc/go-dns-proxy/data${NC}"
        echo -e "管理界面: ${YELLOW}http://$(uci get network.lan.ipaddr 2>/dev/null || echo "路由器IP"):8080${NC}"
        echo -e "\n使用以下命令管理服务:"
        echo -e "${YELLOW}/etc/init.d/go-dns-proxy {start|stop|restart|enable|disable}${NC}"
    fi
}

# 运行主函数
main 