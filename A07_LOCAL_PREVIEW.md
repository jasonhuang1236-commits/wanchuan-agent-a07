# A07 本地预览

本预览用于检查当前开发界面和流程，尚不是上线验收版本。不连接生产数据库，不配置真实支付渠道。

## 查看

- 登录：http://127.0.0.1:3107/sign-in
- 代理总览：http://127.0.0.1:3107/agent-platform
- 兑换码中心：http://127.0.0.1:3107/redemption-codes?view=center
- 兑换码记录：http://127.0.0.1:3107/redemption-codes?view=records
- 钱包：http://127.0.0.1:3107/wallet
- root 管理：http://127.0.0.1:3107/agent-platform/finance

演示账号：代理 `a07_andy`、root `a07_root`、普通用户 `a07_user`。本机生成的登录密码保存在 `~/Library/Application Support/WanchuanA07Preview/ACCOUNTS.txt`，不提交到 Git。

初始演示数据：代理余额 $1,000、可用授信 $1,000、邀请返利 $30 已到账和 $5 冻结中。数据仅用于演示，不对应任何实际资金。

## 构建与运行

在 `web/` 执行 `./node_modules/.bin/rsbuild build`，然后在仓库根目录执行：

```sh
GOWORK=off go build -o /tmp/wanchuan-a07-preview-server .
python3 scripts/a07-preview.py start
python3 scripts/a07-preview.py status
```

停止：`python3 scripts/a07-preview.py stop`。更新构建后先停止旧预览再启动；重复启动保留已有演示操作和密码。前端构建需要使用现有依赖目录。

后端将前端构建文件嵌入同一个程序，浏览器与 API 共用端口 3107；服务仅监听 `127.0.0.1`。SQLite 数据、程序日志、演示账号和会话信息都在独立目录 `~/Library/Application Support/WanchuanA07Preview/`，与数据库兼容性测试目录分开。

预览未配置支付渠道，不能进行真实在线支付；邀请奖励划转保留现有合规确认门槛。A04/A05 已补齐代理入口与七语言文案；报价、记录详情、root 查询与完整上线验收继续收尾。钱包与代理总览都可展开同一邀请卡的说明和记录，并打开联系客服提现窗口。

## 决定

提供本地开发预览供用户查看；继续完成 A07 功能、文案和验收，不部署生产。
