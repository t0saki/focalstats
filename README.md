# focalstats

给定一个目录，**高效、低资源占用**地统计其中照片的**焦段使用分布**——摄影师常用来分析自己最常用的焦段、辅助选购镜头。

- 只读 EXIF 元数据，**从不解码整图**（基于 [`evanoberholster/imagemeta`](https://github.com/evanoberholster/imagemeta)），CPU/内存占用低且与图片尺寸无关。
- 并发遍历（worker 池 = CPU 核数），数十万张图片秒级完成。
- 单次运行：给路径 → 输出统计 → 退出。
- 支持 **JPEG / HEIC / TIFF** 与常见 **RAW**（DNG/CR2/CR3/NEF/ARW/RW2/RAF/ORF…）。
- 多架构 Docker 镜像（`linux/amd64` + `linux/arm64`），发布在 GHCR。

## Docker（推荐）

镜像：`ghcr.io/t0saki/focalstats`。将照片目录以**只读**方式挂载到容器内，把挂载点作为参数传入：

```bash
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats /data

# 机器可读输出
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats --json /data

# 按实际焦距、合并到 5mm 桶、只看前 10
docker run --rm -v /你的照片目录:/data:ro ghcr.io/t0saki/focalstats \
  --basis actual --round 5 --top 10 /data
```

## 本地运行（需要 Go 1.23+）

```bash
go run . /你的照片目录
go build -o focalstats . && ./focalstats /你的照片目录
```

## 用法

```
focalstats [flags] <path>
```

| Flag        | 默认    | 说明                                              |
| ----------- | ------- | ------------------------------------------------- |
| `--basis`   | `35mm`  | 焦段基准：`35mm`（等效35mm）或 `actual`（实际焦距）|
| `--round`   | `1`     | 焦段分桶步长（mm）                                 |
| `--top`     | `0`     | 只显示使用最多的前 N 个焦段（0 = 全部）            |
| `--workers` | `0`     | 并发 worker 数（0 = CPU 核数）                     |
| `--ext`     | 内置集  | 逗号分隔的扩展名，覆盖内置格式集（如 `jpg,heic`）  |
| `--json`    | `false` | 输出 JSON                                         |
| `--csv`     | `false` | 输出 CSV                                          |

统计写到 stdout，进度/错误写到 stderr，便于管道处理。

## 示例输出

```
focalstats — 焦段使用统计 (基准: 等效35mm)
扫描图片: 22572  含焦段: 22572  无焦段: 0  读取失败: 0

  焦段(mm)     数量     占比  分布
  --------  -------  -------  ----------------------------------------
        24    10230    45.3%  ████████████████████████████████████████
        26     2431    10.8%  █████████
        35      984     4.4%  ███
        70     2870    12.7%  ███████████
        77     1983     8.8%  ███████
```

## 工作原理

1. `filepath.WalkDir` 遍历目录，按扩展名过滤候选文件。
2. 文件路径送入带缓冲 channel，worker 池中的每个 goroutine 只读取 EXIF 头、提取焦段、累加到**私有计数表**。
3. 所有 worker 结束后合并计数表（无锁竞争），渲染为表格 / JSON / CSV。

任意时刻每个 worker 只持有一个文件、只读元数据字节，因此内存占用恒定有界，与目录规模和图片大小无关。

## License

MIT
