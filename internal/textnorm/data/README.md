# 简繁转换字典

使用 [OpenCC ver.1.1.9](https://github.com/BYVoid/OpenCC/tree/ver.1.1.9) 的官方原始字典，文件未作修改。许可证为 Apache-2.0，全文随附于 `OpenCC.LICENSE`，版权声明按该许可证保留。

实现仅提供 `t2s`：优先匹配最长词组，再使用字符的首选简体映射。词组可以保留“乾隆”“覆盆子”等例外；不支持其他 OpenCC 配置，不进行同音错词修正，也不将“滑鼠”等地区用词改写为其他词。`t2s.json` 是上游配置原文，供核对词典组合。

| 文件 | 上游路径 | SHA-256 |
| --- | --- | --- |
| TSPhrases.txt | `data/dictionary/TSPhrases.txt` | `504169029c43f7f234b8e2ae470720af3657675c5574ff8aa0feb257e1dc5ce2` |
| TSCharacters.txt | `data/dictionary/TSCharacters.txt` | `6b5a0a799bea2bb22c001f635eaa3fc2904310f0c08addbff275477a80ecf09a` |
| t2s.json | `data/config/t2s.json` | `b818534194f27c2d95f01001edb0a5ec49b9050119892cb30a0504bb202cc07c` |
| OpenCC.LICENSE | `LICENSE` | `b534e465949558eec2597b04f5092b5e161236a68dfbfd04d547592ac3964308` |

下载地址前缀固定为 `https://raw.githubusercontent.com/BYVoid/OpenCC/ver.1.1.9/`。应用使用 Go embed 编入两个字典，无需外部服务、运行时下载或 C/C++ 依赖。中间字幕与最终字幕均应对当前完整文本调用 `textnorm.Simplified`，最终再保存和发送到界面。
