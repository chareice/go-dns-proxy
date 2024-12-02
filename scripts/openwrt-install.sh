#!/bin/sh

# 配置信息
GITHUB_REPO="chareice/go-dns-proxy"
INSTALL_DIR="/usr/bin"
SERVICE_NAME="go-dns-proxy"
CONFIG_DIR="/etc/config"
ARCH="$(uname -m)"

# 根据架构选择正确的二进制文件
case "$ARCH" in
    "x86_64")
        ARCH_NAME="amd64"
        ;;
    "aarch64")
        ARCH_NAME="arm64"
        ;;
    "armv7l")
        ARCH_NAME="arm"
        ;;
    *)
        echo "不支持的架构: $ARCH"
        exit 1
        ;;
esac

# 创建配置目录
mkdir -p "$CONFIG_DIR"

# 获取最新版本号
echo "正在检查最新版本..."
LATEST_VERSION=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "无法获取最新版本信息"
    exit 1
fi

# 检查当前版本
CURRENT_VERSION=""
if [ -f "$INSTALL_DIR/go-dns-proxy" ]; then
    CURRENT_VERSION=$("$INSTALL_DIR/go-dns-proxy" --version 2>/dev/null || echo "")
fi

if [ "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
    echo "已经是最新版本: $LATEST_VERSION"
    exit 0
fi

# 下载最新版本
echo "正在下载版本 $LATEST_VERSION..."
DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_VERSION/go-dns-proxy_${LATEST_VERSION}_linux_${ARCH_NAME}.tar.gz"
TMP_DIR=$(mktemp -d)
curl -L "$DOWNLOAD_URL" -o "$TMP_DIR/go-dns-proxy.tar.gz"

if [ ! -f "$TMP_DIR/go-dns-proxy.tar.gz" ]; then
    echo "下载失败"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 解压并安装
cd "$TMP_DIR"
tar xzf go-dns-proxy.tar.gz
chmod +x go-dns-proxy

# 停止现有服务
if [ -f "/etc/init.d/$SERVICE_NAME" ]; then
    /etc/init.d/$SERVICE_NAME stop
fi

# 安装二进制文件
mv go-dns-proxy "$INSTALL_DIR/"

# 创建配置文件（如果不存在）
if [ ! -f "$CONFIG_DIR/$SERVICE_NAME" ]; then
    cat > "$CONFIG_DIR/$SERVICE_NAME" << 'EOF'
config go-dns-proxy 'main'
    option enabled '1'
    option port '53'
    option china_server '114.114.114.114'
    option oversea_server '1.1.1.1'
    option beian_api_key ''
    option beian_cache_file '/etc/go-dns-proxy/beian_cache.json'
    option beian_cache_interval '10'
EOF
fi

# 创建 OpenWrt 服务文件
cat > "/etc/init.d/$SERVICE_NAME" << 'EOF'
#!/bin/sh /etc/rc.common

START=99
USE_PROCD=1
PROG=/usr/bin/go-dns-proxy

start_service() {
    local enabled
    local port
    local china_server
    local oversea_server
    local beian_api_key
    local beian_cache_file
    local beian_cache_interval

    config_load 'go-dns-proxy'
    config_get enabled main 'enabled' '1'
    config_get port main 'port' '53'
    config_get china_server main 'china_server' '114.114.114.114'
    config_get oversea_server main 'oversea_server' '1.1.1.1'
    config_get beian_api_key main 'beian_api_key' ''
    config_get beian_cache_file main 'beian_cache_file' '/etc/go-dns-proxy/beian_cache.json'
    config_get beian_cache_interval main 'beian_cache_interval' '10'

    [ "$enabled" = "1" ] || return

    # 确保缓存目录存在
    mkdir -p "$(dirname "$beian_cache_file")"

    procd_open_instance
    procd_set_param command $PROG start \
        --port "$port" \
        --chinaServer "$china_server" \
        --overSeaServer "$oversea_server" \
        --apiKey "$beian_api_key" \
        --beianCache "$beian_cache_file" \
        --cacheInterval "$beian_cache_interval"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF

# 设置权限
chmod +x "/etc/init.d/$SERVICE_NAME"

# 启用并启动服务
/etc/init.d/$SERVICE_NAME enable
/etc/init.d/$SERVICE_NAME start

# 清理临时文件
rm -rf "$TMP_DIR"

echo "安装完成！服务已启动。"
echo "配置文件位置: $CONFIG_DIR/$SERVICE_NAME"
echo "可以使用以下命令控制服务："
echo "启动: /etc/init.d/$SERVICE_NAME start"
echo "停止: /etc/init.d/$SERVICE_NAME stop"
echo "重启: /etc/init.d/$SERVICE_NAME restart"
echo ""
echo "要修改配置，请编辑 $CONFIG_DIR/$SERVICE_NAME 文件"
echo "特别注意：请设置 beian_api_key 选项为你的 API Key"
echo "修改配置后需要重启服务才能生效" 