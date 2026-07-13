# Lumen AI

::: warning 注意
流明集当前处于 Beta 阶段。请先使用测试媒体或已有可靠备份的资料库进行试用，不要将本应用作为重要媒体的唯一存储位置。
:::

::: info 文档正在完善
本页是功能文档骨架。Desktop 本地 Hub、Docker Host network、Lumen Host Broker、静态节点与故障排查的可执行步骤尚未发布。当前请仅依据[介绍中的连接边界](../introduction/lumen.md)评估部署方式；缺少明确配置时，不要假定节点已经安全可达。
:::

::: danger 仅限受信网络
不要将 Lumen Hub、Host Broker 或推理服务端口直接暴露到互联网或不可信网络。自动发现只能说明节点可见，不能代替网络访问控制或节点信任判断。
:::

后续内容将包括：

- Desktop 管理本地 Lumen Hub 的启用与状态检查。
- mDNS 自动发现的适用网络与防火墙要求。
- Linux Docker Host network、Docker Desktop Host Broker 与静态节点配置。
- 节点可见但推理请求不可达时的故障排查。
