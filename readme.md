# 说明
> freepbx 安装和配置起来很麻烦，还各种bug。尤其是想要在arm板子上跑支持很少，于是自己做了个镜像自己用。
> 
> 纯 docker运行，支持arm64(rk3588上在跑)和x86

集成了如下类容：

+ asterisk 20
+ asterisk-chan-quectel 模块
+ freepbx 17
+ php8.2
+ acme.sh 自动生成证书脚本
+ postfix通过smtp发送邮件配置
+ fail2ban
+ 短信转发脚本
+ 4g语音转trunk。具体转发到哪个sip账号，在freepbx里面自行配置即可