import { DefaultTheme } from 'vitepress'

// 中文主文档：任务与症状优先；全部 canonical 页面必须在此可达。
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        { text: '文档首页', link: '/zh-cn/user-manual/' },
        {
            text: '开始使用',
            collapsed: false,
            items: [
                { text: '开始使用', link: '/zh-cn/user-manual/getting-started/' },
                {
                    text: '了解与选择',
                    collapsed: false,
                    items: [
                        { text: '认识流明集', link: '/zh-cn/user-manual/getting-started/about' },
                        { text: '选择运行方式', link: '/zh-cn/user-manual/getting-started/choose-runtime' },
                        { text: '系统要求与支持范围', link: '/zh-cn/user-manual/getting-started/system-requirements' },
                    ]
                },
                {
                    text: '安装',
                    collapsed: true,
                    items: [
                        { text: '安装 Desktop', link: '/zh-cn/user-manual/getting-started/install-desktop' },
                        { text: '安装 Docker Server', link: '/zh-cn/user-manual/getting-started/install-docker' },
                        { text: '卸载与保留数据', link: '/zh-cn/user-manual/getting-started/uninstall' },
                    ]
                },
                {
                    text: '完成首次成功',
                    collapsed: true,
                    items: [
                        { text: '第一次设置', link: '/zh-cn/user-manual/getting-started/first-setup' },
                        { text: '创建第一位管理员', link: '/zh-cn/user-manual/getting-started/create-admin' },
                        { text: '保护管理员账户', link: '/zh-cn/user-manual/getting-started/secure-account' },
                        { text: '创建主资源库', link: '/zh-cn/user-manual/getting-started/primary-repository' },
                        { text: '导入第一批媒体', link: '/zh-cn/user-manual/getting-started/import-first-media' },
                        { text: '验收首次运行', link: '/zh-cn/user-manual/getting-started/verify-first-run' },
                        { text: '创建第一份完整备份', link: '/zh-cn/user-manual/getting-started/first-backup' },
                        { text: '接下来可以做什么', link: '/zh-cn/user-manual/getting-started/next-steps' },
                    ]
                },
            ]
        },
        {
            text: '使用流明集',
            collapsed: true,
            items: [
                { text: '使用流明集', link: '/zh-cn/user-manual/use/' },
                { text: '首页与统计', link: '/zh-cn/user-manual/use/home' },
                {
                    text: '浏览与搜索',
                    collapsed: false,
                    items: [
                        { text: '浏览资源库', link: '/zh-cn/user-manual/use/browse' },
                        { text: '筛选、排序与批量操作', link: '/zh-cn/user-manual/use/advanced-browse' },
                        { text: '媒体信息与组件', link: '/zh-cn/user-manual/use/media-details' },
                        { text: '搜索媒体', link: '/zh-cn/user-manual/use/search' },
                        { text: '语义搜索', link: '/zh-cn/user-manual/use/semantic-search' },
                        { text: 'OCR 文字搜索', link: '/zh-cn/user-manual/use/ocr-search' },
                    ]
                },
                {
                    text: '添加媒体',
                    collapsed: true,
                    items: [
                        { text: '添加媒体：上传、扫描或云导入', link: '/zh-cn/user-manual/use/add-media' },
                        { text: '上传媒体', link: '/zh-cn/user-manual/use/upload' },
                        { text: '扫描已有目录', link: '/zh-cn/user-manual/use/scan' },
                        { text: '从 iCloud 导入', link: '/zh-cn/user-manual/use/cloud-import' },
                    ]
                },
                {
                    text: '整理媒体',
                    collapsed: true,
                    items: [
                        { text: '组织媒体', link: '/zh-cn/user-manual/use/organize' },
                        { text: '相册与堆叠', link: '/zh-cn/user-manual/use/albums' },
                        { text: '人物', link: '/zh-cn/user-manual/use/people' },
                        { text: '地点与地图', link: '/zh-cn/user-manual/use/places' },
                        { text: '事件', link: '/zh-cn/user-manual/use/events' },
                        { text: '重复项与回收站', link: '/zh-cn/user-manual/use/duplicates-trash' },
                    ]
                },
                {
                    text: '编辑与分享',
                    collapsed: true,
                    items: [
                        { text: '工作室与非破坏性编辑', link: '/zh-cn/user-manual/use/studio' },
                        { text: '导出与下载', link: '/zh-cn/user-manual/use/export' },
                        { text: '创建与管理分享链接', link: '/zh-cn/user-manual/use/sharing' },
                        { text: '分享接收者指南', link: '/zh-cn/user-manual/use/share-recipient' },
                    ]
                },
                { text: '账户、用户与偏好', link: '/zh-cn/user-manual/use/account' },
                {
                    text: 'Lumilio Agent',
                    collapsed: true,
                    items: [
                        { text: 'Lumilio Agent', link: '/zh-cn/user-manual/use/agent' },
                        { text: '配置 Lumilio Agent 提供方', link: '/zh-cn/user-manual/use/agent-provider' },
                        { text: 'Lumilio Agent 的权限与确认', link: '/zh-cn/user-manual/use/agent-permissions' },
                        { text: 'Lumilio Agent 的隐私边界', link: '/zh-cn/user-manual/use/agent-privacy' },
                    ]
                },
                {
                    text: 'Lumen Intelligence',
                    collapsed: true,
                    items: [
                        { text: 'Lumen Intelligence', link: '/zh-cn/user-manual/use/lumen-intelligence' },
                        { text: '连接或运行 Lumen Intelligence 节点', link: '/zh-cn/user-manual/use/lumen-node' },
                        { text: '重建智能索引', link: '/zh-cn/user-manual/use/rebuild-ai-indexes' },
                    ]
                },
            ]
        },
        {
            text: '解决问题',
            collapsed: true,
            items: [
                { text: '解决问题：从症状开始', link: '/zh-cn/user-manual/troubleshooting/' },
                {
                    text: '安装、启动与登录',
                    collapsed: false,
                    items: [
                        { text: '安装失败', link: '/zh-cn/user-manual/troubleshooting/installation' },
                        { text: 'Desktop 无法启动 Server', link: '/zh-cn/user-manual/troubleshooting/desktop-runtime' },
                        { text: '浏览器无法打开流明集', link: '/zh-cn/user-manual/troubleshooting/cannot-open' },
                        { text: '无法登录或丢失 MFA', link: '/zh-cn/user-manual/troubleshooting/login-mfa' },
                        { text: '通行密钥不可用', link: '/zh-cn/user-manual/troubleshooting/passkey' },
                    ]
                },
                {
                    text: '存储与导入',
                    collapsed: true,
                    items: [
                        { text: '资源库或磁盘显示离线', link: '/zh-cn/user-manual/troubleshooting/repository-offline' },
                        { text: '上传失败', link: '/zh-cn/user-manual/troubleshooting/upload-failed' },
                        { text: '扫描不到媒体', link: '/zh-cn/user-manual/troubleshooting/scan-no-media' },
                    ]
                },
                {
                    text: '处理与播放',
                    collapsed: true,
                    items: [
                        { text: '媒体一直显示处理中', link: '/zh-cn/user-manual/troubleshooting/processing-stuck' },
                        { text: '缩略图缺失或媒体无法播放', link: '/zh-cn/user-manual/troubleshooting/thumbnails-playback' },
                        { text: '队列长期不前进', link: '/zh-cn/user-manual/troubleshooting/queue-stuck' },
                        { text: '失败数量持续增加', link: '/zh-cn/user-manual/troubleshooting/failures-growing' },
                    ]
                },
                {
                    text: '搜索与智能能力',
                    collapsed: true,
                    items: [
                        { text: '搜索结果缺失', link: '/zh-cn/user-manual/troubleshooting/search-missing' },
                        { text: '人物、OCR 或语义没有结果', link: '/zh-cn/user-manual/troubleshooting/ai-no-results' },
                        { text: 'Lumen Intelligence 节点不可用', link: '/zh-cn/user-manual/troubleshooting/lumen-unavailable' },
                        { text: 'Lumilio Agent 不可用', link: '/zh-cn/user-manual/troubleshooting/agent-unavailable' },
                    ]
                },
                {
                    text: '分享、升级与恢复',
                    collapsed: true,
                    items: [
                        { text: '分享链接无法打开', link: '/zh-cn/user-manual/troubleshooting/share-unavailable' },
                        { text: '升级后无法启动', link: '/zh-cn/user-manual/troubleshooting/upgrade-startup' },
                        { text: '数据库恢复期间页面断开', link: '/zh-cn/user-manual/troubleshooting/restore-disconnect' },
                    ]
                },
                {
                    text: '平台、性能与支持',
                    collapsed: true,
                    items: [
                        { text: '磁盘空间不足或性能下降', link: '/zh-cn/user-manual/troubleshooting/disk-performance' },
                        { text: '按平台补充排查', link: '/zh-cn/user-manual/troubleshooting/platforms' },
                        { text: '收集诊断信息', link: '/zh-cn/user-manual/troubleshooting/collect-diagnostics' },
                        { text: '当前已知问题与行为差异', link: '/zh-cn/user-manual/troubleshooting/known-issues' },
                        { text: '获得帮助与提交 Issue', link: '/zh-cn/user-manual/troubleshooting/get-help' },
                    ]
                },
            ]
        },
        {
            text: '管理与安全',
            collapsed: true,
            items: [
                { text: '管理与安全', link: '/zh-cn/user-manual/admin/' },
                { text: '部署决策', link: '/zh-cn/user-manual/admin/deployment' },
                {
                    text: '存储与资源库',
                    collapsed: false,
                    items: [
                        { text: '存储位置与资源库', link: '/zh-cn/user-manual/admin/storage-and-repositories' },
                        { text: '管理存储位置', link: '/zh-cn/user-manual/admin/storage-locations' },
                        { text: '资源库目录与布局策略', link: '/zh-cn/user-manual/admin/repository-layout' },
                        { text: '外置磁盘与网络存储', link: '/zh-cn/user-manual/admin/external-storage' },
                        { text: '移动、复制与身份冲突', link: '/zh-cn/user-manual/admin/identity-conflicts' },
                    ]
                },
                {
                    text: '账户与网络',
                    collapsed: true,
                    items: [
                        { text: '用户与角色', link: '/zh-cn/user-manual/admin/users-and-roles' },
                        { text: '公开注册与访问边界', link: '/zh-cn/user-manual/admin/registration-exposure' },
                        { text: '登录安全与应急恢复', link: '/zh-cn/user-manual/admin/authentication' },
                        { text: 'HTTPS 与网络访问', link: '/zh-cn/user-manual/admin/https' },
                    ]
                },
                {
                    text: '备份与恢复',
                    collapsed: true,
                    items: [
                        { text: '备份与恢复总览', link: '/zh-cn/user-manual/admin/backup-overview' },
                        { text: '备份媒体与资源库工作区', link: '/zh-cn/user-manual/admin/media-backup' },
                        { text: '数据库快照', link: '/zh-cn/user-manual/admin/database-snapshots' },
                        { text: '验证备份与恢复演练', link: '/zh-cn/user-manual/admin/verify-backups' },
                        { text: '恢复数据库', link: '/zh-cn/user-manual/admin/restore' },
                        { text: '灾难恢复清单', link: '/zh-cn/user-manual/admin/disaster-recovery' },
                    ]
                },
                {
                    text: '升级与迁移',
                    collapsed: true,
                    items: [
                        { text: '安全升级', link: '/zh-cn/user-manual/admin/upgrade' },
                        { text: '迁移到新设备或新主机', link: '/zh-cn/user-manual/admin/migrate-device' },
                        { text: '回退应用或数据库', link: '/zh-cn/user-manual/admin/rollback' },
                    ]
                },
                {
                    text: '运行与停用',
                    collapsed: true,
                    items: [
                        { text: '状态、队列与日志', link: '/zh-cn/user-manual/admin/monitor' },
                        { text: 'Lumen Intelligence 运维', link: '/zh-cn/user-manual/admin/lumen-operations' },
                        { text: '停用、导出与保留数据', link: '/zh-cn/user-manual/admin/decommission' },
                    ]
                },
            ]
        },
        {
            text: '了解原理',
            collapsed: true,
            items: [
                { text: '了解原理', link: '/zh-cn/user-manual/concepts/' },
                { text: '流明集的心智模型', link: '/zh-cn/user-manual/concepts/mental-model' },
                { text: '原件、数据库与派生文件', link: '/zh-cn/user-manual/concepts/originals-database-derivatives' },
                { text: '存储位置身份与资源库身份', link: '/zh-cn/user-manual/concepts/storage-root-and-repository' },
                { text: '上传、扫描与云导入为什么不同', link: '/zh-cn/user-manual/concepts/upload-scan-cloud' },
                { text: '内容重复与同名冲突', link: '/zh-cn/user-manual/concepts/duplicate-vs-conflict' },
                { text: '后台处理流水线', link: '/zh-cn/user-manual/concepts/processing-pipeline' },
                { text: '逻辑媒体项目、组件与堆叠', link: '/zh-cn/user-manual/concepts/media-items-and-stacks' },
                { text: '非破坏性编辑如何工作', link: '/zh-cn/user-manual/concepts/non-destructive-editing' },
                { text: '搜索与索引', link: '/zh-cn/user-manual/concepts/search-and-indexes' },
                { text: '人物与事件的派生和人工修正', link: '/zh-cn/user-manual/concepts/people-and-events' },
                { text: 'Desktop 与 Server 的运行边界', link: '/zh-cn/user-manual/concepts/desktop-and-server' },
                { text: 'Lumen Intelligence 数据流', link: '/zh-cn/user-manual/concepts/lumen-data-flow' },
                { text: 'Lumilio Agent 权限模型', link: '/zh-cn/user-manual/concepts/agent-permission-model' },
                { text: '为什么完整备份需要两部分', link: '/zh-cn/user-manual/concepts/complete-backup' },
            ]
        },
        {
            text: '参考',
            collapsed: true,
            items: [
                { text: '参考', link: '/zh-cn/user-manual/reference/' },
                {
                    text: '产品与界面',
                    collapsed: false,
                    items: [
                        { text: '术语表', link: '/zh-cn/user-manual/reference/glossary' },
                        { text: '支持的平台与架构', link: '/zh-cn/user-manual/reference/support-matrix' },
                        { text: '支持的媒体格式', link: '/zh-cn/user-manual/reference/media-formats' },
                        { text: '界面入口索引', link: '/zh-cn/user-manual/reference/ui-entry-index' },
                        { text: '队列与操作状态', link: '/zh-cn/user-manual/reference/queue-statuses' },
                    ]
                },
                {
                    text: '存储与配置',
                    collapsed: true,
                    items: [
                        { text: '资源库存储目录', link: '/zh-cn/user-manual/reference/storage-layout' },
                        { text: '`.lumilioroot` 与 `.lumiliorepo` 参考', link: '/zh-cn/user-manual/reference/repository-markers' },
                        { text: '应用私有状态路径', link: '/zh-cn/user-manual/reference/app-state-paths' },
                        { text: 'Server 配置参考', link: '/zh-cn/user-manual/reference/server-configuration' },
                        { text: '端口与网络参考', link: '/zh-cn/user-manual/reference/ports-network' },
                    ]
                },
                {
                    text: '安全、备份与兼容',
                    collapsed: true,
                    items: [
                        { text: '安全边界', link: '/zh-cn/user-manual/reference/security-boundaries' },
                        { text: '数据库快照格式与恢复状态', link: '/zh-cn/user-manual/reference/backup-format' },
                        { text: '组件与数据兼容性', link: '/zh-cn/user-manual/reference/compatibility' },
                        { text: '诊断字段参考', link: '/zh-cn/user-manual/reference/diagnostics-fields' },
                    ]
                },
                {
                    text: '版本与限制',
                    collapsed: true,
                    items: [
                        { text: '版本与发布产物', link: '/zh-cn/user-manual/reference/releases' },
                        { text: '升级与迁移索引', link: '/zh-cn/user-manual/reference/upgrade-index' },
                        { text: '当前限制', link: '/zh-cn/user-manual/reference/known-limitations' },
                        { text: '安全公告与报告边界', link: '/zh-cn/user-manual/reference/security-notices' },
                    ]
                },
            ]
        },
        {
            text: '开发者',
            collapsed: true,
            items: [
                { text: '开发者入口', link: '/zh-cn/user-manual/developer/' },
                { text: '一页架构概览', link: '/zh-cn/user-manual/developer/architecture' },
                { text: '仓库与组件地图', link: '/zh-cn/user-manual/developer/repository-map' },
                { text: '开发环境', link: '/zh-cn/user-manual/developer/development-environment' },
                { text: 'Taskfile 工作流', link: '/zh-cn/user-manual/developer/taskfile' },
                { text: '测试与门禁', link: '/zh-cn/user-manual/developer/testing' },
                { text: 'OpenAPI 与 API 文档', link: '/zh-cn/user-manual/developer/openapi' },
                { text: '生成代码与契约', link: '/zh-cn/user-manual/developer/generated-contracts' },
                { text: 'Lumen 集成边界', link: '/zh-cn/user-manual/developer/lumen-integration' },
                { text: '中文文档贡献', link: '/zh-cn/user-manual/developer/docs-contribution' },
                { text: '提交变更', link: '/zh-cn/user-manual/developer/contributing' },
            ]
        },
    ],
}
