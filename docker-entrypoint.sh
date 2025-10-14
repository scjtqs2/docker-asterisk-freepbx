#!/bin/bash

set -e

ETC_INSTALL_FLAG=${ASTETCDIR:-/etc/asterisk}/.installed
LIB_INSTALL_FLAG=${ASTVARLIBDIR:-/var/lib/asterisk}/.installed
chown -R asterisk:asterisk /var/lib/asterisk /etc/asterisk /var/spool/asterisk /var/log/asterisk

# 同步 Asterisk 配置目录
if [ ! -f "$ETC_INSTALL_FLAG" ]; then
    echo "初始化 Asterisk 配置文件..."
    rsync -a /usr/src/asterisk-etc/ ${ASTETCDIR:-/etc/asterisk}/
    chown -R asterisk:asterisk ${ASTETCDIR:-/etc/asterisk}
    touch "$ETC_INSTALL_FLAG"
    echo "Asterisk 配置文件初始化完成"
fi

# 同步 Asterisk 库目录
if [ ! -f "$LIB_INSTALL_FLAG" ]; then
    echo "初始化 Asterisk 库文件..."
    rsync -a /usr/src/asterisk-lib/ ${ASTVARLIBDIR:-/var/lib/asterisk}/
    chown -R asterisk:asterisk ${ASTVARLIBDIR:-/var/lib/asterisk}
    touch "$LIB_INSTALL_FLAG"
    echo "Asterisk 库文件初始化完成"
fi


# 替换 USB 端口配置
if [ -n "$AT_PORT" ]; then
    echo "配置 AT 端口: $AT_PORT"
    sed -i "s|%AT_PORT%|$AT_PORT|g" /etc/asterisk/quectel.conf
else
    echo "警告: AT_PORT 环境变量未设置，使用默认配置"
fi

if [ -n "$AUDIO_PORT" ]; then
   echo "配置 语音端口:$AUDIO_PORT"
   sed -i "s|%AUDIO_PORT%|$AUDIO_PORT|g" /etc/asterisk/quectel.conf
else
    echo "警告: AUDIO_PORT 环境变量未设置，使用默认配置"
fi

if [ -n "$FORWARD_SECRET" ]; then
   echo "配置 FORWARD_SECRET:$FORWARD_SECRET"
   sed -i "s|%FORWARD_SECRET%|$FORWARD_SECRET|g" /etc/asterisk/extensions_custom.conf
else
    echo "警告: FORWARD_SECRET 环境变量未设置，使用默认配置"
fi

if [ -n "$FORWARD_URL" ]; then
   echo "配置 FORWARD_URL:$FORWARD_URL"
   sed -i "s|%FORWARD_URL%|$FORWARD_URL|g" /etc/asterisk/extensions_custom.conf
else
    echo "警告: FORWARD_URL 环境变量未设置，使用默认配置"
fi

if [ -n "$CALL_FORWARD_URL" ]; then
   echo "配置 CALL_FORWARD_URL:$CALL_FORWARD_URL"
   sed -i "s|%CALL_FORWARD_URL%|$CALL_FORWARD_URL|g" /etc/asterisk/extensions_custom.conf
else
    echo "警告: CALL_FORWARD_URL 环境变量未设置，使用默认配置"
fi

if [ -n "$PHONE_ID" ]; then
   echo "配置 PHONE_ID:$PHONE_ID"
   sed -i "s|%PHONE_ID%|$PHONE_ID|g" /etc/asterisk/extensions_custom.conf
else
    echo "警告: PHONE_ID 环境变量未设置，使用默认配置"
fi


# 创建必要的目录
mkdir -p /data/log /data/db /var/lib/asterisk/agi-bin /var/log/asterisk/cdr-csv

chown -R asterisk /data /var/lib/asterisk/agi-bin /var/log/asterisk/cdr-csv

# 检查配置文件是否存在
if [ ! -f "/etc/asterisk/quectel.conf" ]; then
    echo "错误: /etc/asterisk/quectel.conf 配置文件不存在"
    exit 1
fi

chown -R asterisk /var/www/

# 显示配置信息
echo "=== asterisk 配置信息 ==="
echo "AT 端口: ${AT_PORT:-未设置}"
echo "语言端口 : ${AUDIO_PORT:-未设置}"
echo "=========================="

chmod 666 /dev/ttyUSB* 2>/dev/null || true
chmod -R 666 /dev/snd/* 2>/dev/null || true
#chmod -R 666 /proc/asound 2>/dev/null || true
sudo usermod -a -G dialout asterisk  2>/dev/null || true
sudo usermod -a -G audio asterisk  2>/dev/null || true
#Fail2Ban
if [ "$FAIL2BAN_ENABLE" == "true" ]; then
  echo "Enabling fail2ban for asterisk logs at /var/log/asterisk/full"
  echo "Starting fail2ban server"
  rm -f /var/run/fail2ban/fail2ban.sock
  fail2ban-client start &
fi
# 启动计划任务服务
cron

echo "--- 正在应用SMTP配置 ---"

# 1. 动态生成 main.cf
# 替换 relayhost 占位符。环境变量应由 docker run 或 docker-compose 传入
# 环境变量示例: SMTP_SERVER="smtp.yourserver.com:587"
if [ -n "$SMTP_SERVER" ]; then
    # 1. 动态生成 main.cf
    # SMTP_SERVER 变量必须包含 [hostname]:port 格式，例如: [smtp.ym.163.com]:587

    # 从模板复制到正式文件
    cp /etc/postfix/main.cf.template /etc/postfix/main.cf

    # 检查 SMTP_SERVER 是否包含端口，如果没有，Postfix 默认使用 25。
    # 我们假设用户提供的 SMTP_SERVER 已经是 [HOST]:PORT 的格式。

    # 使用 sed 替换 relayhost 占位符。
    # 注意：我们假设用户输入是 HOSTNAME 或 [HOSTNAME]:PORT
    # Postfix 推荐的格式是 [HOST]:PORT，我们将直接使用变量进行替换
    RELAY_HOST_CONFIG="[$SMTP_SERVER]"  # 增加括号，确保 Postfix 正确处理

    # 智能处理 SMTP_SERVER 变量，分离主机和端口
    # 正确格式为: [hostname]:port
    if [[ "$SMTP_SERVER" == *:* ]]; then
        # 如果变量包含端口 (例如 smtp.example.com:587)
        SMTP_HOST=${SMTP_SERVER%%:*}  # 提取冒号前的主机部分
        SMTP_PORT=${SMTP_SERVER##*:}  # 提取冒号后的端口部分
        RELAY_HOST_CONFIG="[${SMTP_HOST}]:${SMTP_PORT}"
    else
        # 如果变量不包含端口 (例如 smtp.example.com)
        SMTP_HOST=$SMTP_SERVER
        RELAY_HOST_CONFIG="[${SMTP_HOST}]" # Postfix 将默认使用 25 端口
    fi

    echo "生成的 Postfix relayhost 配置: $RELAY_HOST_CONFIG"

    # 替换 main.cf 中的占位符
    sed -i "s/SMTP_HOST_PORT_PLACEHOLDER/$RELAY_HOST_CONFIG/g" /etc/postfix/main.cf

    if [ "$SMTP_SSL" = "true" ]; then
        echo "检测到 SMTP_SSL=true，动态启用 smtp_tls_wrappermode (SMTPS 模式)。"
        echo "smtp_tls_wrappermode = yes" >> /etc/postfix/main.cf
        # 将加密级别从 'may' (可选) 提升为 'encrypt' (强制)
        echo "  -> 强制加密 smtp_tls_security_level = encrypt"
        sed -i 's/^[[:space:]]*smtp_tls_security_level[[:space:]]*=[[:space:]]*may/smtp_tls_security_level = encrypt/' /etc/postfix/main.cf
    else
        echo "未设置 SMTP_SSL=true，将使用标准的 STARTTLS 模式。"
    fi

    # 2. 动态生成 sasl_passwd
    # 环境变量示例: SMTP_USERNAME="user@domain.com", SMTP_PASSWORD="yourpassword"
    if [ -n "$SMTP_USERNAME" ] && [ -n "$SMTP_PASSWORD" ]; then
        # 配置认证文件
        echo "正在配置 SMTP 认证..."
        AUTH_STRING="${RELAY_HOST_CONFIG} ${SMTP_USERNAME}:${SMTP_PASSWORD}"
        echo "$AUTH_STRING" > /etc/postfix/sasl_passwd

        # 3. 配置 Postfix
        echo "正在设置 sasl_passwd 权限和生成哈希文件..."
        chmod 600 /etc/postfix/sasl_passwd
        postmap /etc/postfix/sasl_passwd
        # 【新增】: 动态生成发件人地址重写规则
        echo "正在配置发件人地址重写..."
        echo "/.+/    ${SMTP_USERNAME}" > /etc/postfix/sender_canonical
        #postmap /etc/postfix/sender_canonical
        echo "发件人地址将全部重写为: ${SMTP_USERNAME}"
        echo "Postfix 配置完成。"
        echo "正在启动 Postfix 邮件传输代理..."
        echo "正在为 Postfix chroot 环境同步 DNS 配置..."
        cp /etc/resolv.conf /var/spool/postfix/etc/resolv.conf
        cp /etc/hosts /var/spool/postfix/etc/hosts
        cp /etc/nsswitch.conf /var/spool/postfix/etc/nsswitch.conf
        echo "DNS 配置同步完成。"
        /usr/sbin/postfix start
        # 检查服务是否成功启动
        if [ $? -eq 0 ]; then
            echo "Postfix 启动成功。"
        else
            echo "警告: Postfix 启动失败！邮件发送可能无法工作。"
        fi
    else
        echo "警告: 找到了 \$SMTP_SERVER，但缺少 \$SMTP_USERNAME 或 \$SMTP_PASSWORD。无法配置认证。"
    fi
else
    echo "未设置 \$SMTP_SERVER 环境变量，跳过 SMTP 配置。"
    # 确保 main.cf 仍然是可用的默认版本（如果需要）
    # 如果没有SMTP，通常会使用默认的本地配置，您可能需要在此处处理它。
fi

# ====================================================================
#  1. 动态检测CPU架构并设置ODBC驱动路径
# ====================================================================
echo "正在检测系统架构以配置ODBC驱动..."

# 使用 uname -m 在运行时检测，比构建时的 ARG 更可靠
ARCH=$(uname -m)
DRIVER_PATH=""

case "$ARCH" in
    "x86_64")
        echo "检测到 x86_64 (amd64) 架构。"
        DRIVER_PATH="/usr/lib/x86_64-linux-gnu/odbc/libmaodbc.so"
        ;;
    "aarch64")
        echo "检测到 aarch64 (arm64) 架构。"
        DRIVER_PATH="/usr/lib/aarch64-linux-gnu/odbc/libmaodbc.so"
        ;;
    *)
        echo "错误：不支持的架构 '$ARCH'！无法配置ODBC驱动。"
        exit 1
        ;;
esac

# 检查驱动文件是否存在，增加脚本的健壮性
if [ ! -f "$DRIVER_PATH" ]; then
    echo "致命错误：ODBC驱动文件在路径 '$DRIVER_PATH' 未找到！"
    echo "请确认 'odbc-mariadb' 包已在 Dockerfile 中正确安装。"
    exit 1
fi

echo "ODBC 驱动路径已确定为: $DRIVER_PATH"


# ====================================================================
#  2. 动态生成 /etc/odbcinst.ini
# ====================================================================
echo "正在生成 /etc/odbcinst.ini 配置文件..."
cat <<EOF > /etc/odbcinst.ini
[MySQL]
Description = ODBC for MySQL (MariaDB)
Driver      = ${DRIVER_PATH}
FileUsage   = 1
EOF


# ====================================================================
#  3. 动态生成 /etc/odbc.ini
# ====================================================================
echo "正在生成 /etc/odbc.ini 配置文件..."
# 使用环境变量，并提供合理的默认值以防变量未设置
cat <<EOF > /etc/odbc.ini
[MySQL-asteriskcdrdb]
Description = MySQL connection to '${CDRDBNAME:-asteriskcdrdb}' database
Driver      = MySQL
Server      = ${DBHOST:-mariadb}
User        = ${DBUSER:-freepbxuser}
Password    = ${DBPASS:-freepbxpass}
Database    = ${CDRDBNAME:-asteriskcdrdb}
Port        = ${DBPORT:-3306}
EOF

# ====================================================================
#  3. 【新增】动态配置 Apache 端口
# ====================================================================
echo "--- [Entrypoint] 正在配置 Apache 端口 ---"

# 如果环境变量未设置，则使用标准的默认端口
HTTP_PORT=${APACHE_HTTP_PORT:-80}
HTTPS_PORT=${APACHE_HTTPS_PORT:-443}

echo "设置 Apache HTTP 监听端口为: ${HTTP_PORT}"
echo "设置 Apache HTTPS 监听端口为: ${HTTPS_PORT}"

# 定义 Apache 配置文件路径
PORTS_CONF="/etc/apache2/ports.conf"
VHOST_CONF="/etc/apache2/sites-enabled/000-default.conf"
# 注意：您的 Dockerfile 中没有 SSL 的 VirtualHost，但我们预留逻辑
SSL_VHOST_CONF="/etc/apache2/sites-available/default-ssl.conf"

# 修改主端口配置文件 ports.conf
# 使用 sed 将 "Listen 80" 替换为新的 HTTP 端口
sed -i "s/Listen 80/Listen ${HTTP_PORT}/" "${PORTS_CONF}"
# 使用 sed 将 "Listen 443" 替换为新的 HTTPS 端口
sed -i "s/Listen 443/Listen ${HTTPS_PORT}/" "${PORTS_CONF}"

# 修改 HTTP 虚拟主机配置文件 000-default.conf
# 使用 sed 将 "<VirtualHost *:80>" 替换为新的 HTTP 端口
sed -i "s/<VirtualHost \*:[0-9]*>/<VirtualHost *:${HTTP_PORT}>/" "${VHOST_CONF}"

# (可选，增强健壮性) 检查并修改默认的 SSL 虚拟主机文件（如果存在）
if [ -f "${SSL_VHOST_CONF}" ]; then
    echo "检测到 SSL 虚拟主机配置文件，正在修改..."
    sed -i "s/<VirtualHost _default_:[0-9]*>/<VirtualHost _default_:${HTTPS_PORT}>/" "${SSL_VHOST_CONF}"
fi

# 执行传入的命令
exec "$@"
