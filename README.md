# scpper

为 [pwikit](https://github.com/WikitTeam/ProjectWikit) 站点提供云币、抽卡和分部指数。scpper 作为独立程序运行，挂载在站点的 `/scpper/` 下，使用站点的账号登录。

## 安装

1. 从 Releases 下载对应系统的 `scpper`，与 `.env.example` 放在同一目录，并将后者复制为 `.env`。
2. 为 scpper 准备一个 PostgreSQL 数据库，并在 pwikit 的数据库上创建一个只读账号，填入 `.env`。
3. 在 pwikit 的 `pwikit.toml` 中添加挂载，然后重启 pwikit：

   ```toml
   [[mount]]
   path = "/scpper/"
   upstream = "http://127.0.0.1:8100"
   strip_prefix = true
   secret_file = "mount-scpper.key"
   ```

4. 将 `.env` 中的 `PWIKIT_SECRET_FILE` 指向 pwikit 数据目录下的 `secrets/mount-scpper.key`，然后运行 `scpper serve`。

pwikit 的超级管理员访问 `/scpper/` 时将自动成为管理员。挂载的完整说明见 pwikit 文档的「挂载外部程序」。

## 更新

```sh
scpper update
```

完成后请重启 scpper。

## 从源码构建

```sh
cd web && npm ci && npm run build && cd ..
go build -o scpper ./cmd/scpper
```
