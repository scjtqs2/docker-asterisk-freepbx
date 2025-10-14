FROM ubuntu:24.04
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive
ENV ASTERISK_VERSION=20
ENV FREEPBX_VERSION=17


# 使用 sed 替换源地址，同时支持 x86 和 arm64 架构
RUN sed -i 's#http://ports.ubuntu.com#http://mirrors.aliyun.com#g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's#http://archive.ubuntu.com#http://mirrors.aliyun.com#g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's#http://security.ubuntu.com#http://mirrors.aliyun.com#g' /etc/apt/sources.list.d/ubuntu.sources

# 添加php8.2的支持。
RUN apt update && \
    apt install -y software-properties-common lsb-release ca-certificates apt-transport-https && \
    add-apt-repository ppa:ondrej/php
# 预先配置时区和其他交互式包
RUN echo 'tzdata tzdata/Areas select Asia' | debconf-set-selections && \
    echo 'tzdata tzdata/Zones/Asia select Shanghai' | debconf-set-selections && \
    echo "postfix postfix/mailname string localhost" | debconf-set-selections && \
    echo "postfix postfix/main_mailer_type string Local only" | debconf-set-selections
# 设置环境变量
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai

# 安装依赖
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    asterisk asterisk-dev \
    apache2 mariadb-client unixodbc odbc-mariadb \
    php8.2 php8.2-cli php8.2-common libapache2-mod-php8.2 \
    php8.2-mysql php8.2-curl php8.2-gd php8.2-mbstring php8.2-xml \
    php8.2-zip php8.2-bcmath php8.2-intl php8.2-fileinfo php8.2-sockets \
    php8.2-sysvmsg php8.2-sysvsem php8.2-sysvshm php8.2-posix php8.2-calendar \
    php8.2-opcache php8.2-ldap php8.2-imap \
    wget git sudo curl sox mpg123 sqlite3 zip rsync \
    libtiff-dev libxml2-dev libncurses5-dev uuid-dev libjansson-dev ca-certificates net-tools \
    libsqlite3-dev libasound2-dev alsa-utils \
    fail2ban iptables iptables-persistent ipset iproute2 conntrack netfilter-persistent \
    nodejs npm vim cron libicu-dev pkgconf libbluetooth3 mailutils build-essential g++ \
    libx264-dev libvpx-dev \
    && update-alternatives --set php /usr/bin/php8.2 \
    && a2enmod php8.2 \
    && rm -rf /var/lib/apt/lists/*


RUN apt-get update && apt-get install -y \
    build-essential git autoconf automake libtool pkg-config \
    libtalloc-dev libsqlite3-dev libasound2-dev alsa-utils \
    asterisk asterisk-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# 拉取并编译 asterisk-chan-quectel
RUN git clone https://github.com/IchthysMaranatha/asterisk-chan-quectel.git && \
    cd asterisk-chan-quectel && \
    mkdir -p  /usr/lib64/asterisk && \
    if [ "$TARGETARCH" = "arm64" ]; then \
        DESTDIR="/usr/lib/aarch64-linux-gnu/asterisk/modules" && \
        ln -s /usr/lib/aarch64-linux-gnu/asterisk/modules /usr/lib64/asterisk/modules; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
        DESTDIR="/usr/lib/x86_64-linux-gnu/asterisk/modules" && \
        ln -s /usr/lib/x86_64-linux-gnu/asterisk/modules /usr/lib64/asterisk/modules; \
    else \
        echo "Unsupported architecture: $TARGETARCH"; \
        exit 1; \
    fi && \
    echo "Building for architecture: $TARGETARCH, DESTDIR: $DESTDIR" && \
    ./bootstrap && \
    ./configure --with-astversion=20 DESTDIR=$DESTDIR && \
    make && \
    make install

RUN \
  sed -i 's|#AST_USER|AST_USER|' /etc/default/asterisk && \
  sed -i 's|#AST_GROUP|AST_GROUP|' /etc/default/asterisk && \
  sed -i 's|;runuser|runuser|' /etc/asterisk/asterisk.conf && \
  sed -i 's|;rungroup|rungroup|' /etc/asterisk/asterisk.conf && \
  sed -i 's/\(^upload_max_filesize = \).*/\120M/' /etc/php/8.2/apache2/php.ini && \
  sed -i 's/\(^memory_limit = \).*/\1256M/' /etc/php/8.2/apache2/php.ini && \
  sed -i 's/^\(User\|Group\).*/\1 asterisk/' /etc/apache2/apache2.conf && \
  sed -i 's/AllowOverride None/AllowOverride All/' /etc/apache2/apache2.conf && \
  a2enmod rewrite proxy proxy_http proxy_wstunnel && \
  rm -rf /var/www/html/index.html /build/asterisk-chan-quectel


WORKDIR /usr/src


# ============================================================
# 安装 FreePBX
# ============================================================
# 下载并安装 FreePBX

RUN \
  wget -O /usr/src/freepbx-17.0-latest-EDGE.tgz http://mirror.freepbx.org/modules/packages/freepbx/freepbx-17.0-latest-EDGE.tgz && \
  tar zxvf /usr/src/freepbx-17.0-latest-EDGE.tgz -C /usr/src && \
  rm /usr/src/freepbx-17.0-latest-EDGE.tgz && \
  apt-get clean

# 配置npm国内镜像源
RUN npm config set registry https://registry.npmmirror.com

USER asterisk
RUN npm config set registry https://registry.npmmirror.com
USER root

# ============================================================
# 配置基础文件
# ============================================================
COPY extensions_custom.conf /etc/asterisk/extensions_custom.conf
COPY quectel.conf /etc/asterisk/quectel.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY run-httpd.sh /run-httpd.sh
COPY forward_sms.php /usr/local/bin/forward_sms.php
COPY pjsip.transports_custom.conf /etc/asterisk/pjsip.transports_custom.conf
COPY 000-default.conf /etc/apache2/sites-enabled/000-default.conf
COPY gai.conf /etc/gai.conf
# 添加 fail2ban 配置文件
COPY fail2ban/jail.local /etc/fail2ban/jail.local
COPY fail2ban/asterisk.conf /etc/fail2ban/filter.d/asterisk.conf
COPY fail2ban/check-fail2ban.sh /usr/src/check-fail2ban.sh
#postfix
COPY postfix/main.cf.template /etc/postfix/main.cf.template
COPY acme_renew_and_import.sh /usr/local/bin/
#COPY update-ip-db.sh /usr/local/bin/update-ip.sh
# 为 Postfix chroot jail 创建 etc 目录
RUN mkdir -p /var/spool/postfix/etc

RUN chmod +x /docker-entrypoint.sh && \
    chmod +x /run-httpd.sh && \
    chmod +x /usr/local/bin/forward_sms.php && \
    chmod +x /usr/local/bin/acme_renew_and_import.sh && \
    chown -R asterisk:asterisk /var/lib/asterisk /etc/asterisk /var/spool/asterisk /var/log/asterisk /usr/local/bin/forward_sms.php /etc/apache2/sites-enabled/000-default.conf


RUN chown -R asterisk:asterisk /var/lib/asterisk  /etc/asterisk && \
    cp -a /etc/asterisk /usr/src/asterisk-etc && \
    cp -a /var/lib/asterisk /usr/src/asterisk-lib && \
    usermod -aG audio asterisk && \
    usermod -aG dialout asterisk


ENV AUDIO_PORT=/dev/ttyUSB1
ENV AT_PORT=/dev/ttyUSB2
ENV PHONE_ID=SMS1_1234567890
ENV FORWARD_URL=http://forwardsms:8080/api/v1/sms/receive
ENV CALL_FORWARD_URL=http://forwardsms:8080/api/v1/call/receive
ENV WEBROOT=/var/www/html
ENV ASTETCDIR=/etc/asterisk
ENV ASTMODDIR=/usr/lib64/asterisk/modules
ENV ASTVARLIBDIR=/var/lib/asterisk
ENV ASTAGIDIR=/var/lib/asterisk/agi-bin
ENV ASTSPOOLDIR=/var/spool/asterisk
ENV ASTRUNDIR=/var/run/asterisk
ENV AMPBIN=/var/lib/asterisk/bin
ENV AMPSBIN=/usr/sbin
ENV AMPCGIBIN=/var/www/cgi-bin
ENV AMPPLAYBACK=/var/lib/asterisk/playback
ENV FAIL2BAN_ENABLE=true

EXPOSE 80 443 5060 8088 8089

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/run-httpd.sh"]
