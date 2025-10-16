#!/bin/bash
#set -e

AMP_MGR_USER=${AMP_MGR_USER:-admin}
AMP_MGR_SECRET=${AMP_MGR_SECRET:-amp111}


INSTALL_FLAG=${WEBROOT:-/var/www/html}/.installed

  ENV_FILE="/etc/freepbx.env"
  echo "Creating environment file for cron at ${ENV_FILE}"
  # 清空旧文件
  > ${ENV_FILE}
  # 将需要的环境变量写入文件
  echo "export DBENGINE='${DBENGINE}'" >> ${ENV_FILE}
  echo "export DBUSER='${DBUSER}'" >> ${ENV_FILE}
  echo "export DBPASS='${DBPASS}'" >> ${ENV_FILE}
  echo "export DBHOST='${DBHOST}'" >> ${ENV_FILE}
  echo "export DBPORT='${DBPORT}'" >> ${ENV_FILE}
  echo "export DBNAME='${DBNAME}'" >> ${ENV_FILE}
  echo "export CDRDBNAME='${CDRDBNAME}'" >> ${ENV_FILE}
  echo "export USER='${USER}'" >> ${ENV_FILE}
  echo "export GROUP='${GROUP}'" >> ${ENV_FILE}
  echo "export FQDN='${FQDN}'" >> ${ENV_FILE}
  echo "export TZ='${TZ}'" >> ${ENV_FILE}
  # 设置文件权限，保护密码
  chmod 600 ${ENV_FILE}

# 初始化 FreePBX
if [ ! -f "$INSTALL_FLAG" ]; then
  echo ">>> [FreePBX] First-time installation..."
  cd /usr/src/freepbx || exit 1
  ./start_asterisk start
   sleep 8
  ./install \
    --dbengine="${DBENGINE:-mysql}" \
    --dbname="${DBNAME:-asterisk}" \
    --dbhost="${DBHOST:-localhost}" \
    --dbport="${DBPORT:-3306}" \
    --cdrdbname="${CDRDBNAME:-asteriskcdrdb}" \
    --dbuser="${DBUSER:-freepbxuser}" \
    --dbpass="${DBPASS:-freepbxpass}" \
    --user="${USER:-asterisk}" \
    --group="${GROUP:-asterisk}" \
    --webroot="${WEBROOT:-/var/www/html}" \
    --astetcdir="${ASTETCDIR:-/etc/asterisk}" \
    --astmoddir="${ASTMODDIR:-/usr/lib64/asterisk/modules}" \
    --astvarlibdir="${ASTVARLIBDIR:-/var/lib/asterisk}" \
    --astagidir="${ASTAGIDIR:-/var/lib/asterisk/agi-bin}" \
    --astspooldir="${ASTSPOOLDIR:-/var/spool/asterisk}" \
    --astrundir="${ASTRUNDIR:-/var/run/asterisk}" \
    --astlogdir="${ASTLOGDIR:-/var/log/asterisk}" \
    --ampbin="${AMPBIN:-/var/lib/asterisk/bin}" \
    --ampsbin="${AMPSBIN:-/usr/sbin}" \
    --ampcgibin="${AMPCGIBIN:-/var/www/cgi-bin}" \
    --ampplayback="${AMPPLAYBACK:-/var/lib/asterisk/playback}" \
    -n
  # 链接 fwconsole
  if [ -f /var/lib/asterisk/bin/fwconsole ] && [ ! -f /usr/sbin/fwconsole ]; then
    ln -s /var/lib/asterisk/bin/fwconsole /usr/sbin/fwconsole
    ln -s /var/lib/asterisk/bin/amportal /usr/sbin/amportal
  fi

  if [ -f /var/lib/asterisk/bin/amportal ] && [ ! -f /usr/sbin/amportal ]; then
    ln -s /var/lib/asterisk/bin/amportal  /usr/sbin/amportal
  fi
  # 链接 freepbx.conf
  if [ -f /var/lib/asterisk/etc/freepbx.conf ] && [ ! -f /etc/freepbx.conf ]; then
    ln -s /var/lib/asterisk/etc/freepbx.conf /etc/freepbx.conf
  fi
  fwconsole setting ASTMODDIR ${ASTMODDIR:-/usr/lib64/asterisk/modules}
  fwconsole setting ASTAGIDIR ${ASTAGIDIR:-/var/lib/asterisk/agi-bin}
  fwconsole setting PHPTIMEZONE ${TZ:-Asia/Shanghai}
  fwconsole ma install pm2
  fwconsole ma installall
  fwconsole reload
  fwconsole ma refreshsignatures
  fwconsole ma downloadinstall soundlang
  fwconsole ma downloadinstall framework --force
  fwconsole reload
  fwconsole reload
  touch "$INSTALL_FLAG"
  mkdir /var/lib/asterisk/etc
  cp /etc/freepbx.conf /var/lib/asterisk/etc/
  chown -R asterisk:asterisk /var/lib/asterisk/etc
  chown -R asterisk:asterisk /var/www/html
  echo ">>> [FreePBX] Installation complete."
else
  echo ">>> [FreePBX] Existing installation detected, starting services..."
#  cd /usr/src/freepbx || exit 1
#  ./start_asterisk start
  # 链接 fwconsole
  if [ -f /var/lib/asterisk/bin/fwconsole ] && [ ! -f /usr/sbin/fwconsole ]; then
    ln -s /var/lib/asterisk/bin/fwconsole /usr/sbin/fwconsole
  fi

  if [ -f /var/lib/asterisk/bin/amportal ] && [ ! -f /usr/sbin/amportal ]; then
    ln -s /var/lib/asterisk/bin/amportal  /usr/sbin/amportal
  fi
  # 链接 freepbx.conf
  if [ -f /var/lib/asterisk/etc/freepbx.conf ] && [ ! -f /etc/freepbx.conf ]; then
    ln -s /var/lib/asterisk/etc/freepbx.conf /etc/freepbx.conf
  fi
  # 修复缺失的 /usr/share/asterisk/agi-bin/phpagi*.php 文件。
  #fwconsole ma downloadinstall framework --force
  fwconsole start
  fwconsole reload --no-interaction
fi
fwconsole chown

ACME_HOME="/data/acme"
ACME_SH="$ACME_HOME/acme.sh"
ACME_DNS_TYPE=${ACME_DNS_TYPE:-dns_cf}

mkdir -p $ACME_HOME
# -----------------------------------------------------
# 【新增/修改部分】: ACME.sh 证书管理
# -----------------------------------------------------

echo "--- 正在检查和管理 Let's Encrypt SSL 证书 ---"

if [ -n "$FQDN" ] && [ "$ACME_ENABLE" == "true" ]; then

    # 1. 检查 acme.sh 是否已安装到持久化目录
    if [ ! -f "$ACME_SH" ]; then
        echo "未检测到 acme.sh 安装。正在执行首次安装到持久化目录 $ACME_HOME ..."

        # 确保目录存在
        mkdir -p "$ACME_HOME"

        # 执行安装，将 home 目录指向挂载点
        curl https://get.acme.sh | sh -s install --home "$ACME_HOME" --nocron

        # 检查安装是否成功
        if [ ! -f "$ACME_SH" ]; then
            echo "致命错误：acme.sh 安装失败，请检查网络和权限！"
            # 尽管失败，仍然继续启动主进程
        fi
    else
        echo "acme.sh 已安装到 $ACME_HOME，跳过安装。"
    fi

    # 将 acme.sh 所在的目录添加到 PATH，方便后续调用
    export PATH="$ACME_HOME:$PATH"


    # 3. 颁发/续订证书 (使用 DNS 挑战，启用 Cloudflare API)
    # 每次容器启动时执行续订/颁发，acme.sh 会自动判断是否需要更新（小于60天）
    CERT_NAME="freepbx-${FQDN//./-}"

     # 首次颁发/续订证书 (为了容器启动后证书立即可用)
    echo "正在执行证书颁发/续订..."
    echo "记得去webui上面去点击 Delete Self-Signed CA 和 import Locally"
    "$ACME_SH" --issue -d "$FQDN" --dns "$ACME_DNS_TYPE" --keylength ec-256 \
        -m "$ACME_EMAIL" --home "$ACME_HOME" || true
    ACME_EXIT_CODE=$?
    if [ "$ACME_EXIT_CODE" -ne 0  ]; then
        echo "致命错误：acme.sh 证书颁发/续订失败！请检查 $ACME_DNS_TYPE 凭证和域名配置。"
    elif [ "$ACME_EXIT_CODE" -eq 2 ]; then
        # 退出码 2: 跳过续订（证书未到期）
        echo "acme.sh 续订跳过 (证书未到期)。无需导入，继续启动主应用..."
    else
        echo "acme.sh 证书操作成功。正在准备导入 FreePBX..."

        # 证书文件路径： acme.sh 默认的存放路径
        FULL_CHAIN="$ACME_HOME/$FQDN"_ecc/$FQDN.cer
        KEY_FILE="$ACME_HOME/$FQDN"_ecc/$FQDN.key

#        # 4. 导入和设置证书到 FreePBX (fwconsole)
#        echo "正在导入证书到 FreePBX Certificate Management..."
#        fwconsole certificate --import \
#            --chain "$FULL_CHAIN_FILE" \
#            --privkey "$KEY_FILE" \
#            --force
        echo "acme.sh 证书操作成功 (已颁发/续订)。正在准备导入 FreePBX..."

        # 证书文件路径 (在 /data/acme 下)
        CERT_DIR="$ACME_HOME/${FQDN}_ecc"
        SERVER_CERT_FILE="$CERT_DIR/$FQDN.cer"
        KEY_FILE="$CERT_DIR/$FQDN.key"
        CA_CHAIN_FILE="$CERT_DIR/ca.cer"
        FULL_CHAIN_FILE="$CERT_DIR/fullchain.cer"

        # 【新增】: FreePBX 期望的文件名和目录
        ASTERISK_KEYS_DIR="/etc/asterisk/keys"
        IMPORT_CERT_NAME="acme_auto_import"

        # 检查所有必需文件是否存在
        if [ ! -f "$SERVER_CERT_FILE" ] || [ ! -f "$KEY_FILE" ] || [ ! -f "$CA_CHAIN_FILE" ]; then
            echo "【严重错误】: acme.sh 成功执行，但必需的证书文件 ($SERVER_CERT_FILE, $KEY_FILE, $CA_CHAIN_FILE) 未找到！"
        else
            ASTERISK_KEYS_DIR="/etc/asterisk/keys"
            IMPORT_CERT_NAME="acme_auto_import"
            mkdir -p "$ASTERISK_KEYS_DIR"

            # 分别复制服务器证书、私钥和 CA 链文件
            echo "正在复制证书文件到 $ASTERISK_KEYS_DIR 以匹配 FreePBX 结构..."
#            cp "$SERVER_CERT_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"               # <-- 服务器证书
            cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"               # <-- 服务器证书
            cp "$KEY_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"                  # <-- 私钥
            cp "$CA_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME-ca-bundle.crt" # <-- CA 证书链 ()
#            cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME-fullchain.crt"   # <-- 完整的证书链

            # 【关键修正】: 确保所有文件的权限正确
            chown asterisk:asterisk "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME."*

            # 4. 导入和设置证书到 FreePBX (fwconsole)
            echo "正在导入证书到 FreePBX Certificate Management..."
            fwconsole certificate --import --force

            CERT_ID=$(fwconsole certificate --list | \
                grep -vE '^\+|\ ID\ |^\s*$' | \
                grep "$IMPORT_CERT_NAME" | \
                awk -F'|' '{print $2}' | \
                tr -d '[:space:]' | \
                sed 's/^0*//'
            )

            if [[ "$CERT_ID" =~ ^[0-9]+$ ]]; then
                echo "找到证书 ID: $CERT_ID。设置为默认证书并重载 FreePBX 配置..."
                fwconsole certificate --default="$CERT_ID"
                fwconsole reload
            else
                echo "【严重警告】: 证书导入成功，但无法自动获取证书 ID。提取结果: $CERT_ID。"
                echo "请手动在 Web 界面中将证书（名称: $IMPORT_CERT_NAME）设置为默认。"
            fi
        fi
    fi
    CRON_JOB="0 2 * * * . ${ENV_FILE}; /usr/local/bin/acme_renew_and_import.sh" # 每天凌晨2点执行
    echo "正在启动 Cron 服务并设置定时任务..."
    # 写入 Crontab 文件
    (crontab -l 2>/dev/null; echo "$CRON_JOB") | crontab -

    # 启动 Cron 服务到后台 (& 后台运行)
    # 不同的 Linux 发行版启动命令不同
    if command -v crond &> /dev/null; then
        crond &
    elif command -v cron &> /dev/null; then
        cron
    fi

    echo "Cron 定时任务已设置并启动。"
else
    echo "跳过 SSL 证书配置和 Cron 设置。"
fi

# ============================================================
# 【新增】: 在后台启动 Go 短信网关服务
# ============================================================
echo ">>> [Daemon] Starting SMS Gateway service in the background..."
# 检查二进制文件是否存在
if [ -f "/usr/local/bin/sms-gateway" ]; then
    # 使用 & 将进程放到后台运行
    # 将日志输出到 stdout/stderr，以便 docker logs 可以捕获
    /usr/local/bin/sms-gateway > /proc/1/fd/1 2>/proc/1/fd/2 &
    echo "SMS Gateway service started."
else
    echo "Warning: /usr/local/bin/sms-gateway not found, skipping startup."
fi

exec /usr/sbin/apachectl -DFOREGROUND
