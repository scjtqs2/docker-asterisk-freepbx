#!/bin/bash
#set -e

AMP_MGR_USER=${AMP_MGR_USER:-admin}
AMP_MGR_SECRET=${AMP_MGR_SECRET:-amp111}


INSTALL_FLAG=${WEBROOT:-/var/www/html}/.installed

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
        FULL_CHAIN_FILE="$ACME_HOME/$FQDN"_ecc/fullchain.cer
        KEY_FILE="$ACME_HOME/$FQDN"_ecc/$FQDN.key

        # 【新增】: FreePBX 期望的文件名和目录
        ASTERISK_KEYS_DIR="/etc/asterisk/keys"
        IMPORT_CERT_NAME="acme_auto_import"

        # 确保 keys 目录存在
        mkdir -p "$ASTERISK_KEYS_DIR"

        # 【关键步骤】：将证书文件复制到 FreePBX 期望的导入目录，并重命名
        echo "正在复制证书文件到 $ASTERISK_KEYS_DIR..."
        cp "$FULL_CHAIN_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt"
        cp "$KEY_FILE" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"

        # 确保文件权限正确 (asterisk 用户需要访问)
        chown asterisk:asterisk "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.crt" "$ASTERISK_KEYS_DIR/$IMPORT_CERT_NAME.key"

        # 4. 导入和设置证书到 FreePBX (fwconsole)
        echo "正在导入证书到 FreePBX Certificate Management..."
        # 【修正】: 直接执行 --import，让 FreePBX 扫描目录
        fwconsole certificate --import --force

        # ... (后续的查找 CERT_ID 逻辑不变) ...
        # FreePBX 导入成功后，分配的证书名称通常是复制的文件名 'acme_auto_import'
        # 查找证书 ID 并设置为默认
        CERT_ID=$(fwconsole certificate --list | \
            grep -vE '^\+|\ ID\ |^\s*$' | \
            grep "$IMPORT_CERT_NAME" | \
            awk -F'|' '{print $2}' | \
            tr -d '[:space:]' | \
            sed 's/^0*//'  # <-- 【关键修正】: 移除前导零
        )

        # 确保提取的 ID 是有效的数字
        if [[ "$CERT_ID" =~ ^[0-9]+$ ]]; then
            echo "找到证书 ID: $CERT_ID。设置为默认证书并重载 FreePBX 配置..."

            # 证书 ID 已经确认是可靠的数字，执行设置默认证书
            fwconsole certificate --default="$CERT_ID"
            fwconsole reload
        else
            # 打印提取失败原因，并提供手动指导
            echo "【严重警告】: 证书导入成功，但无法自动获取证书 ID。提取结果: $CERT_ID。"
            echo "请手动在 Web 界面中将证书（名称: $IMPORT_CERT_NAME）设置为默认。"
        fi
    fi
    CRON_JOB="0 2 * * * /usr/local/bin/acme_renew_and_import.sh" # 每天凌晨2点执行
    echo "正在启动 Cron 服务并设置定时任务..."
    # 写入 Crontab 文件
    (crontab -l 2>/dev/null; echo "$CRON_JOB") | crontab -

    # 启动 Cron 服务到后台 (& 后台运行)
    # 不同的 Linux 发行版启动命令不同
    if command -v crond &> /dev/null; then
        crond &
    elif command -v cron &> /dev/null; then
        service cron start
    fi

    echo "Cron 定时任务已设置并启动。"
else
    echo "跳过 SSL 证书配置和 Cron 设置。"
fi

exec /usr/sbin/apachectl -DFOREGROUND
