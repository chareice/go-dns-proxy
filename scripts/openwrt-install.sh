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

echo "最新版本是: $LATEST_VERSION"

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
DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_VERSION/go-dns-proxy_${LATEST_VERSION#v}_linux_${ARCH_NAME}.tar.gz"
echo "正在下载: $DOWNLOAD_URL"

TMP_DIR=$(mktemp -d)
echo "使用临时目录: $TMP_DIR"

# 使用 -L 跟随重定向，使用 -f 失败时显示错误
curl -L -f "$DOWNLOAD_URL" -o "$TMP_DIR/go-dns-proxy.tar.gz"
CURL_EXIT_CODE=$?

if [ $CURL_EXIT_CODE -ne 0 ]; then
    echo "下载失败，curl 退出码: $CURL_EXIT_CODE"
    echo "请检查以下问题："
    echo "1. 网络连接是否正常"
    echo "2. 版本号是否正确：$LATEST_VERSION"
    echo "3. 架构是否正确：$ARCH_NAME"
    echo "4. 完整的下载 URL：$DOWNLOAD_URL"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 检查下载的文件大小
FILE_SIZE=$(ls -l "$TMP_DIR/go-dns-proxy.tar.gz" | awk '{print $5}')
echo "下载的文件大小: $FILE_SIZE 字节"

if [ $FILE_SIZE -lt 1000 ]; then
    echo "下载的文件太小，可能不是有效的压缩包"
    echo "文件内容："
    cat "$TMP_DIR/go-dns-proxy.tar.gz"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 解压并安装
cd "$TMP_DIR"
echo "正在解压文件..."
tar xzf go-dns-proxy.tar.gz
TAR_EXIT_CODE=$?

if [ $TAR_EXIT_CODE -ne 0 ]; then
    echo "解压失败，tar 退出码: $TAR_EXIT_CODE"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 检查解压后的文件是否存在
if [ ! -f "go-dns-proxy" ]; then
    echo "解压后未找到可执行文件"
    echo "目录内容："
    ls -la
    rm -rf "$TMP_DIR"
    exit 1
fi

chmod +x go-dns-proxy
CHMOD_EXIT_CODE=$?

if [ $CHMOD_EXIT_CODE -ne 0 ]; then
    echo "设置可执行权限失败，chmod 退出码: $CHMOD_EXIT_CODE"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 停止现有服务
if [ -f "/etc/init.d/$SERVICE_NAME" ] && [ -x "/etc/init.d/$SERVICE_NAME" ]; then
    echo "停止现有服务..."
    /etc/init.d/$SERVICE_NAME stop
fi

# 安装二进制文件
echo "安装二进制文件..."
mv go-dns-proxy "$INSTALL_DIR/"
MV_EXIT_CODE=$?

if [ $MV_EXIT_CODE -ne 0 ]; then
    echo "移动文件失败，mv 退出码: $MV_EXIT_CODE"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 创建 OpenWrt 服务文件
echo "创建服务文件..."
curl -s -L -f "https://raw.githubusercontent.com/$GITHUB_REPO/main/scripts/openwrt-init.d" -o "/etc/init.d/$SERVICE_NAME"

if [ $? -ne 0 ]; then
    echo "下载服务文件失败"
    rm -rf "$TMP_DIR"
    exit 1
fi

# 设置权限
chmod +x "/etc/init.d/$SERVICE_NAME"

# 创建配置文件（如果不存在）
if [ ! -f "$CONFIG_DIR/$SERVICE_NAME" ]; then
    echo "创建默认配置文件..."
    cat > "$CONFIG_DIR/$SERVICE_NAME" << 'EOF'
config go-dns-proxy 'main'
    option enabled '1'
    option port '53'
    option china_server '114.114.114.114'
    option oversea_server '1.1.1.1'
    option beian_api_key ''
    option data_dir '/etc/go-dns-proxy/data'
    option admin_port '8080'
    option log_level 'info'
EOF
fi

# 清理临时文件
rm -rf "$TMP_DIR"

echo "安装完成！"
echo "配置文件位置: $CONFIG_DIR/$SERVICE_NAME"
echo ""
echo "请按以下步骤操作："
echo "1. 编辑配置文件：vi $CONFIG_DIR/$SERVICE_NAME"
echo "2. 启用开机自启：/etc/init.d/$SERVICE_NAME enable"
echo "3. 启动服务：/etc/init.d/$SERVICE_NAME start"
echo ""
echo "其他命令："
echo "停止服务：/etc/init.d/$SERVICE_NAME stop"
echo "重启服务：/etc/init.d/$SERVICE_NAME restart" 