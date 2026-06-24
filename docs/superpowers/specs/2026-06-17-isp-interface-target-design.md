# Phase 2 設計書 — ISP の検査対象の再定義（interface 定義を一級対象に）

**日付:** 2026-06-17
**ステータス:** Design（実装未着手）
**親設計書:** `docs/superpowers/specs/2026-06-16-accuracy-improvement-design.md`（rev3）の Phase 2（§4 / 課題 B = C9）
**前提:** Phase 1（測定基盤 `gss evaluate` ＋ recall 非回帰ゲート）はマージ済み（PR #9）

---

## 0. 目的（なぜ）

ISP は「クライアントが使わないメソッドへの依存を強制されないこと」（Martin）。違反が宿る場所は
**太い interface 定義**であり、Go では Pike「太い interface は弱い抽象」＋暗黙実装ゆえこれが二重に強まる。

現状の `go-solid-score` は ISP を **「struct の public メソッド数」**で採点している。だが文献・ツールの
両面で、これは ISP ではなく **SRP（低凝集 / god-class）の指標**である:

- ISP 固有のヒューリスティックは一貫して「interface のサイズ」（Martin / Fowler RoleInterface）。
- Go の `interfacebloat` リンター（golangci-lint 同梱）も **interface 定義のメソッド数だけ**を数える
  （デフォルト閾値 `max=10`、>10 で警告）。struct 実装やクライアント利用パターンの解析はしない。
- 「class/struct の public メソッド数」を ISP 指標にしている確立ツールは確認されず（現状実装は outlier）。

→ **検査対象が原則とズレている（C9）。** 太い interface 定義（例 `testdata/isp` の `FatInterface`、
11 メソッド）は現状 **採点対象ですらない**（struct しか見ないため不可視）。

### 採用方針（案A）

interface 定義を ISP の**一級検査対象に追加**する。**既存の struct 採点は温存**する。

- 全面シフト（案B = struct 採点撤去）は recall ガード `FatImpl` の捕捉が interface 実装の有無に依存して
  脆くなるため不採用。
- クライアント利用パターン解析（案C）は理論的に最も正統だが、cross-package の call-graph / SSA を
  新規導入する必要があり（現状 `CountImplementors` は同一パッケージ scope のみ）、コスト過大。
  将来の理想形として位置づけ、今は見送る。

案A が低コストな根拠（コードベース確定事実）:
- `eval/collect.go` は interface の `// solid:want` ラベルを `<pkg>.<name>` で**既に収集済み**
  （コメントに "until interface scoring lands"）。analyzer が interface ごとの `Result` を emit するだけで
  join が成立する。
- `scorer.targetID(pkg,name,file)` により `FatInterface` と `FatImpl` は**別行**になり衝突しない。
- ISP 閾値 = 50（`config.go`、`score < threshold` で違反検出）。

---

## 1. スコープ

### やること
1. `analyzer/isp.go` に **interface 定義の採点**を追加（`pkg.Interfaces` を走査して Result を emit）。
2. `testdata/isp/` に interface ラベル（`FatInterface=violation` ほか）を付与。
3. `testdata/eval_baseline.json` を更新（ISP の TP 純増を記録）。

### やらないこと（Phase 2 のスコープ外）
- **struct 採点側の変更**（外部契約割引の追加など）。`FatImpl` の捕捉も afero ラッパの扱いも現状維持。
  外部契約由来の減点緩和は判別軸（実装 interface の定義元が外部か自パッケージか）が理論上ありうるが、
  型解析の新規実装を要し、afero の FP が実害かは未実測。**実測してから別 Phase**で判断する
  （設計書 rev3 §4.2「判別軸が無ければ両方 TP のまま＝recall 優先」に忠実）。
- クライアント利用パターン解析（案C）。
- DIP / SRP / 他原則への波及。

---

## 2. 設計詳細

### 2.1 interface 採点ロジック（`analyzer/isp.go`）

`ISPAnalyzer.Analyze` に interface 走査を追加する。struct 走査は現状のまま:

```
for _, s := range pkg.Structs {
    results = append(results, a.analyzeStruct(s, pkg))   // 現状維持
}
for _, iface := range pkg.Interfaces {
    results = append(results, a.analyzeInterface(iface, pkg))  // 新規
}
```

`analyzeInterface` は `InterfaceInfo.TotalMethods`（embed 由来を含む総数）を主シグナルにする。
`Result` は struct と同じ構造（`Principle=ISP`, `TargetName=iface.Name`, `TargetPkg=pkg.PkgPath`,
`TargetFile/Line=iface.File/Line`）。

### 2.2 スコア関数（interfacebloat 整合）

`interfacebloat` の業界標準（メソッド数 > 10 で違反）を ISP 閾値 50 割れに整合させる:

| メソッド数（TotalMethods） | スコア | 根拠 |
|---|---|---|
| ≤ 3 | 100 | Go idiom（`io.Reader` 等の小 interface） |
| 4–5 | 90 | まだ凝集的 |
| 6–7 | 75 | やや太い（splitting を検討） |
| 8–10 | 60 | 警告域（interfacebloat 閾値の手前） |
| 11–15 | **40** | **閾値 50 割れ＝違反検出。`FatInterface`(11) はここ → TP** |
| 16+ | 20 | 重度肥大 |

`Details` に「N methods (...)」の説明を付す（既存 struct 採点と同じ語法）。

### 2.3 embedding 合成への加点（FP 回避）

`len(iface.Embeds) > 0` の interface は、小さい role interface を合成したもの（例 `io.ReadWriteCloser`、
`testdata/isp/good.go` の `ReadWriter`）であり、ISP に**忠実**。よって **+15** 加点する
（合成後も巨大なら下限は残るよう `Clamp` 後の値で判断）。これにより正当な合成 interface が
FP にならないことを担保する。

### 2.4 採点対象の限定（外部契約の構造的除外）

採点対象は `pkg.Interfaces`（= 解析スコープ内で**自パッケージに定義された** interface）のみ。
afero.File / zap encoder のような**外部パッケージ定義**の太い契約 interface はそもそも `pkg.Interfaces`
に現れないため、interface 採点側では構造的に除外される。判別ロジックは不要。

### 2.5 Confidence

interface 採点の `Confidence` は、メソッド数が閾値域（≥8）なら `ConfidenceMediumHigh`、
小 interface（≤3）は `ConfidenceMedium`。合成 interface（Embeds あり）は判断が明快なので
`ConfidenceMediumHigh`。（連続スコア＋confidence で白黒二値にしない方針。）

---

## 3. testdata ラベル

`testdata/isp/` に以下を付与（ID は `<pkg>.<name>`、Phase 1 のハーネスが即 join）:

| 型 | 種別 | ラベル | 役割 |
|---|---|---|---|
| `FatInterface`（11 メソッド） | interface | `ISP=violation` | **新規 TP**（現状無ラベルで不可視） |
| `Reader`（1 メソッド） | interface | `ISP=ok` | TN（小 interface） |
| `Writer`（1 メソッド） | interface | `ISP=ok` | TN |
| `ReadWriter`（embedding 合成） | interface | `ISP=ok` | 合成は ISP 忠実 → 加点で FP 回避を実証 |
| `FatImpl`（struct） | struct | `ISP=violation`（既存） | recall ガード（struct 経路で死守） |
| `SimpleReader`（struct） | struct | `ISP=ok`（既存） | TN |

ラベルの `reason` には文献根拠（Martin / Pike / interfacebloat）を明記する。

---

## 4. 完了の定義（設計書 §4.3 準拠）

1. ISP の precision/recall が**改善**（または precision 改善かつ recall **非回帰**）。
2. `FatImpl`（struct）が引き続き ISP 違反として捕捉される（**recall ガード死守**）。
3. `FatInterface`（interface）が新規 TP として捕捉される。
4. embedding 合成 interface（`ReadWriter`）が **FP にならない**。
5. `testdata/eval_baseline.json` を更新し、ISP の TP が純増することを記録。
   recall 非回帰ゲート（`scripts/evaluate.sh`）が緑。
6. 全テスト緑（`go test -race ./...`）、gofmt / vet clean。

「afero.File は許容、FatImpl は捕捉」を分ける根拠は文献基準で説明可能だが（実装 interface の定義元が
外部か自パッケージか）、Phase 2 では interface 採点側を自パッケージ限定にすることで**自然に達成**し、
struct 採点側の外部契約割引は実害を実測してから別 Phase に送る。

---

## 5. リスクと不確実性

- **既存 ISP テストへの影響**: `analyzer/isp_test.go` は struct（`FatImpl` 等）にキーしているため、
  interface 採点の**追加**では壊れない想定。実装時に確認する。
- **baseline 非互換**: interface 採点追加で ISP の混同行列の母数が変わる（意図した変更）。
  `scripts/evaluate.sh --update` で再生成し、diff をレビューしてコミット。recall 非回帰は gate で担保。
- **閾値の妥当性**: スコア境界（11 で 40 点）は interfacebloat の >10 を根拠にした初期値。
  Phase 1 の P/R 測定で FP/FN が出れば調整する（train/test split で調整は train 側に閉じる）。
- **interfacebloat 既定値の出典**: `max=10` は golangci-lint docs / 既知情報ベース。一次ソース
  （リポジトリ README）での byte-exact 確認は未了だが、設計の結論には影響しない（連続スコアで吸収）。

---

## 6. 参考文献

- Martin, "Interface Segregation Principle"（C++ Report 1996）:
  https://condor.depaul.edu/dmumaugh/OOT/Design-Principles/isp.pdf
- Fowler, RoleInterface: https://martinfowler.com/bliki/RoleInterface.html /
  HeaderInterface（アンチパターン）: https://martinfowler.com/bliki/HeaderInterface.html
- Go Proverbs（"The bigger the interface, the weaker the abstraction" — Pike）:
  https://go-proverbs.github.io/
- Go Code Review Comments（Interfaces）: https://go.dev/wiki/CodeReviewComments
- Cheney, "SOLID Go Design": https://dave.cheney.net/2016/08/20/solid-go-design
- interfacebloat: https://github.com/sashamelentyev/interfacebloat /
  https://golangci-lint.run/usage/linters/#interfacebloat
