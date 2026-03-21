# ghsummon 実装プラン

ファイル (当初は主にMarkdown内) の `@copilot <prompt>` パターンを検出し、GitHub Copilot Coding AgentにPR上で作業(リサーチなど)をさせるGo CLI + GitHub Action。

## 決定事項

- **ツール名**: `ghsummon` — ファイルのインライン指令からCopilot Coding Agentを「召喚」する
- **アーキテクチャ**: PR駆動（PRのみで完結、Issue不要）
- **初回コミット**: 空コミット（Git Data API経由）
- **リポジトリ**: `github.com/Songmu/ghsummon`
- **排他制御**: ブランチ名ベース（ファイル単位）。複数ファイルの並列PRはOK
- **参考アーキテクチャ**: tagpr (Go CLI + composite action)

## 全体フロー

```
1. ユーザーがファイル(Markdown)に `@copilot <prompt>` と記述してpush
2. GitHub Actions起動（push to default branch, *.md変更時）
3. Go CLIが shallow clone を検出した場合 `git fetch --deepen=1` を自動実行
4. git diffを解析、@copilotパターンを検出（複数行対応）
5. ブランチ `ghsummon-<filepath>` の存在チェック（排他制御）
   - 存在する場合 → スキップ（既に作業中）
5. Git Data APIで空コミット作成 → ブランチ作成
6. PR作成（タイトル: "ghsummon: <filepath>"、本文にプロンプト+指示）
7. copilot を PR の assignee に設定（GraphQL API + `GraphQL-Features: issues_copilot_assignment_api_support` ヘッダー）
8. PRに @copilot コメント投稿（プロンプト内容+指示テンプレート）
9. Copilot Coding Agentが起動、PRブランチに直接コミット追加
   - 対象ファイルの `@copilot <prompt>` 部分を調査結果で上書き
10. レビュー → マージ → ブランチ自動削除 → 同一ファイルの再作業可能に
```

## tagprパターン（参考アーキテクチャ）

tagprは「Go CLI + composite action」の3層アーキテクチャ。本ツールもこれに倣う。

### 3層構造

| 層                        | 役割                                                 |
| ------------------------ | -------------------------------------------------- |
| `cmd/ghsummon/main.go`   | 薄いエントリーポイント。`Run(ctx, args, stdout, stderr)` を呼ぶだけ |
| パッケージ本体 (`.go`)          | 全ロジック。テスタビリティ確保のため `io.Writer` を引数に取る              |
| `action.yml` (composite) | `install.sh` でプリビルドバイナリをDL → PATH追加 → 実行           |

### action.yml の設計（tagpr方式）

```yaml
runs:
  using: "composite"
  steps:
  - name: run
    run: |
      TEMP_PATH="$(mktemp -d)"
      PATH="${TEMP_PATH}:$PATH"
      curl -sfL "https://raw.githubusercontent.com/Songmu/ghsummon/${ACTION_REF}/install.sh" \
        | sh -s -- -b "$TEMP_PATH" "$VERSION" 2>&1
      ghsummon
    shell: bash
```

### GitHub API操作パターン

tagprはサーバーサイドGit Data APIでブランチを構築する:

```
1. 現在のHEAD SHAとtree SHAを取得
2. Git.CreateCommit でコミットオブジェクト作成（同じtree = 空コミット）
3. Git.CreateRef で refs/heads/<branch> を作成
4. PullRequests.Create でPR作成
```

メリット: `git push` 不要、credentials設定不要、`GITHUB_TOKEN` のみで完結。
`go-github` ライブラリ + `go-github-ratelimit` でレート制限自動待機。

### Actions連携

Go CLI内部から `GITHUB_OUTPUT` ファイルに書き込むことで後続ステップに値を渡す:

```go
func setOutput(name, value string) error {
    fpath := os.Getenv("GITHUB_OUTPUT")
    f, _ := os.OpenFile(fpath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
    defer f.Close()
    fmt.Fprintf(f, "%s=%s\n", name, value)
    return nil
}
```

## Copilot Coding Agent トリガー方法

### ハンドル

- アサイン/メンション: `copilot` (`@copilot`)
- Agentが作成したPRの作者: `copilot[bot]`（CI判定時に注意）

### 3つのトリガー方法

| 方法                                | 対象    | 結果                   | 本ツールでの利用  |
| --------------------------------- | ----- | -------------------- | --------- |
| **Issue assignee**                | Issue | 新しいPRが自動作成される        | フォールバック案  |
| **PR assignee + @copilot コメント**   | PR    | PRブランチに直接コミット追加      | **メイン方式** |
| **PRコメント `@copilot`（assigneeなし）** | PR    | 子PRを作成（元PRのブランチがベース） | 使用しない     |
| **PRレビュアー `copilot`**             | PR    | コードレビュー（変更なし）        | 使用しない     |

> **PR assignee vs コメントのみの違い（実測）**
>
> 公式ドキュメントでは `@copilot` コメントで「child PR」が作成されると記載されているが、
> PR の **assignee に `copilot` を設定** した上で `@copilot` コメントすると、
> **子PRではなくPRブランチに直接コミットされる**挙動となるので、これをメイン方式とする。

### ghsummon のトリガーフロー

```
1. PR作成（Git Data API）
2. copilot を PR の assignee に設定（GraphQL API）
3. PR に @copilot コメント投稿（指示テンプレート）
4. Copilot が PR ブランチに直接コミット
```

### PR assignee の設定（GraphQL API）

Copilot Coding Agent の assign には **user token**（PAT または GitHub App user-to-server token）が必要。GitHub App installation token や `GITHUB_TOKEN` では assign できない。

参考: [Assigning an issue to Copilot via the GitHub API](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/create-a-pr#assigning-an-issue-to-copilot-via-the-github-api)

> Make sure you're authenticating with the API using a user token, for example a personal access token or a GitHub App user-to-server token.

1. `suggestedActors(capabilities: [CAN_BE_ASSIGNED])` で `copilot-swe-agent` の node ID を取得
2. `addAssigneesToAssignable` mutation で PR に assign
3. リクエストには `GraphQL-Features: issues_copilot_assignment_api_support` ヘッダーが必須

### PRコメントでのトリガー（REST API）

```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/{owner}/{repo}/issues/{pr_number}/comments \
  -d '{"body":"@copilot このファイルを調査結果で更新してください"}'
```

> ⚠️ PR のコメントは Issues API を使う（GitHub内部ではPRはIssueの一種）

### トークン要件

| 方式                       | 必要な権限                                                      |
| ------------------------ | ---------------------------------------------------------- |
| **PAT (fine-grained)**（推奨） | `contents:write`, `pull_requests:write`    |
| **GitHub App Token**     | ⚠️ Copilot assign ができない（user token が必要） |
| **`GITHUB_TOKEN`**       | ⚠️ 再帰防止でCopilotトリガーが動かない                                   |

> Copilot Coding Agent の assign には **user token** が必要。GitHub App installation token では `suggestedActors` クエリが `copilot-swe-agent` を返さず、assign が機能しない。GitHub Actions 環境では **fine-grained PAT** を `secrets` に格納して使用する。

### 前提条件

- Copilotライセンス（Pro/Pro+/Business/Enterprise）— 個人アカウントでもOK（Orgは不要）
- Coding Agent有効化（個人: Settings > Copilot、Org: ポリシー設定）
- `.github/workflows/copilot-setup-steps.yml` が存在すること
- GitHub Actionsが有効

### `copilot-setup-steps.yml`

Copilot Coding Agentが作業を開始する前に実行される**環境セットアップ用ワークフロー**。デフォルトブランチの `.github/workflows/copilot-setup-steps.yml` に配置する。**このファイルが存在しないとCoding Agentは起動しない**。

- **ジョブ名は `copilot-setup-steps` 固定**（別名にするとCopilotが認識しない）
- 言語ランタイム、依存関係、環境変数、CLIツールなどを事前インストールできる
- セットアップステップが失敗してもCopilotは作業を続行する
- `workflow_dispatch` と自身のパス変更トリガーを入れておくと単体テストが容易

ghsummonの用途（主にMarkdownファイルの編集）では最小構成で十分:

```yaml
name: "Copilot Setup Steps"
on:
  workflow_dispatch:
  push:
    paths:
      - .github/workflows/copilot-setup-steps.yml
  pull_request:
    paths:
      - .github/workflows/copilot-setup-steps.yml

jobs:
  copilot-setup-steps:          # この名前は固定
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v5
```

> 参考: [Customizing the development environment for Copilot coding agent](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-environment)

## 関連プロダクト

| プロジェクト                                                                                | 関連性                                 |
| ------------------------------------------------------------------------------------- | ----------------------------------- |
| [Songmu/tagpr](https://github.com/Songmu/tagpr)                                       | Go CLI + composite actionのアーキテクチャ参考 |
| [peter-evans/create-pull-request](https://github.com/peter-evans/create-pull-request) | ファイル変更→PR自動生成のAction                |
| `.github/agents/*.agent.md`                                                           | Copilotカスタムエージェント定義（指示連携に活用可能）      |

## 設計詳細

### `@copilot <prompt>` パース仕様

```
パターン: /^@copilot (.+)$/
複数行: 次行がタブ or 2スペース以上インデントで始まる場合、プロンプト継続
```

例:
```markdown
@copilot GitHub Copilot Coding Agentのトリガー方法について調べて
  REST API、GraphQL、GitHub CLIそれぞれの方法を比較して
  ベストプラクティスもまとめてほしい
```

### ブランチ名正規化

ブランチ名: `ghsummon-<normalized-filepath>`

1. `filepath.Clean()` でパスを正規化（`./` 削除、`..` 解決、末尾 `/` 削除）
2. `filepath.ToSlash()` でパスセパレータを `/` に統一
3. 拡張子はそのまま保持（将来 `.md` 以外にも対応するため）
4. ブランチ名不安全文字（空白、`~`, `^`, `:`, `?`, `*`, `[`, `\` 等、非ASCII文字）を含む場合 → パス全体をMD5ハッシュ化

例:
- `notes/memo.md` → `ghsummon-notes/memo.md`
- `日本語ファイル.md` → `ghsummon-<md5hash>`

### Copilotへの指示テンプレート（PRコメント、英語ベース）

> [!note]
> ※ テンプレートのカスタマイズ機能は将来スコープ  
>  将来的には、ファイルタイプ毎に設定できるようにするなどするかも


```markdown
@copilot

Please replace the `@copilot` prompt line(s) in the following file
with your research results.

**Target file**: `<filepath>`

**Prompt**:
> <prompt全文>

**Instructions**:
- Remove the `@copilot` prompt line(s) (including continuation lines) and replace them with your research results
- Write the results in Markdown format
- Cite sources (URLs, etc.) for factual claims
- Preserve the existing file structure (heading levels, etc.)
- Search the web for up-to-date information when needed
```

## リポジトリ構成

```
github.com/Songmu/ghsummon/
├── cmd/ghsummon/main.go     ← 薄いエントリーポイント
├── ghsummon.go              ← CLI引数パース + コアロジック
├── parser.go                ← @copilot パターンのパース
├── parser_test.go
├── diff.go                  ← git diff解析・プロンプト検出
├── diff_test.go
├── github.go                ← GitHub API クライアント（go-github + GraphQL）
├── branch.go                ← ブランチ名生成・正規化
├── branch_test.go
├── action.yml               ← composite action定義
├── install.sh               ← バイナリDLスクリプト
├── go.mod / go.sum
├── Makefile
├── README.md
└── SKETCH.md                ← 設計メモ（本ファイル）
```

## 利用イメージ

```yaml
name: ghsummon
on:
  push:
    branches: [main]
    paths: ['**.md']

permissions:
  contents: write
  pull-requests: write

jobs:
  research:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          persist-credentials: false
      - uses: Songmu/ghsummon@v0
        with:
          token: ${{ secrets.GHSUMMON_TOKEN }}
```


## Appendix

### Web検索

Copilot Coding Agentは **2025年10月以降Web検索に対応**。リサーチ用途に活用できる。

- 個人アカウント（Pro/Pro+）ではデフォルト有効
- Org/Enterprise: "Copilot can search the web" ポリシーを有効化
- Agent自身が必要に応じて自動的にWeb検索を実行する（明示的な指示は不要だが、指示すると確実性が上がる）

### カスタムエージェント連携

Copilot Coding Agentは `.github/agents/*.agent.md` で定義された**カスタムエージェントを認識する**。

- リポジトリに `researcher.agent.md` があれば、指示テンプレートで `/researcher` デリゲーションを指定可能
- これによりリポジトリ固有のリサーチ品質向上が期待できる
- 本ツールの指示テンプレートでは、カスタムエージェントの存在を検知して自動参照する（将来スコープ）

### コスト管理

Copilot Agentは **Premium Requests + Actions Minutes** の両方を消費する。意図しない大量トリガーによるコスト増加に注意。排他制御が重要。
