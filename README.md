# 运维平台k8s资源推送 

**提供容器登录、日志、资源推送等功能**

## 配置
vim /etc/profile  
>export GO111MODULE=on  
>GOPROXY=https://goproxy.io  
>export GOPROXY  

source /etc/profile  

## 运行
git clone http://gitlab.ccfox.com/ops/ops-push.git  
cd ops-push   
make   
make install   
cd /opt/ops-push/   
./ops-push  




