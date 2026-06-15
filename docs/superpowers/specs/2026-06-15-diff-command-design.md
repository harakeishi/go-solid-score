# 設計: 相対評価（回帰検出）による品質ゲート用 `diff` サブコマンド

日付: 2026-06-15
ステータス: 承認済み（スペックレビュー待ち）
ブランチ: `feat/diff-command`

## 背景 / 動機

`go-solid-score` は struct ごとに SOLID スコアを算出するが、**スコアの絶対値の
根拠は薄い**（−40/−70 といった減点幅はヒューリスティック）。そのため本ツールは
「絶対的な良し悪し」を断じる用途より、**相対評価** ＝「変更によって設計が *悪化*
したか」を検出する用途で最も価値を発揮する。

先行 PR（#3, マージ済み）で **安定したターゲット識別子** を導入した。各結果は
`<pkgPath>.<TypeName>`（例: `github.com/foo/bar.MyStruct`）形式の `id` を持ち、
絶対ファイルパスに依存しないため、ファイルのリネーム・移動を跨いでも2回の実行
結果で同一ターゲットを突き合わせられる。本スペックはこの土台の上に回帰検出層を
構築する。

## 業界調査（本設計の根拠）

SonarQube / Codecov / Coveralls / golangci-lint / betterer / reviewdog /
octocov を調査。一貫した「勝ちパターン」:

1. **2軸: 全体 vs 差分。** コードベース全体ではなく *変更された* 集合でゲートする
   （"Clean as You Code"）。既存レガシーで開発者を罰しない。
2. **絶対閾値と前回比悪化（relative drop）の両方をサポート。**
3. **微小な下落を無視する「遊び（tolerance）」が必須** — Coveralls は `-0.0%` で
   赤バツを出して悪名高く、Codecov の `project` デフォルト閾値は 5%、SonarQube は
   20行の fudge factor を持つ。**離散的な**我々のスコアでは特に重要。
4. **報告のみ vs ブロッキングはフラグで切り替え、exit code で表現**
   （reviewdog の `-fail-level`、golangci-lint の `issues-exit-code`）。
5. **baseline の渡し方**: ファイル方式（betterer / tsc-baseline）と git-rev 方式
   （golangci-lint）がある。ファイル方式はレビュー可能で、shallow clone でも動き、
   さらに我々は安定 ID を持つため、git-rev / 行ベース diff にありがちな行ずれ
   誤検知を回避できる。
6. **出力は 増加 / 減少 / 変化なし を一目で区別**（Codecov の `+`/`-`/`ø`）。
7. **octocov の責務分割**: CLI は計算と Markdown 生成まで。PR コメントの *投稿*
   （および「前回コメントの更新」）は CI 側の薄い層であり CLI の責務ではない。
   「baseline をどこに保存するか」は octocov では datastore 抽象化で解決して
   いるが、我々はこれを CI に委ねる（Non-goals 参照）。

## ゴール

- baseline JSON と、その場で解析した head を比較し、各ターゲットを分類する
  `diff` サブコマンド。
- **既存ターゲット（相対下落）** と **新規ターゲット（絶対 min-score）** を分けて
  扱う。
- 微小・ゼロの下落でノイズを出さないための遊び（tolerance）フラグ。
- デフォルトは報告のみ。フラグで CI 失敗をオプトイン。
- text / Markdown 出力（Markdown は octocov 風の PR コメント用）。
- PR コメントを投稿/更新する GitHub Actions ワークフローのサンプル。

## Non-goals（YAGNI のため意図的に見送り）

- baseline 取得のための **datastore 抽象化**（artifact/S3/GCS/BigQuery）。
  `base.json` の取得は CI ワークフローの責務とし、CLI は `--base <file>` だけを
  受け取る。将来追加する可能性はある。
- **git-rev ベースの baseline**（`--base-rev HEAD~1`）。今回はファイルのみ。
- **原則別ゲート** のデフォルト化。原則別の差分は verbose/details で表示する
  かもしれないが、ゲート判定は `total` で行う。
- バッジの自動コミット / SVG 出力。

## CLI

```
go-solid-score diff --base <base.json> [flags] [packages...]
```

`base.json` は事前に `go-solid-score -f json ...` で出力したもの。head は既存の
parse→analyze→score コアを再利用してその場で解析する。

### フラグ

| フラグ | デフォルト | 意味 |
|------|---------|------|
| `--base <file>` | （必須） | 比較元となる baseline JSON。 |
| `--max-drop <float>` | **5.0** | 既存ターゲットの `total` がこの値を**超えて**下落したら REGRESSED とする（業界に合わせた遊び）。 |
| `--min-score <float>` | 0（無効） | 新規ターゲットがこの値未満なら NEW-LOW とする。 |
| `--fail-on-regression` | false | REGRESSED または NEW-LOW が1件でもあれば exit 1。デフォルトは報告のみ（exit 0）。 |
| `-f, --format` | text | `text` / `json` / `markdown`。 |
| `-c, --config` | `.go-solid-score.yaml` | head の解析に既存の設定ロード（weights/thresholds/dip whitelist）を再利用。 |

解析に関わるルートフラグ（config, weights）は head の解析に適用し、base と head が
同じルールでスコアリングされるようにする。**注意:** baseline は同じ weights で
生成されている必要がある。weights の不一致はユーザーの責任（ドキュメントに明記）で
あり、ツールでは検証しない。

### 分類

base のターゲットと head のターゲットを `id`（安定した `<pkgPath>.<name>`、pkg path
が空なら `<file>:<name>` にフォールバック ＝ マージキーと同じルールで、既に
`scorer.targetID` に集約済み）で突き合わせる。

各 id について:

- **両方に存在** → `total` を比較:
  - `base.total - head.total > maxDrop` → **REGRESSED**
  - `head.total > base.total` → **IMPROVED**
  - それ以外 → **UNCHANGED**（遊びの範囲内の小さな下落も含む）
- **head のみ** → **NEW**、または `minScore > 0 && head.total < minScore` なら
  **NEW-LOW**
- **base のみ** → **REMOVED**（情報表示のみ。回帰とは扱わない）

exit code 判定上、REGRESSED または NEW-LOW を1件以上含む実行を「回帰あり」とみなす。

**符号規約:** 表示する `diff` は `head.total - base.total`。したがって下落は負
（例: `-14.0`）、改善は正（例: `+20.0`）。REGRESSED の判定式
`base.total - head.total > maxDrop` は `diff < -maxDrop` と等価。

### exit code

- 報告のみ（デフォルト）: 常に **0**。
- `--fail-on-regression` 指定時: 回帰ありなら **1**、なければ **0**。
- 使用方法 / I/O エラー: 非ゼロ（cobra の既存挙動）。

## 出力

### text（Codecov 風のマーカー）

```
go-solid-score diff (base: base.json)
====================================================
REGRESSED  github.com/foo/bar.Handler  72.0 -> 58.0 (-14.0)
NEW-LOW    github.com/foo/baz.Worker   45.0 (< min 70.0)
IMPROVED   github.com/foo/svc.Svc      60.0 -> 80.0 (+20.0)
NEW        github.com/foo/x.Y          90.0
REMOVED    github.com/foo/old.Thing
----------------------------------------------------
1 regressed, 1 new-low, 1 improved, 1 new, 1 removed, 12 unchanged
```

UNCHANGED のターゲットは個別に列挙せず、集計行にまとめる。

### markdown（octocov 風、PR コメント用）

先頭の HTML マーカーコメントにより、CI 側が前回コメントを探して更新できる:

```markdown
<!-- go-solid-score-diff -->
## go-solid-score

**1 regressed**, 1 new-low, 1 improved, 1 new, 1 removed, 12 unchanged.

| | target | base | head | diff |
|--|--|--|--|--|
| 🔻 REGRESSED | `github.com/foo/bar.Handler` | 72.0 | 58.0 | -14.0 |
| ⚠️ NEW-LOW | `github.com/foo/baz.Worker` | – | 45.0 | (< min 70.0) |
| 🔺 IMPROVED | `github.com/foo/svc.Svc` | 60.0 | 80.0 | +20.0 |
| ✨ NEW | `github.com/foo/x.Y` | – | 90.0 | – |
| 🗑 REMOVED | `github.com/foo/old.Thing` | 72.0 | – | – |

<details><summary>All targets (incl. 12 unchanged)</summary>

| target | base | head | diff |
|--|--|--|--|
... full table ...

</details>
```

注目すべき（変化のある）ターゲットを上部のテーブルに表示し、全件（UNCHANGED 含む）は
`<details>` に畳む。これは本リポジトリ既存の README スタイルおよび octocov と揃える。

### json

ツール連携用の機械可読形式。`{id, name, package, status, base, head, diff}` の
配列に、件数のサマリブロックと `regressed` 真偽値を付与する。

## コンポーネント（責務分離）

新規パッケージ **`differ`** — 純粋な比較ロジックのみ。I/O もスコアリングも持たない:

- `type Snapshot struct { ID, Name, Package string; Total float64 }`
  （diff に必要な最小限の投影。base JSON からデコードし、head の `ScoreResult` から
  生成する）。
- `type Status string` と定数 REGRESSED / IMPROVED / UNCHANGED / NEW /
  NEW_LOW / REMOVED。
- `type Entry struct { ID, Name, Package string; Status Status; Base, Head *float64 }`
- `type Report struct { Entries []Entry; Counts map[Status]int; Regressed bool }`
- `func Diff(base, head []Snapshot, opts Options) Report` — **純関数**。テスト対象の
  中核。`Options{MaxDrop, MinScore float64}`。

周辺の配線（薄く保ち、`differ` の外に置く）:

- **base のデコード**: `formatter` が出力する JSON 形状を再利用する。struct の二重
  定義を避けるため、JSON の結果形状を共有型（`formatter.JSONResult`）として抽出し、
  formatter が書き、diff 経路が読む。これで契約のソースを1つにする。
- **head のスコアリング**: `cmd.run()` の parse→analyze→score コア部分を再利用可能な
  関数（例: `cmd.analyze(cfg, patterns) ([]*scorer.ScoreResult, error)`）に切り出し、
  `run` と `diff` の両方から呼ぶ。これが唯一必要となる既存コードのリファクタで、
  現状「何でもやっている」`root.go` の形を改善する。
- **diff 出力の整形**: `differ.Report` の text/markdown/json レンダリングを追加する。
  発見しやすさのため、既存フォーマッタと同じ `formatter` 配下に置く（新規
  `diff_text.go` / `diff_markdown.go` / `diff_json.go`）。

## GitHub Actions サンプル

`.github/workflows/solid-diff.yml`（サンプル。README に記載）:

1. PR をチェックアウト。
2. merge base の `base.json` を取得 — サンプルでは `actions/cache`、または base ref を
   チェックアウトして `go-solid-score -f json` を実行する方式を示す（「datastore」的な
   関心事はここに置き、CLI には持たせない）。
3. `go-solid-score diff --base base.json -f markdown ./... > comment.md`
   （必要に応じて `--fail-on-regression`）。
4. `<!-- go-solid-score-diff -->` マーカーを使って PR コメントを投稿/更新する
   （メンテされている marketplace action、または小さな `gh pr comment` スクリプト）。
5. `permissions: pull-requests: write` が必要。fork PR の制約に注意: 書き込み
   トークンが無いため job summary にフォールバックする（octocov 同様、ドキュメントに
   明記）。

## テスト

- `differ.Diff`: 全ステータスを網羅するテーブル駆動テスト。`maxDrop` 境界
  （下落 == maxDrop → UNCHANGED、わずかに超過 → REGRESSED）、NEW と NEW-LOW の
  `minScore` on/off、REMOVED、`Regressed` フラグを検証。
- base デコードのラウンドトリップ: `formatter` の JSON 出力が id を保ったまま
  `Snapshot` にデコードできること（共有型の契約を守る）。
- フォーマッタ: 代表的な `Report` に対する text/markdown/json 出力（マーカー、
  マーカーコメント、集計行、details 折り畳みを検証）。
- `cmd diff`: ゴールデン / エンドツーエンドテスト — base.json を書き出し、testdata
  パッケージに対して実行し、回帰あり/なしの両方で `--fail-on-regression` 時の
  exit code を検証。

## ロールアウト

`feat/diff-command` 上の単一 PR: `differ` パッケージ + フォーマッタのレンダラ +
`cmd diff` + 共有 analyze リファクタ + Actions サンプル + README セクション +
テスト。Actions サンプルはドキュメント / CI 設定であり、CLI の実装をブロックしない。
