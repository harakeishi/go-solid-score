# go-solid-score スコアリングの仕組み

Go ソースコードを SOLID 原則（SRP / OCP / LSP / ISP / DIP）の観点から静的解析し、
各原則ごとに 0〜100 のスコアを付け、重み付き合算で総合スコアを算出するツール。
本ドキュメントはスコアリングの内部仕組みを 5 原則ごとにセクション分けして整理したもの。

---

## 全体アーキテクチャ

スコアリングは「メトリクス計算」と「宣言的ルール評価」の 2 段に分かれている。
原則ごとのスコア計算ロジックは Go コードではなく `rules/presets.yaml` の
**宣言的ルール**として定義されており、ユーザーが上書き・追加・無効化できる。

```
ソースコード
   │  parser がパース
   ▼
model.PackageInfo（struct / interface / method / func 情報）
   │  analyzer/metrics.go がメトリクス（事実）を算出
   ▼
rules.Metrics（"lcom4": 2, "type_switch_count": 3 ... という数値の辞書）
   │  rules/engine.go が presets.yaml のルールを上から順に適用
   ▼
各原則ごとの Outcome（Score / Confidence / Details）
   │  scorer/scorer.go がターゲット単位にマージし、重み付き合算
   ▼
ScoreResult（5原則スコア + Total + Confidence + Details）
```

### 主要コンポーネント

| 役割 | ファイル |
|------|----------|
| メトリクス（解析事実）の算出 | `analyzer/metrics.go` ＋ 各原則のヘルパー（`srp.go` 等） |
| 原則ごとのスコアリングルール（**スコアの本体**） | `rules/presets.yaml`（バイナリに埋め込み） |
| ルール評価エンジン | `rules/engine.go` |
| 原則アナライザー（メトリクス→エンジンへ橋渡し） | `analyzer/rule_analyzer.go` |
| ターゲット集約・重み付き合算 | `scorer/scorer.go` |
| 重み・しきい値のデフォルト | `config/config.go` |

### ルールエンジンの評価モデル

`rules/engine.go` の `Evaluate` が 1 ターゲット × 1 原則に対し以下を行う。

1. その原則の `defaults`（`base_score` / `base_confidence`）を初期値に置く。
   例: SRP・DIP は base_score=100 / confidence=1.0、OCP・LSP・ISP は confidence=0.7。
2. その原則のルールを **YAML に並んだ順に上から** 適用。各ルールは前のルールが
   残したスコアを起点に動く（積み上げ式）。
3. 各ルールの構造:
   - `metric` … 読み取る 1 つのメトリクス名
   - `where` … 前提条件（すべて満たさないとルールをスキップ）
   - `when` … 発火条件（`> 0`, `<= 5`, `== 1` 等の比較）。`bands` を使うと
     複数の閾値帯を上から判定し最初にマッチした帯のみ適用
   - `effect` … スコアへの作用
     - `penalty` … 減点（デフォルト）
     - `bonus` … 加点
     - `set` … スコアを固定値に上書き
     - `none` … スコアは変えない（confidence の更新や `stop` 用）
   - 効果量: `value`（固定値） / `from_metric: true`（メトリクス値 × `scale`、
     `cap` で上限）
   - `confidence` … マッチ時に信頼度を更新
   - `stop: true` … マッチしたらその原則のルール評価を打ち切る
   - `message` … 詳細メッセージ（`%v` にメトリクス値を埋め込む）
4. 最後にスコアを **[0, 100] にクランプ**。

`StructMetrics`（struct 用）と `InterfaceMetrics`（interface 用）の 2 種があり、
ルールは `target: struct` / `target: interface` で対象を切り替える。

### 信頼度（Confidence）

スコアとは別に各原則ごとの「判定の確からしさ」を 0.3〜1.0 で持つ。
メソッド数が少ない・依存が無い等で判定材料が乏しいときに低く設定され
（例: メソッド 1 個以下なら 0.3）、形式上スコアは出るが過信しないための指標。

### 総合スコア（Total）の算出

`scorer/scorer.go` の `computeTotal` が、各原則スコアを**重み付き加重平均**する。

```
Total = Σ(score_i × weight_i) / Σ(weight_i)   （小数第1位に丸め）
```

デフォルト重み（`config/config.go`）:

| 原則 | 重み | デフォルトしきい値 |
|------|------|------|
| SRP | 0.30 | 60 |
| OCP | 0.15 | 50 |
| LSP | 0.10 | 50 |
| ISP | 0.20 | 50 |
| DIP | 0.25 | 60 |
| **Total** | — | 70 |

重み 0 の原則は合算から除外される。

> **interface ターゲットの注意**: interface 定義は ISP のみで採点されるため、
> その Total は struct の 5 原則 Total とは比較できない。formatter は両者を
> 別セクションで表示する（`ScoreResult.IsInterface` で区別）。

---

## 1. SRP（単一責任の原則）

**考え方**: メソッド群とフィールドの結びつき（凝集度）を **LCOM4** で測り、
責任が分裂している型（凝集度が低い型）を減点する。サイズ・複雑度も加味。

### メトリクス（`analyzer/srp.go`, `metrics.go`）

- **LCOM4**: メソッドをノード、「同じフィールドにアクセス」または「互いを呼び出す」
  関係をエッジとした無向グラフの**連結成分数**（`calculateLCOM4`）。
  - 1 なら全メソッドが繋がっている＝凝集が高い。2 以上で責任分裂の疑い。
  - フィールドにアクセスせず他メソッドとも結合しない単独メソッド
    （`errors.Is` 規約メソッド等）は成分から除外し、誤検知を防ぐ。
- **凝集ペナルティ `srp_cohesion_penalty`** (`srpCohesion`):
  - 基礎ペナルティ `baseLCOM4Penalty`: 成分 2 個で 40、1 個増えるごとに +15、
    上限 70（線形ランプ）。
  - **サイズ減衰**: 平均成分サイズ（メソッド数 / LCOM4）が 3〜10 に増えるにつれ
    ペナルティを 1.0→0.25 へ減衰。大きく構造化された集約型を、小さく断片化した
    型と同じには罰しないため。
- `total_complexity`: 全メソッドの循環的複雑度の合計。
- `method_count`, `field_count`, `has_fields`。

### ルール（`presets.yaml`）

| ルール | 条件 | 効果 |
|--------|------|------|
| `srp-too-few-methods` | method_count ≤ 1 | 採点せず終了（confidence 0.3） |
| `srp-stateless` | フィールド無し かつ method_count ≤ 5 | スコア=80 固定で終了（LCOM4 非適用） |
| `srp-confidence-*` | メソッド数に応じ | 信頼度を 0.5 / 0.7 / 1.0 に設定 |
| `srp-cohesion` | cohesion_penalty > 0 | その値ぶん減点 |
| `srp-cohesion-confidence` | 平均成分サイズ ≥ 7.67 | 大型集約とみなし confidence 0.7 |
| `srp-complexity` | 複雑度 > 40 / > 20 | -20 / -10 |
| `srp-method-count` | メソッド > 15 / > 10 | -15 / -5 |

**ベース 100 点**。凝集度・複雑度・メソッド過多で減点していく。
フィールドの無い小さなユーティリティ型は LCOM4 が無意味なため一律 80 点固定。

---

## 2. OCP（開放閉鎖の原則）

**考え方**: 型分岐（type switch / type assertion / reflection）が多いほど、
新種の追加に対してコード修正が必要＝拡張に閉じていないとみなして減点。
逆に interface を引数に取るメソッドは拡張ポイントとして加点。

### メトリクス（`metrics.go`）

メソッド全体を走査して集計:

- `type_switch_count`: type switch 文の総数
- `type_assert_count`: 型アサーションの総数
- `reflect_count`: reflect パッケージ使用回数
- `type_check_density`: `(switch + assert + reflect) / 総ステートメント数`
- `iface_param_count`: interface 型を取る引数の数

### ルール（`presets.yaml`）

| ルール | 条件 | 効果 |
|--------|------|------|
| `ocp-no-methods` | method_count == 0 | 採点せず終了（confidence 0.3） |
| `ocp-type-switch` | > 0 | 件数 × 15 を減点（上限 40） |
| `ocp-type-assert` | > 0 | 件数 × 10 を減点（上限 40） |
| `ocp-reflect` | > 0 | 件数 × 5 を減点（上限 20） |
| `ocp-density` | 密度 > 0.3 / > 0.15 | -20 / -10 |
| `ocp-iface-param-bonus` | iface引数 > 0 | 件数 × 5 を加点（上限 20） |
| `ocp-confidence-high` | method_count ≥ 5 | confidence 1.0 |

**ベース 100 点 / confidence 0.7**。型分岐の密度が高いほど減点、
interface 駆動の設計には加点。

---

## 3. LSP（リスコフの置換原則）

**考え方**: 型が実装している interface の契約を、サブタイプが破っていないか。
パッケージ内の interface を**完全実装**しているメソッドだけを「契約メソッド」とみなし
（`implementedInterfaceMethods`）、その中の契約違反シグナルを減点する。

### メトリクス（`analyzer/lsp.go`, `metrics.go`）

契約メソッドに限定して数える:

- `implements_interface`: パッケージ内 interface を実装しているか（0/1）
- `unconditional_panic_count`: 無条件 panic するメソッド数
  （契約上呼べるはずが必ず panic ＝置換不能）
- `noop_count`: 中身が空（no-op）の契約メソッド数
- `embed_missing_override_count`: 埋め込んだ in-package interface のメソッドのうち
  オーバーライドしていない数（意図しない振る舞いの継承）

### ルール（`presets.yaml`）

| ルール | 条件 | 効果 |
|--------|------|------|
| `lsp-no-interface` | interface 未実装 | 採点せず終了（confidence 0.3） |
| `lsp-implements` | 実装あり | confidence 0.85 |
| `lsp-unconditional-panic` | > 0 | 件数 × 20 を減点 |
| `lsp-noop` | > 0 | 件数 × 15 を減点 |
| `lsp-embed-missing-override` | > 0 | 件数 × 10 を減点 |

**ベース 100 点 / confidence 0.7**。interface を実装していない型は採点対象外
（DIP/LSP の判定材料が無いため）。

---

## 4. ISP（インターフェース分離の原則）

唯一 **struct と interface 定義の両方**を採点する原則。
「肥大化したインターフェースを利用者に強いない」という原則本来の主語は
interface 定義側だが、struct の public メソッド面（事実上の利用面）も評価する。

### 4-1. struct ターゲット

#### メトリクス（`analyzer/isp.go`, `metrics.go`）

- `public_method_count`: public メソッド数（面の広さ）
- `is_decorator`: デコレータ/アダプタ判定（`isDecoratorPattern`）。
  単一フィールドを包み、public メソッドの 70% 以上がそのフィールドへ委譲する型。
  面の広さが包んだ型に由来するため寛大に扱う。
- `public_lcom4`: public メソッド面の LCOM4（public ≥ 4 のとき）
- `isp_large_iface_penalty` / `isp_composition_bonus`
  （`ispInterfaceCoupling`）: 実装している大型 interface への減点と、
  composition で組まれた interface 実装への加点。

#### ルール

| ルール | 条件 | 効果 |
|--------|------|------|
| `isp-no-public-methods` | public == 0 | 採点せず終了 |
| `isp-decorator` | デコレータ | スコア=85 固定で終了 |
| `isp-struct-size` | public > 20 / >15 / >10 / >5 | スコアを 20 / 40 / 60 / 80 に**設定** |
| `isp-large-interface` | 大型iface実装 | ペナルティ値ぶん減点 |
| `isp-composition-bonus` | composition実装 | 加点 |
| `isp-public-cohesion` | public_lcom4 > 2 | -15（public 面の凝集低下） |

### 4-2. interface ターゲット

#### メトリクス（`InterfaceMetrics`）

- `total_methods`: 埋め込み込みの総メソッド数
- `direct_methods`: 直接宣言メソッド数
- `embed_count`: 埋め込み interface 数

#### ルール

| ルール | 条件 | 効果 |
|--------|------|------|
| `isp-interface-size` | total > 15 / >10 / >7 / >5 / >3 | スコアを 20 / 40 / 60 / 75 / 90 に**設定** |
| `isp-interface-composition` | 直接 ≤ 5 かつ embed > 0 | +15（合成で組まれた小さな interface） |

メソッドが多い interface ほど低スコア。小さな interface を埋め込みで合成した
設計は加点する。

---

## 5. DIP（依存性逆転の原則）

**考え方**: 具体型ではなく抽象（interface）に依存しているか。
依存対象を「フィールド・コンストラクタ引数・public メソッド引数」から集計し、
**interface 依存の比率**をスコアにする。

### メトリクス（`analyzer/dip.go`, `metrics.go`）

依存を重み付きで集計（`dipMetrics`）:

- **重み**: フィールド依存 = 1.0、コンストラクタ引数 = 1.0、
  メソッド引数 = 0.3（呼び出し時依存は DIP 上の重要度が低い）
- `weighted_dep_total` / `weighted_dep_iface`: 重み付き総依存 / うち interface 依存
- `iface_dep_ratio`: `iface / total`（**スコアの主体**）
- `structural_dep_total`: フィールド＋コンストラクタの構造的依存合計
- `has_constructor_injection`: コンストラクタが interface 引数で DI しているか

**`skipDep` による除外**: 偽陽性を避けるため次の依存はカウントしない。
- whitelist 済みの組込/標準ライブラリ値型
- 関数型（コールバック/ストラテジ＝振る舞い注入）
- 純粋データ値型（int, string, map[string]string 等）
- 自己参照（再帰・木構造）

ただし **interface 依存は常にカウント**（抽象への依存こそ DIP の本質）。
`db *sql.DB` や `[]*Worker` のような具体依存はちゃんと残す。

### ルール（`presets.yaml`）

| ルール | 条件 | 効果 |
|--------|------|------|
| `dip-no-dependencies` | 総依存 == 0 | 採点せず終了（DIP 非適用） |
| `dip-calltime-floor` | 構造依存無し かつ ratio < 0.5 | スコア=50 固定（confidence 0.3） |
| `dip-calltime` | 構造依存無し | ratio × 100 をスコアに設定（confidence 0.3） |
| `dip-ratio` | （上記以外） | ratio × 100 をスコアに**設定** |
| `dip-di-bonus` | コンストラクタ DI あり | +15 |
| `dip-confidence-*` | 総依存 ≥ 3 / < 3 | confidence 0.85 / 0.7 |

**ベース 100 / confidence 1.0**。最終スコアは概ね「interface 依存比率 × 100」に
DI ボーナスを足したもの。呼び出し時依存しか無い場合は判定材料が弱いため
中立寄り（floor 50）かつ低信頼度で扱う。

---

## まとめ

- スコアの**本体は `rules/presets.yaml` の宣言的ルール**にあり、Go コードは
  メトリクス（事実）の算出とエンジン駆動に徹する。
- 各原則は **ベース 100 点から減点/加点/上書き**していく積み上げ方式。
- **総合スコア = 5 原則の重み付き加重平均**（SRP 0.30 / DIP 0.25 / ISP 0.20 /
  OCP 0.15 / LSP 0.10）。
- スコアと別に **信頼度**を持ち、判定材料の乏しさを表現する。
- ユーザーは `.go-solid-score.yaml` の `rules:` で preset と同じ `id` を使って
  上書き、新 `id` で追加、`enabled: false` または `disable_rules:` で無効化できる。
