## About
目前本脚本仅在`Ubuntu-22`的单节点环境中进行了测试，其他环境未测试，欢迎测试并反馈。

## Usages

1. 请尽量从一个干净的`Ubuntu-22`环境中使用本脚本
2. 首先，你需要运行一次`setup_node.sh`,以初始化环境，该操作将：
    - 检查和设置运行kubernetes所需的环境
    - 安装和配置containerd以及CNI
    - 安装kubeadm, kubelet and kubectl
    - 编译和安装firecracker-containerd，包括：firecracker-ctr，firecracker以及运行所需的rootfs等
    - 下载linux kernel，这里使用提供的支持faascale的内核
    - 配置firecracker-containerd
    - 创建devmapper
3. 之后你可以通过`start.sh`脚本，一键启动，你可以使用`start.sh containerd`或者`start.sh firecracker`。

   前者的操作包括：
    - 启动containerd服务
    - 初始化kubernetes控制平面节点
    - 安装Flannel网络插件
    - 安装MetalLB
    - 安装Knative环境

   后者的操作包括：
    - 启动containerd daemon
    - ***启动firecracker-containerd daemon***
    - ***构建、启动Puffer***
    - 初始化kubernetes控制平面节点
    - 安装Flannel网络插件
    - 安装MetalLB
    - 安装Knative环境

4. 如果你需要重新启动，则必须要首先执行`clean.sh`,再执行`start.sh`,或者直接执行`restart.sh`

## Notes

1. **在使用脚本之前，你必须要重新配置http_proxy**，我们在必要的位置使用了https代理以加速网络下载速度，你必须替换为你自己代理配置
   你可以使用`set_proxy.sh`删除代理，或者使用`set_proxy.sh https_proxy=http://{ip}:{port}` 将所有脚本中的代理替换为自己的
2. 在安装kubernetes时，我们安装了MetalLB以实现bare metal主机上的LoadBalancer,以提供External-IP，如果你在共有云平台上运行，这是不需要的
3. 我们在安装kubernetes时，使用了阿里云的镜像源：`registry.aliyuncs.com/google_containers`，以加速拉取所需的镜像，如果你不需要可手动删除此参数
4. 我们手动下载了安装kubernetes和knative过程中所需要的配置文件，并将其放置到了各自的`config`目录中，并对部分进行了修改，文件及其对应的链接和我们的修改如下：
    - k8s/config/flannel/kube-flannel.yml
        - download from https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
        - 将默认的`podCIDR(10.244.0.0/16)` 修改为 `192.168.0.0/16` 以和我们使用`kubeadm init`初始化控制平面节点时的配置保持一致
    - /k8s/config/metallb/metallb-native.yaml
        - download from https://raw.githubusercontent.com/metallb/metallb/v0.13.10/config/manifests/metallb-native.yaml
        - unchanged
    - knative/config/kourier.yaml
        - download from https://github.com/knative/net-kourier/releases/download/knative-v1.11.1/kourier.yaml
        - unchanged
    - knative/config/serving-core.yaml
        - download from https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-core.yaml
        - 将其中所有的容器镜像，替换为从我个人的镜像仓库拉取：`registry.cn-hangzhou.aliyuncs.com/kingdo_knative`
    - knative/config/serving-core.yaml
        - download from https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-crds.yaml
        - unchanged
    - knative/config/serving-core.yaml
        - download from https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-default-domain.yaml
        - 将其中所有的容器镜像，替换为从我个人的镜像仓库拉取：`registry.cn-hangzhou.aliyuncs.com/kingdo_knative`