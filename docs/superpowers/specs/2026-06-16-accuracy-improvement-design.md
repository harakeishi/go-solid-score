# go-solid-score 精度改善 設計書

**日付:** 2026-06-16
**ステータス:** Design（実装未着手）
**基準コミット:** `b7b2a97`（origin/main）
**改訂:** rev3 — recall 測定を最優先に再編。rev2 の「FP 件数順」方針の構造的欠陥を是正（経緯は §0.1）

---

## 0. この文書の位置づけ

ツールの精度を、**precision と recall の両面を測ったうえで**改善するための設計書。
実装の合意形成と着手順序の固定が目的で、本文書段階ではコードを変更しない。

確定済みの方針（ユーザー合意）:

1. **0-100 連続スコアと diff 機能のインターフェースは維持する。** 改善はペナルティ算出の
   内部ロジックに閉じ、出力契約（`Result.Score` が 0-100 float、JSON スキーマ、CI diff コメント）は変えない。
   「維持」= 出力インターフェース契約の維持であり、スコア値は改善により変わりうる（§4.4）。
2. **recall（真の違反の見逃し）を測ってから、precision/recall のバランスで直す。** FP（誤検出）だけを
   緩める片側修正はしない（§0.1, §2.4 のバイアス問題）。
3. 研究文献ベースの構造改革（合成ルール化・分布閾値導出・LLM）は、recall/precision の土台が
   できてから中長期の上積みとして取り組む。
4. 成果物は本設計書。実装は別セッション・別ブランチ（`feat/*`）で行う。最新 main を基準とする。

### 0.1 改訂履歴と、rev2 で露呈した根本問題

- **rev1 → rev2:** 「`benchmark.sh` もコーパスも存在しない」という事実誤認を、実測で全面改稿。
  （benchmark.sh / scoring-analysis.md は実在。§0.2 参照）
- **rev2 → rev3:** rev2 は「実測で件数の多い FP（ISP≤25 が 8.5%）を最優先で潰す」とした。
  しかし red team レビューが、この方針に**実コードで裏付けられる構造的欠陥**を指摘した:

  1. **【C9】ISP は原則を検査していない。** `analyzer/isp.go:22` は `pkg.Structs` を走査し
     **public メソッド数**を段階減点するだけ（`isp.go:60-75`）。ISP 本来の「太い interface を分割せよ
     （クライアントに不要メソッドを強制するな）」ではなく「実装 struct のメソッドが多い」を見ている。
     直すべきは「割引の追加」ではなく**検査対象の再定義**かもしれない。
  2. **【C3】recall ガードと本命施策が構造的に区別不能。** ISP の唯一の真陽性テスト
     `testdata/isp/bad.go` の `FatImpl` は、`FatInterface`（Read/Write/Seek/Stat/Sync/Truncate/Lock…=
     os.File 風の外部契約 11 メソッド）を実装する struct。一方 rev2 §5.2 が割り引きたい
     afero.File / zap.jsonEncoder も「外部契約を実装する広い surface」で**構造的に同一**。
     外部契約割引を入れると、守ると宣言した recall ガード `FatImpl` も同時に割り引かれる。
  3. **【C1】「有名ライブラリ=正しい」は未検証の権威バイアス。** rev2 §1.3 は gin.Context の
     ISP=20 を「FP 濃厚」としたが、scoring-analysis.md:83 自身が gin.Context を
     "frequently-criticised god object" と認めている。「中核型だから FP」と断じる根拠が自己矛盾。
  4. **【C4】全校正 5 ラウンドが penalty 緩和の片側バイアス。** git 履歴 6 コミット中 5 件が
     「stop/don't/only penalize」=緩和。新規の検出強化は 1 件もない。rev2 も「FP のみ手当て」と
     繰り返しており、ツールは反復のたび「何も検出しない」方向へ単調に漸近するラチェット構造。
  5. **【C2/C5】recall の分母を一度も測っていない。** P/R/F は未実装、recall 監査は自作 6 fixture のみ。
     god-object FN「実害ゼロ」は、定義上 god-object を含まない優良ライブラリ標本で不在を結論する
     サンプリングバイアス（しかも §2.3 自身が「真の god-object と facade は構造的に区別困難」と認めている）。

  → rev3 は **recall 測定を最優先**に据え、FP 緩和に走る前に「いま何件の真の違反を見逃しているか」を
  確定する。ISP は割引追加でなく検査対象の妥当性（C9）から問い直す。

### 0.2 既存資産（rev1 が見落としていたもの）

- `scripts/benchmark.sh`：cobra/gin/logrus/zap/fasthttp/bbolt をリリースタグ固定で clone し
  `gss -f json ./...` で採点、原則別 mean と最低スコア型を集計するコーパス + 校正ハーネス。
- `docs/scoring-analysis.md`：5 ラウンドの校正履歴（DIP/LSP/OCP/SRP の FP 修正と recall 監査）。
  本ツールの校正の一次資料であり、§0.1 の C1/C4 バイアスの証拠でもある。

---

## 1. 実態調査（実測） — precision 側の現状

### 1.1 計測
基準コミット `b7b2a97` で `scripts/benchmark.sh`（6 ライブラリ・258 targets）を実行。原則別 mean と
total 最下位型を取得。**この計測は precision 側（高得点であるべき型が低く出ていないか）しか見えない**
ことに注意（recall 側は §3 で別途測る）。

### 1.2 mean スコア

| Library | targets | SRP | OCP | LSP | ISP | DIP | total |
|---|--:|--:|--:|--:|--:|--:|--:|
| bbolt | 56 | 86.0 | 99.6 | 99.1 | 86.3 | 74.4 | 86.5 |
| cobra | 9 | 95.0 | 100 | 100 | 89.4 | 68.9 | 88.6 |
| fasthttp | 79 | 89.2 | 98.2 | 99.7 | 89.1 | 53.1 | 82.5 |
| gin | 50 | 90.5 | 99.2 | 99.7 | 93.2 | 85.2 | 91.9 |
| logrus | 9 | 75.1 | 96.1 | 100 | 77.8 | 57.9 | 77.0 |
| zap | 55 | 92.1 | 98.9 | 99.3 | 90.0 | 67.1 | 87.2 |

OCP/LSP はほぼ飽和（mean 96〜100）。残シグナルは ISP/DIP/SRP の中核型 low スコア。

### 1.3 low スコア型 — 「FP 候補」（断定しない）

| 型 | 原則 | 値 | 状態 |
|---|---|--:|---|
| zapcore.jsonEncoder / MapObjectEncoder | ISP | 0 | **FP 候補**（外部 Encoder 契約強制） |
| cobra.Command / gin.Engine・Context / fasthttp.Request系 / logrus.Logger | ISP | 0〜25 | **要判定**（C1: 権威バイアスで FP と即断しない） |
| bbolt.Tx, gin.Context 等 | DIP | 0 | **要精査**（具象結合 TP か取りこぼし FP か） |
| 各 facade | SRP | 51〜75 | 5 ラウンド目で中間帯へ |

rev2 はこれらを「FP 濃厚」と断じたが、rev3 では**ラベルを外部基準で確定するまで「候補」に留める**（§3.2）。
特に ISP は C9（検査対象が原則とズレている）の解決が先。

### 1.4 ISP low スコアの母数（参考値、判断根拠にはしない）
ISP ≤ 25 は 22/258 = 8.5%。ただし 258 は過少標本（§3.4）で、この比率を単独の意思決定根拠にはしない
（rev2 はこれを優先順位の主根拠にした＝C2 の precision 偏重）。

---

## 2. 課題の再定義（precision と recall の両面）

### 課題 A【最優先】recall を測れていない
真の SOLID 違反を何件見逃しているか（recall の分母）が未計測。自作 6 fixture の監査しかない。
**ここを測らずに FP を緩めると、検出器として劣化する**（C2/C4/C5）。
→ Phase 1 で recall 計測コーパスを構築する。

### 課題 B【ISP の根本】検査対象が原則とズレている（C9）
ISP は「struct の public メソッド数」を見ているが、原則は「太い interface の分割」。
afero.File 系・zap.jsonEncoder の ISP=0 を「FP」と片付ける前に、**そもそも何を ISP 違反と定義するか**を
決める。安易な外部契約割引は recall ガード `FatImpl` を壊す（C3）。

### 課題 C【DIP】facade の DIP=0 が TP か FP か未確定
fasthttp の DIP mean 53、複数 facade の DIP=0。ソース精査で TP/FP を確定してから手を入れる。

### 課題 D【中長期】独立加算の二重計上 / god-object 偽陰性
- 二重計上: SRP の avg-size attenuation（`srp.go:79-103`）は後付けパッチ。
- god-object FN: testdata `LargeFacade`（req 側 8 + resp 側 8 = **計 16 メソッド**、LCOM4=2）が
  模す構造の「真の god-object」。優良ライブラリ標本では観測されない（C5: 観測されない≠存在しない）。
→ 検出戦略（Marinescu）で対応するが、recall 計測で実在を確認してから（§5）。

### 2.1 マジックナンバー全量（中長期の閾値導出対象）

<details><summary>16 定数（クリックで展開、実装時はコードを正として再確認）</summary>

| Analyzer | 定数 | 値 | 出典 |
|---|---|---|---|
| SRP | LCOM4 base `at2/perGroup/maxPenalty` | 40/15/70 | `srp.go:153` |
| SRP | attenuation `loSize/hiSize/minAtten` | 3/10/0.25 | `srp.go:93` |
| SRP | complexity / method-count 境界 | >40→−20,>20→−10 / >15→−15,>10→−5 | `srp.go:123,132` |
| OCP | switch/assert/reflect 係数・cap / density | 15/10/5,40/40/20 / >0.3→−20,>0.15→−10 | `ocp.go:59-90` |
| LSP | panic/no-op/埋め込み | −20/−15/−10 | `lsp.go:56-77` |
| ISP | method 段階 / interface サイズ | ≤5/10/15/20→100/80/60/40/20 / ≤8/12/>12→10/20/30 | `isp.go:61,85` |
| DIP | param/owned weight, neutral, DI bonus | 0.3/1.0, 50, +15 | `dip.go:40-156` |

</details>

### 2.2 アーキテクチャ要点（実装制約）
- ペナルティは独立加算（`r.Score -= …` → `Clamp`）。
- 下流（scorer/differ/formatter）は `Result.Score`（float64）と Principles map の数値しか見ない
  → analyzer 内部の差し替えは出力契約を壊さない（検証済み）。
- `config.Thresholds` は `plugin/analysis.go:74,122` の合否ゲート専用。analyzer/scorer は参照しない。
  §2.1 の定数を config 化するには**全 5 analyzer のコンストラクタ配線変更**が要る（中長期の前提作業）。
- analyzer は 1 パッケージ単位の純関数（`Analyze(pkg)`、`analyzer.go:45`）。分布ベース正規化は 2 パス化が必要。
- ISP は `pkg.Structs` のみ走査し、`pkg.Interfaces` は struct 減点の修飾子にしか使わない（C9 の根拠）。

### 2.3 `Result.Confidence` の扱い（rev2 で落ちた論点を回復）
ISP/SRP は confidence を penalty ロジックと結合して算出（例: `isp.go:37,52`、`srp.go:114`）。
- aggregate total は confidence で重み付けされない（scoring-analysis.md:120）。
  → DIP-N/A 型（100/low-conf）が mean を嵩上げしており、§1.2 の mean を「精度の証拠」として
  過信しない（C8）。
- 検出戦略化（Phase 6）で penalty ロジックを変えると confidence の算出根拠が消える。
  **合成ルール下の confidence 再設計を Phase 6 の必須項目とする**（rev2 で抜けていた）。

### 2.4 片側バイアスの是正（C4 への構造的対策）
全校正が「penalty 緩和」一方向だった問題に対し、rev3 では各 Phase の完了条件に
**「recall 非回帰（真の違反の検出が減っていない）」を precision 改善と同格で必須化**する（§各 Phase）。
緩和とセットで、見逃しが増えていないことを毎回証明する。

---

## 3. Phase 1 — recall/precision 両面の測定基盤（最優先）

### 3.1 目的
「いま何件の真の違反を見逃し（FN）、何件を誤検出している（FP）か」を、**外部基準のラベル**で測る。
FP 緩和に着手する前提条件。

### 3.2 ラベルの間主観性を担保する（C6 への対策）
rev2 の「作者が fp/tp/na を付ける」は、作者の直感との一致度を測るだけ（トートロジー）。rev3 では:
- ラベル基準を**外部の SOLID 文献・公開 code smell データセット**（例: Lanza & Marinescu の God Class
  例題、公開 god class ラベル付きデータ）に紐づけ、作者の主観だけに依存させない。
- 可能なら**複数アノテータ + κ係数**で間主観一致度を報告。最低でもラベル根拠を型ごとに文章で残す。
- 「有名ライブラリだから高得点が正しい」を**ラベル基準にしない**（C1）。gin.Context のように
  著名でも批判のある型は、文献基準で是々非々に判定する。

### 3.3 recall コーパス（FN を測るための標本、C5 への対策）
優良ライブラリ標本は定義上 god-object 等を含みにくい。recall を測るには**意図的に違反を含む標本**が要る:
- 既存 `testdata/{principle}/bad.go` の violation 群（FatImpl 等）を recall の既知 TP として整備。
- 各原則の典型違反を**外部基準に基づいて追加合成**（自作 fixture は §3.2 の文献基準でラベル根拠を明記）。
- god-object の実在確認用に、レガシー寄り/大規模 OSS をコーパスに追加（優良 6 ライブラリと分離）。

### 3.4 統計的扱い
- 現コーパス 258 の low 帯は数十件規模 → P/R/F は**ブートストラップ信頼区間付き**で報告。
- train/test 分離: 校正に使った既知ケースと評価専用の未見ケースを分ける。ただし**ラベル者が同一なら
  prior leakage は残る**（C7）→ §3.2 の外部基準化で系統誤差を抑える。
- Alves 分位点導出（Phase 6/①）に進む場合のみ、多数 OSS で数千エンティティ規模に拡張。

### 3.5 完了の定義
`scripts/benchmark.sh --score`（仮）が原則別 **Precision・Recall・F1** を信頼区間付きで出力し、
現状ベースラインが記録される。**recall の分母（既知 TP 総数）が明示される。**

---

## 4. Phase 2 — ISP の検査対象の再定義（C9 を解く）

### 4.1 目的
ISP を「struct の public メソッド数」から、**原則に忠実な検査**へ寄せる。割引追加で糊塗しない。

### 4.2 検討事項（Phase 1 の recall/precision で判断）
- ISP 本来の対象は「太い interface 定義」。`pkg.Interfaces` を一級の検査対象にすべきか。
- 実装 struct を見る場合でも、「surface が広い」だけでなく「**クライアントが部分集合しか使わない**」
  証拠（呼び出し側の利用パターン）を要件にできるか。
- 外部契約実装（FatImpl / afero.File / zap.jsonEncoder）の扱い:
  **FatImpl を TP に保ったまま** afero.File を許容できる判別軸があるか（C3 の核心）。
  単純な「外部契約割引」は FatImpl も割り引くため不可。判別軸が見つからなければ、
  **両方 TP のまま（緩和しない）**を選ぶ — recall 優先。

### 4.3 完了の定義
- ISP の P/R が **両方** 改善（または precision 改善かつ recall 非回帰）。
- `FatImpl` が引き続き ISP 違反として捕捉される（recall ガード死守）。
- 「afero.File は許容、FatImpl は捕捉」を分ける根拠が文献基準で説明できる。できなければ緩和を見送る。

---

## 5. Phase 3 — DIP facade=0 の TP/FP 精査

DIP=0 の型（bbolt.Tx, gin.Context, fasthttp.Request系）のフィールド/コンストラクタを実ソースで確認し
TP/FP を台帳化。FP のみ手当て、TP（`BadService` fixture 等）は DIP=0 維持。
完了条件: DIP の P/R 両面が非回帰、FP 解消。

## 6. Phase 4 — SRP god-object FN の実在確認

Phase 1 で追加した recall コーパス（レガシー OSS 等）で、「LCOM4=2 かつ avg group size 大」で
attenuate された型を抽出し、文献基準で真の god-object か facade か判定。真の FN が一定数あれば
Phase 6（検出戦略）に進む根拠とする。**無ければ Phase 6 は見送り**（C5 を逆手に取り、実在を確認してから動く）。

---

## 7. Phase 5-6 — 中長期の構造改革（recall/precision の裏付け後）

研究的に妥当だが、Phase 1-4 の P/R/F で実害が裏付けられてから着手:

- **② 検出戦略（Marinescu）**: 独立加算 → AND 合成ルール。avg-size attenuation パッチ除去、
  god-object を「≥2 コンポーネントが各 N メソッド超 ∧ 互いに素なフィールド集合」で捕捉。
  ブール発火 → 連続 severity 写像が核心難所（候補: max/加算/重み付き、P/R/F で決定）。
  **合成ルール下の confidence 再設計を必須項目とする**（§2.3）。前提: §2.2 の config 配線変更。
- **① 分布閾値導出（Alves et al.）**: §2.1 の定数を SLOC 重み付き累積分布の分位点から導出。
  数千エンティティ規模のコーパスが前提。LCOM4 等の離散値メトリクスへの適用可否は要検証。
- **③ 語彙的コヒージョン（C3/LCCM）**: 推移 LCOM は現状 BFS 連結成分にほぼ含意済み（※根拠の精査は
  Phase 5 で：BFS は `shareField OR callsEachOther` の無向連結性で、フィールド共有の推移性とは厳密には
  非等価。実益が薄いという結論のみ暫定）。識別子・godoc のトークン類似度を②の合成ルールの一条件に。
  Jaccard は表層一致のみで語彙ゆれに弱い（C3 が LSI/LDA を使う理由）。
- **⑤ LLM ハイブリッド**: low-confidence のみ Claude に委譲する selective routing。
  `--llm-adjudicate` off で従来動作と完全一致。判定ばらつきは self-consistency で集約。
  `claude-api` スキルでモデル ID・料金確認。

---

## 8. baseline 非互換の移行設計

各 Phase でスコアが動き、CI diff（`solid-diff.yml`）が全 target を IMPROVED/REGRESSED として噴出させる。
- 各 Phase の PR で baseline JSON を再生成、リリースノートでツール起因の一斉変動を明示。
- 移行期に「ツール変更由来」と「コード変更由来」の差分が混在する問題に対し、差分抑制フラグまたは
  ツールバージョンを baseline に記録して差分理由を区別する仕組みを Phase 1 で具体化する
  （rev2 では「要否検討」止まりだった）。

---

## 9. 未決事項（実装着手前に詰める）

1. **ラベルの外部基準の選定**（§3.2）— どの文献/公開データセットを ground truth の根拠にするか。
   複数アノテータを確保できるか。**最重要**（C6/C7、設計の客観性の根幹）。
2. **ISP の検査対象再定義の方針**（§4.2）— interface を一級対象にするか、利用パターンを見るか。
3. **FatImpl と afero.File を分ける判別軸**（§4.2）— 見つからなければ ISP 緩和は見送り。
4. **recall コーパスの違反標本**（§3.3）— god-object 等を含むレガシー OSS の選定。
5. **baseline 差分理由の区別方式**（§8）。

---

## 10. 参考文献
- Alves, Ypma, Visser — Deriving Metric Thresholds from Benchmark Data (ICSM 2010)
- Marinescu — Detection Strategies / Lanza & Marinescu — Object-Oriented Metrics in Practice（God Class 例題）
- Marcus & Poshyvanyk — C3 / LCCM
- iSMELL (ASE 2024) / Beyond Strict Rules: LLMs for Code Smell Detection (2026)
- 一次資料: `docs/scoring-analysis.md`（5 ラウンド校正履歴、§0.1 のバイアス証拠）, `scripts/benchmark.sh`
