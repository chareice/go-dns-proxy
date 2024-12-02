# Go DNS Proxy

一个支持国内外分流的 DNS 代理服务器。可以根据域名自动选择合适的上游 DNS 服务器，支持普通 DNS 和 DOH(DNS over HTTPS)。

## 特性

- 支持普通 DNS 和 DOH 服务器作为上游
- 支持根据域名后缀自动判断国内外分流（如 .cn, .中国 等）
- 支持根据备案信息判断国内外分流（需要 API Key）
- 支持 OpenWrt 自动安装和配置

## 安装和使用

### OpenWrt

1. 安装：

```bash
export url='https://raw.githubusercontent.com/chareice/go-dns-proxy/main/scripts' && sh -c "$(curl -kfsSl $url/openwrt-install.sh)"
```

2. 配置服务：

```bash
# 编辑配置文件
vi /etc/config/go-dns-proxy

# 或使用 UCI 命令配置
uci set go-dns-proxy.main.china_server='114.114.114.114'
uci set go-dns-proxy.main.oversea_server='8.8.8.8'
# 如果需要备案查询功能
uci set go-dns-proxy.main.beian_api_key='your_api_key'
uci commit go-dns-proxy
```

3. 启动服务：

```bash
# 启用开机自启
/etc/init.d/go-dns-proxy enable
# 启动服务
/etc/init.d/go-dns-proxy start
```

如需更新，只需再次运行安装命令即可。

### 手动安装

1. 从 [Releases](https://github.com/chareice/go-dns-proxy/releases) 页面下载对应架构的二进制文件
2. 解压并赋予执行权限
3. 运行程序：

```bash
./go-dns-proxy start --port 53 --chinaServer 114.114.114.114 --overSeaServer 1.1.1.1
```

## 配置说明

### OpenWrt 配置文件

配置文件位于 `/etc/config/go-dns-proxy`，格式如下：

```
config go-dns-proxy 'main'
    # 是否启用服务
    option enabled '1'

    # DNS 服务监听端口
    option port '53'

    # 国内 DNS 服务器地址
    # 支持普通 DNS（例如：114.114.114.114）
    # 或 DOH（例如：https://120.53.53.53/dns-query）
    option china_server '114.114.114.114'

    # 海外 DNS 服务器地址
    # 支持普通 DNS（例如：8.8.8.8）
    # 或 DOH（例如：https://1.1.1.1/dns-query）
    option oversea_server '1.1.1.1'

    # 备案查询 API Key（可选）
    # 如果设置了 API Key，将使用备案信息判断国内外分流
    option beian_api_key ''

    # 备案信息缓存文件路径
    option beian_cache_file '/etc/go-dns-proxy/beian_cache.json'

    # 备案信息缓存写入间隔（秒）
    option beian_cache_interval '10'
```

### 服务控制

```bash
# 启动服务
/etc/init.d/go-dns-proxy start

# 停止服务
/etc/init.d/go-dns-proxy stop

# 重启服务
/etc/init.d/go-dns-proxy restart

# 设置开机自启
/etc/init.d/go-dns-proxy enable

# 禁用开机自启
/etc/init.d/go-dns-proxy disable
```

### 诊断调试

如果遇到问题，可以运行以下命令测试上游 DNS 服务器的连通性：

```bash
export url='https://raw.githubusercontent.com/chareice/go-dns-proxy/main/scripts' && sh -c "$(curl -kfsSl $url/openwrt-debug.sh)"
```

诊断工具会：

1. 测试国内 DNS 服务器连通性（使用 www.baidu.com 作为测试域名）
2. 测试海外 DNS 服务器连通性（使用 www.google.com 作为测试域名）
3. 自动识别普通 DNS 和 DOH 服务器
4. 在连接失败时提供可能的原因

## 工作原理

1. 不使用备案 API Key 时：

   - 根据域名后缀判断是否为中国域名（如 .cn, .中国 等）
   - 如果是中国域名，使用国内 DNS 服务器
   - 如果不是中国域名，使用海外 DNS 服务器

2. 使用备案 API Key 时：
   - 首先根据域名后缀判断
   - 如果是通用域名（如 .com, .net 等），则查询备案信息
   - 根据备案信息决定使用哪个 DNS 服务器
   - 备案信息会被缓存以提高性能

## 注意事项

1. 如果使用 DOH 服务器，地址必须以 `https://` 开头
2. 备案查询功能需要单独申请 API Key
3. 修改配置后需要重启服务才能生效
4. 确保 DNS 服务端口（默认 53）没有被其他服务占用
