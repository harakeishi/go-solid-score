# 設計: 凝集度メトリクスを LCOM4 から LSCC に置換 (#18)

- **対象 Issue**: https://github.com/harakeishi/go-solid-score/issues/18
- **日付**: 2026-06-30
- **ステータス**: 承認済み

## 背景

SRP（単一責任の原則）スコアリングの中核である凝集度メトリクスは、現在 **LCOM4**
（メソッド-フィールドアクセスグラフの連結成分数）で計算されている。

LCOM4 には実証研究で繰り返し指摘されてきた弱点がある:

- **欠陥予測・保守性との相関が弱い**: Al Dallal 2012 / 2018 の比較研究で、
  LCOM3/LCOM4 は判別力が最も低く、欠陥予測子として統計的に非有意。
- **測定特性の違反**: 非正規化で型サイズに依存（Briand-Daly-Wüst 1998）。
- **誤検知が多い**: SonarQube は false positive が多すぎるとして LCOM4 を削除（SONAR-4853）。

このため、より測定特性に優れた **LSCC**（Low-level Similarity-based Class Cohesion,
Al Dallal & Briand 2012）に置き換える。

## 決定事項（ブレインストーミングでの合意）

| 論点 | 決定 |
|---|---|
| LCOM4 と LSCC の関係 | **即置換（B）**。導入箇所が少ないうちに切り替える |
| LSCC の定義 | **論文定義に従う**。LSCC 公式自体は変えないが、論文が適用手順として想定する **メソッド集合 M の構成**で規約メソッドを除外する（下記「規約メソッド除外」参照） |
| 偽陽性対策 | (1) 規約メソッド除外（メソッド集合の構成）+ (2) しきい値（yml ルール）側で調整 |
| ペナルティ変換 | **生の LSCC(0〜1) をメトリクスとして出し、しきい値・ペナルティは yml の bands で表現（A）** |
| 旧 LCOM4 の扱い | **SRP からは完全に外す（B）**。`lcom4` メトリクスも SRP では出さない |

### 規約メソッド除外（実装中の調査で追加した決定）

実装中、testdata の `ParseError`（`msg string` 1フィールド、`Error()` が使用・`Is(target error)`
は引数だけ見てフィールド非参照）が **素の LSCC=0** となり偽陽性となることが実測で判明した。
Web 調査の結論に基づき、対策として **LSCC のメソッド集合 M から Go の規約メソッドを除外**する:

- 除外対象（シグネチャ一致）: `Is(error) bool` / `As(any) bool`（= `As(interface{}) bool`）/
  `Unwrap() error` / `Unwrap() []error`。
- 除外後の実効メソッド数が `< 2` なら LSCC 適用外（0.0 を返し、ルールの `where` で発火させない）。

**この除外は「素の定義を曲げる独自ヒューリスティック」ではない**:
- LSCC 論文（Al Dallal & Briand 2012）自身が、公式の `k`（メソッド数）が指すメソッド集合 M の
  構成は適用上の判断と位置づけ、著者の後続実証（J. Software 2013）は「access methods must be
  excluded」と明記。
- `errors.Is/As/Unwrap` が自フィールドを触らないのは Go 公式ドキュメント記載の標準パターン
  （smell ではない）。除外は `errors` パッケージ仕様から正当化できる。
- SonarQube も規約メソッド/logger を LCOM4 計算から除外していた。小クラスへの適用抑止も
  NDepend 等の標準慣行。

当初の「素の定義のみ・除外しない」方針は、LSCC の適用手順を踏まえると不完全だったため、
規約メソッド除外（争いの少ない範囲に限定。アクセサ除外やコンストラクタ除外までは行わない）に
更新する。

### 当初「素の定義のみ」を検討した際のメリデメ整理（記録）

- **メリット**: 論文定義に忠実なため実証研究の裏付けが保てる。正規化済み(0〜1)で
  外れ値に強く、シングルトンの影響は `1/(k·l·(l-1))` 程度に希釈されるため
  独自の除外ロジックが不要。実装がシンプル（約30行）。偽陽性対策が yml に一元化され、
  「repo 別にルールでノイズ調整」という本ツールの思想と一致。
- **デメリット**: LCOM4 で個別対応していた偽陽性（例: `errors.Is/As` 規約メソッド）が
  移行直後に再発しうるが、正規化の希釈効果と yml しきい値で吸収可能。LSCC は連続値なので
  しきい値の再設計が必要。LCOM4 時代とスコア非互換（導入箇所が少ないため許容）。

## LSCC の計算定義

各名前付きフィールド `f` にアクセスするメソッド数を `x_f`、メソッド数を `l`、
名前付きフィールド数を `k` とすると:

```
LSCC = Σ_f ( x_f · (x_f − 1) ) / ( k · l · (l − 1) )
```

- 値域は 0〜1。1 = 全メソッドが全フィールドを共有（最高凝集）、0 = 共有なし。
- `AccessedFields`（`model.MethodInfo`）から `x_f` を数える。
- **メソッド集合 M からは Go 規約メソッド（`Is(error) bool` / `As(any) bool` /
  `Unwrap() error` / `Unwrap() []error`）を除外**してから `l` と `x_f` を数える
  （上記「規約メソッド除外」参照）。シングルトン除外などその他の独自ヒューリスティックは
  持ち込まない。

### 境界ケース

- `l <= 1`（メソッド1個以下）または `k == 0`（名前付きフィールドなし）→ 分母が0。
  **LSCC 適用外**とし、`calculateLSCC` は **0.0 を返す**。
  スコアへの誤反映を防ぐため、ルール側で `where: ["method_count >= 2", "has_fields == 1"]`
  によりガードする（加えて既存の `srp-too-few-methods` / `srp-stateless` ルールが先に
  `stop: true` で打ち切るため、これらのケースで LSCC ルールは発火しない）。

## 変更内容

### 1. メトリクス計算（`analyzer/srp.go`, `analyzer/metrics.go`）

- **追加**: `calculateLSCC(methods []*model.MethodInfo, namedFields []model.FieldInfo) float64`
  - 上記定義を実装（約30行）。
- **追加**: `StructMetrics` に `m["lscc"]`。
- **削除**: SRP 用の `m["lcom4"]`, `m["srp_cohesion_penalty"]`, `m["srp_avg_component_size"]`、
  および `srpCohesion`・`baseLCOM4Penalty` 関数。
- **残す**: `calculateLCOM4`・`shareField`・`callsEachOther`
  （ISP の `public_lcom4` で現役のため）。

### 2. プリセットルール（`rules/presets.yaml`）

`srp-cohesion` / `srp-cohesion-confidence` を LSCC ベースに置換:

```yaml
- id: srp-cohesion
  principle: SRP
  metric: lscc
  where: ["method_count >= 2", "has_fields == 1"]
  bands:
    - { when: "< 0.2", value: 40, message: "very low cohesion (LSCC=%v)" }
    - { when: "< 0.4", value: 25, message: "low cohesion (LSCC=%v)" }
    - { when: "< 0.6", value: 10, message: "moderate cohesion (LSCC=%v)" }
```

- しきい値 0.2 / 0.4 / 0.6 とペナルティ 40 / 25 / 10 は初期値。実データで調整する前提。
- LCOM4 時代の「large aggregate を大目に見る size 減衰」は初期は再現しない（YAGNI）。
  必要なら別ルールの `where: ["method_count >= N"]` で yml 表現する。

### 3. メトリクス語彙（`metrics.go` の `metricNames`）

- `"lscc"` を追加。
- SRP 用の `"lcom4"`, `"srp_cohesion_penalty"`, `"srp_avg_component_size"` を削除。
- これらを参照する既存テスト/ユーザールールが壊れるため、テスト更新が必要。

### 4. テスト

- `calculateLSCC` の単体テスト:
  - 高凝集（全メソッドが全フィールドを共有）→ 1.0 付近
  - 低凝集（各メソッドが別フィールドのみ）→ 0 付近
  - 境界（メソッド1個、フィールド0個）→ 適用外として 0.0 を返す
- 既存 SRP スコアリングテスト・`config/rules_test.go`・`analyzer/metrics_test.go` の
  LCOM4 依存箇所を LSCC 基準に更新。

### 5. ドキュメント（README）

- メトリクス語彙の表を更新（SRP の lcom4 系を LSCC に差し替え、LSCC を追記）。

## スコープ外（YAGNI）

- TCC/LCC、C3（概念的凝集）などの他の凝集度メトリクス（別 Issue）。
- size 減衰ロジックの完全再現（初期は入れず、しきい値運用で代替）。
- LCOM4 を参考値として出力し続けること（B を選択したため出さない）。

## 出典

- Al Dallal & Briand 2012 "A Precise Method-Method Interaction-Based Cohesion Metric (LSCC)"
- Al Dallal 2018 CK replication study
- Briand-Daly-Wüst 1998（凝集度メトリクスの公理）
- SonarQube LCOM4 削除: SONAR-4853
