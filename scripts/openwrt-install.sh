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

    echo -e "${GREEN}开始安装 go-dns-proxy...${NC}"
    echo -e "系统架构: ${YELLOW}$ARCH${NC}"

    # 创建临时目录
    TMP_DIR=$(mktemp -d)
    if [ ! -d "$TMP_DIR" ]; then
        echo -e "${RED}错误: 无法创建临时目录${NC}"
        exit 1
    }

    # 下载最新版本
    LATEST_VERSION=$(curl -sSL https://api.github.com/repos/chareice/go-dns-proxy/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${RED}错误: 无法获取最新版本信息${NC}"
        rm -rf "$TMP_DIR"
        exit 1
    }

    echo -e "最新版本: ${YELLOW}$LATEST_VERSION${NC}"
    DOWNLOAD_URL="https://github.com/chareice/go-dns-proxy/releases/download/$LATEST_VERSION/go-dns-proxy-$ARCH.tar.gz"

    # 下载并解压
    echo -e "${GREEN}下载程序...${NC}"
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

    # 下载配置文件
    echo -e "${GREEN}安装配置文件...${NC}"
    mkdir -p /etc/config
    download_file "$url/openwrt-config" /etc/config/go-dns-proxy
    download_file "$url/openwrt-init.d" /etc/init.d/go-dns-proxy
    chmod 755 /etc/init.d/go-dns-proxy

    # 启用并启动服务
    echo -e "${GREEN}启用服务...${NC}"
    /etc/init.d/go-dns-proxy enable
    /etc/init.d/go-dns-proxy start

    # 清理
    cd / || exit 1
    rm -rf "$TMP_DIR"

    echo -e "${GREEN}安装完成！${NC}"
    echo -e "配置文件位置: ${YELLOW}/etc/config/go-dns-proxy${NC}"
    echo -e "数据目录: ${YELLOW}/etc/go-dns-proxy/data${NC}"
    echo -e "管理界面: ${YELLOW}http://$(uci get network.lan.ipaddr 2>/dev/null || echo "路由器IP"):8080${NC}"
    echo -e "\n使用以下命令管理服务:"
    echo -e "${YELLOW}/etc/init.d/go-dns-proxy {start|stop|restart|enable|disable}${NC}"
}

# 运行主函数
main 