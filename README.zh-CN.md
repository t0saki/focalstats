# focalstats

[English](README.md) | 简体中文

给定一个目录，**高效、低资源占用**地统计其中照片的**焦段使用分布**——便于了解自己的拍摄习惯、辅助选购镜头。

- 只读 **EXIF 元数据**，**从不解码整图**（基于 [`evanoberholster/imagemeta`](https://github.com/evanoberholster/imagemeta)），CPU/内存占用低且与图片尺寸无关。
- 并发遍历（worker 池 = CPU 核数），数十万张图片秒级完成。
- 单次运行：给路径 → 输出统计 → 退出。
- 支持 **JPEG / HEIC / TIFF** 与常见 **RAW**（DNG/CR2/CR3/NEF/ARW/RW2/RAF/ORF…）。
- 输出终端表格、**JSON**、**CSV**，或自包含的**交互式 HTML 报告**（按机身/镜头/日期/光圈等任意筛选，所有图表在浏览器内实时重算）。
- 多架构 Docker 镜像（`linux/amd64` + `linux/arm64`），发布在 GHCR。

> 程序输出为英文。本文件是中文说明；程序界面与报告文案均为英文。

## Docker（推荐）

镜像：`ghcr.io/t0saki/focalstats`。将照片目录以**只读**方式挂载，把挂载点作为参数传入：

```bash
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats /data

# JSON / CSV
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats --json /data

# HTML 报告——再挂载一个可写目录，把文件写进去
docker run --rm -v /你的照片目录:/data:ro -v "$PWD":/out \
  ghcr.io/t0saki/focalstats --html -o /out/report.html /data

# 按实际焦距、合并到 5mm 桶、只看前 10
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats \
  --basis actual --round 5 --top 10 /data
```

## 本地运行（Go 1.23+）

```bash
go run . /你的照片目录
go build -o focalstats . && ./focalstats --html -o report.html /你的照片目录
```

## 用法

```
focalstats [flags] <path>
```

| Flag        | 默认     | 说明                                              |
| ----------- | -------- | ------------------------------------------------- |
| `--basis`   | `35mm`   | 焦段基准：`35mm`（等效35mm）或 `actual`（实际焦距）|
| `--round`   | `1`      | 焦段分桶步长（mm）                                 |
| `--top`     | `0`      | 焦段表与器材榜单的条数上限（0 = 合理默认值）       |
| `--workers` | `0`      | 并发 worker 数（0 = CPU 核数）                     |
| `--ext`     | 内置集   | 逗号分隔的扩展名，覆盖内置格式集                   |
| `--json`    | `false`  | 输出 JSON                                          |
| `--csv`     | `false`  | 输出 CSV                                           |
| `--html`    | `false`  | 输出自包含的 HTML 报告                             |
| `-o`        | stdout   | 将输出写入文件而非 stdout                          |

统计写到 stdout，进度/错误写到 stderr，便于管道处理。

## HTML 报告

`--html` 生成单一自包含、**可交互**文件：把每张照片的原始记录以 gzip+base64
内联，配套一小段脚本在**浏览器内**完成筛选与重新聚合。**无外部/CDN 资源、不联网、
完全离线。**

可按**机身、镜头、日期范围、时段、星期、光圈、ISO、快门速度**筛选，并切换
**焦段基准**（等效35mm ⇄ 实际焦距）；下列所有图表/表格实时重算：

- 概览计数 +「匹配 N / 总数」指示。
- 焦段分布直方图 + Top 表。
- **焦段 × 年份**热力图（看用焦习惯随时间演变）。
- **各相机机身的焦段分布**——对比不同机身/手机型号的差异。
- 一天中的时段、星期、年份与月度时间线。
- 相机机身、镜头 TopN；光圈、ISO、快门速度分布。

**体积与要求**：报告体积与"带 EXIF 的照片数"成正比，压缩后约**每 10 万张几百 KB**
（例如 15 万张的库约 1MB）。解压使用浏览器内置 `DecompressionStream`，请用较新的
Chrome/Edge/Firefox 或 Safari 16.4+ 打开。

## 示例（终端）

```
focalstats — focal-length usage (basis: 35mm-equivalent)
scanned: 22572  with focal: 22572  no focal: 0  failed: 0

  focal        count    share  distribution
  --------  --------  -------  ----------------------------------------
     24mm        10230    45.3%  ████████████████████████████████████████
     26mm         2431    10.8%  █████████
     35mm          984     4.4%  ███
     70mm         2870    12.7%  ███████████
     77mm         1983     8.8%  ███████
```

## 工作原理

1. `filepath.WalkDir` 遍历目录，按扩展名过滤候选文件。
2. 路径经带缓冲 channel 分发，每个 worker 只读 EXIF 头、提取字段、累加到**私有直方图**。
3. 所有 worker 结束后合并直方图（无锁竞争），渲染为表格 / JSON / CSV / HTML。

任意时刻每个 worker 只持有一个文件、只读元数据字节，因此内存占用恒定有界，与目录规模和图片大小无关；聚合按低基数值（焦段、小时、机身……）分桶，体积同样很小。

## License

MIT
