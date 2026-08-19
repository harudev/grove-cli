# 🌳 grove (Git Worktree Manager)

Jira 이슈나 GitHub PR URL을 기반으로 Git 워크트리를 자동 생성하고 스마트하게 관리하는 CLI 도구입니다.



## ⚡ 빠른 시작 (Quick Start)

### 1. 설치
#### 1) 릴리즈 설치
```bash
curl -fsSL https://raw.githubusercontent.com/harudev/grove-cli/main/install.sh | bash
```

#### 2) 소스에서 직접 빌드
```bash
git clone https://github.com/harudev/grove-cli.git ~/projects/grove-cli
cd ~/projects/grove-cli
make install    # /usr/local/bin/grove
```

> Go 1.22+ 필요. `brew install go`로 설치 가능.


### 2. 초기 설정
설치 후 아래 명령어를 통해 필수 도구를 확인하고 기본 설정을 진행하세요.
```bash
grove setup                # 의존 도구 확인 및 쉘(ow) 함수 등록 / --install 파라미터 호출 시 필수 의존 도구 자동 설치 시도
grove config prefix PROJ # 이 레포의 기본 Jira 이슈 Prefix 설정 (레포별 저장)
grove config jira          # Jira API 토큰 설정 (선택)
```
> **Note:** `prefix`는 레포지토리별로 `<repo>/.grove/config.json`에 저장됩니다. 레포마다 다른 prefix를 쓸 수 있습니다.


### 3. 필수 디펜던시

| 도구 | 용도 | 설치 |
|------|------|------|
| `git` | 워크트리 관리 | `brew install git` |
| `gh` (GitHub CLI) | PR 연동, 릴리스 다운로드 | `brew install gh` |
| `jira-cli` | Jira 이슈 상태 조회 및 업데이트 | `brew tap ankitpokhrel/jira-cli & brew install jira-cli` |

> `grove setup --install`로 미설치 필수 도구를 자동 설치할 수 있습니다.
>
> `gh auth login` / `jira init`으로 각 도구 인증을 완료해야 Jira·GitHub 연동 기능을 사용할 수 있습니다.


## 주요 기능 1. 워크트리 관리

### 🌱 생성 (`init`)
다양한 방식으로 워크트리와 브랜치를 생성할 수 있습니다.

```bash
grove init PROJ-12345                       # 전체 이슈키 입력 (다른 Prefix도 가능)
grove init 12345 -t feature                 # 브랜치 타입 명시 (기본: feature)
grove init 12345 -t bugfix -b develop       # base 브랜치 직접 지정
grove init 12345 -t hotfix -b v1.2.0        # base 태그 직접 지정
grove init 12345 -t bugfix -b               # 값 없이 -b: base 를 목록에서 검색·선택

# URL 기반 생성
grove init https://your-jira.atlassian.net/browse/PROJ-12345
grove init https://github.com/org/repo/pull/456
```
> **Tip:** Jira 이슈 입력 시 이슈 내용을 워크트리에 저장할지 묻습니다. 수락 시 `.grove/{이슈키}.md`에 내용을 저장하고 `CLAUDE.local.md`에 참조를 추가합니다.
>
> **base 선택:** `-b/--base` 값을 주면 pin된 base보다 우선하며, 브랜치·태그 어느 쪽이든 지정할 수 있습니다. sub-feature 등 base가 고정되지 않은 타입은 프롬프트로 base를 선택합니다.

### 📋 조회 및 선택 (`list` & `select`)
```bash
grove list          # 생성된 워크트리 목록 조회 (테이블 형식)
grove list --json   # JSON 형식으로 출력

grove select 12345  # 특정 워크트리 경로로 이동 (shell function 용도)
```
> 동일한 이슈 번호의 워크트리가 여러 개일 경우, 대화형(Interactive) 선택 UI가 제공됩니다.

### 🧹 정리 (`clean`)
작업이 끝난 워크트리를 정리합니다.
```bash
grove clean             # 상태 확인 후 대화형으로 정리
grove clean PROJ-12345  # 특정 이슈 명시하여 정리
grove clean --done      # '리뷰통과' 이상 상태인 이슈 일괄 정리
grove clean --done -f   # 묻지 않고 강제 일괄 정리
```



## 주요 기능 2. Jira 연동 관리

Jira CLI와 연동하여 워크트리에서 바로 이슈 상태를 관리할 수 있습니다.
*사전 설정:*
1. `grove config jira` — API 토큰 등록
2. `grove config jira-workflow --scaffold` — 상태/전이 이름을 프로젝트에 맞게 매핑 ([워크플로우 설정](#5-jira-워크플로우-jira-workflow) 참고)

> Jira 워크플로우가 설정되지 않으면 `jira update/deploy/weekly`와 init 자동 전이는 비활성화됩니다 (워크트리 관리 기능은 그대로 동작).

### 🔄 상태 자동 동기화 (`jira update`)
PR 상태를 기반으로 Jira 이슈 상태를 동기화합니다. 아래 규칙의 각 상태/전이 이름은 `jira-workflow` 설정에 매핑한 값으로 동작합니다 (파이프라인 단계는 고정).
```bash
grove jira update          # 현재 워크트리 동기화
grove jira update --all    # 모든 워크트리 일괄 동기화 (옵션: --dry-run)
```
<details>
<summary><b>상태 전이 규칙 보기</b></summary>

파이프라인 단계: `inProgress` → `devComplete` → `inReview` / `reviewComplete` → `reviewPassed` → `resolvedClosed`

* **PR Open:** `devComplete` 이전 ➔ `devComplete`
* **PR Merged:** `inReview` 이전 ➔ `inReview`
* **PR Merged + noQA 라벨:** ➔ `reviewComplete`
* **배포일자 도래:** ➔ `resolvedClose` (미종료 자식 이슈가 있다면 확인 후 일괄 종료)
</details>

### 🚀 배포 관리 (`jira deploy`)
배포일자는 `jira-workflow`의 `deployField`(커스텀 필드 id)에 저장됩니다. 요일 제안은 `deployWeekday`(기본 화요일)를 따릅니다.
```bash
grove jira deploy                    # 다가오는 배포 요일 중 선택
grove jira deploy --date 2026-04-07  # 특정 일자 직접 지정
grove jira deploy --all              # 배포일 미설정 이슈 자동 일괄 설정
```

### 그 외 유틸리티 기능

| 명령어 | 설명 |
|---|---|
| `grove jira weekly` | 주간 진행 및 예정 이슈 현황 테이블 조회 |
| `grove jira status` | 현재 워크트리의 이슈 상태 조회 |
| `grove jira link [-o]` | Jira 이슈 URL 출력 ( `-o` 옵션 시 브라우저 오픈) |
| `grove jira qa -m "내용"` | QA 코멘트 추가 |
| `grove jira noqa` | PR에 noqa 라벨 추가 |
| `grove jira e2e` | E2E TC를 Jira 코멘트로 추가 |


## ⚙️ 상세 설정 (`config`)

설정은 두 곳에 나뉘어 저장됩니다. `grove config`로 현재 레포 기준 전체 설정을 조회할 수 있습니다.

| 위치 | 범위 | 내용 |
|---|---|---|
| `~/.config/grove/config.json` | 전역 | Jira 서버·로그인·API 토큰, Jira 워크플로우(`jiraWorkflow`) |
| `<repo>/.grove/config.json` | 레포별 | 이슈 prefix, 로컬 파일 패턴, post-checkout 명령어, 브랜칭 정책(`branchTypes`), Jira 워크플로우 오버라이드 |

> 레포별 설정은 커밋해 팀원과 공유할 수 있습니다.
> `jiraWorkflow`는 전역값을 기준으로, 레포별 값이 있으면 항목별로 덮어씁니다.

### 1. 이슈 prefix (`prefix`)
```bash
grove config prefix          # 현재 레포의 prefix 조회
grove config prefix PROJ   # 설정
```

### 2. 로컬 파일 자동 복사 (`local-files`)
`grove init` 시 Git에 트래킹되지 않는 환경 변수 파일 등을 메인 저장소에서 워크트리로 자동 복사합니다. (`filepath.Glob` 형식, `**` 글로브 지원)
```bash
grove config local-files --add ".env.local"
grove config local-files --remove ".env.local"
```

### 3. Post-checkout 스크립트 (`post-checkout`)
워크트리 생성 시 의존성 설치 후 자동 실행할 명령어를 설정합니다.
```bash
grove config post-checkout --add "pnpm run codegen"
grove config post-checkout --remove "pnpm run codegen"
```
> **실행 순서:** 패키지 매니저 install ➔ `.husky/post-checkout` (존재 시) ➔ 등록된 명령어 순차 실행

### 4. 브랜칭 정책 (`branch-types`)
이슈 타입별 브랜치 생성 정책(이름·prefix·전략·base)을 레포별로 관리합니다. 미설정 시 built-in 기본 정책을 사용합니다.
```bash
grove config branch-types             # 현재 정책 표로 조회
grove config branch-types --scaffold  # 기본 정책을 config.json에 풀어써서 직접 편집

# 타입별 브랜치 이름 prefix ({prefix}/{이슈키})
grove config branch-prefix -a feature feat   # 추가: feature → feat/PROJ-123
grove config branch-prefix -d feature        # 삭제 (기본값으로 복원)

# 타입별 base 브랜치/태그 고정 (고정 시 init에서 프롬프트 없이 해당 ref 사용)
grove config base-branch -a feature develop  # feature base를 develop으로 고정
grove config base-branch -a hotfix v1.2.0    # hotfix base를 태그로 고정
grove config base-branch -d feature          # 고정 해제
```

### 5. Jira 워크플로우 (`jira-workflow`)
grove의 이슈 파이프라인을 Jira 프로젝트의 실제 상태·전이 이름에 매핑합니다. 파이프라인 단계는 고정이고, 각 단계에 대응하는 **상태 이름·전이(액션) 이름**을 여러분의 Jira에 맞게 채워 넣는 방식입니다.
```bash
grove config jira-workflow             # 현재 매핑 조회
grove config jira-workflow --scaffold  # 전역 config.json에 예시 템플릿 작성
grove config jira-workflow --scaffold --repo  # 레포별 config.json에 작성
```
템플릿을 작성한 뒤 `config.json`의 `jiraWorkflow`를 직접 편집합니다.

| 항목 | 설명 |
|---|---|
| `statuses` | 각 단계의 Jira 상태 이름 (`inProgress`, `devComplete`, `inReview`, `reviewComplete`, `reviewPassed`, `resolvedClosed`) + 종료 판정용 `terminal[]` |
| `transitions` | 각 단계로의 전이(액션) 이름. Jira 이슈의 전이 메뉴에 뜨는 이름과 **정확히** 일치해야 합니다 |
| `deployField` | 배포일자 커스텀 필드 id (예: `customfield_11642`). 비우면 배포 기능 비활성화 |
| `deployWeekday` | 배포일 제안 요일 (기본 `Tuesday`, 한글 요일도 허용) |
| `excludedStatuses` | `jira weekly`에서 제외할 상태 (예: 보류/대기) |

> 값이 빈 단계는 "이 프로젝트 워크플로우에 없는 단계"로 간주되어 해당 전이를 건너뜁니다. 종료 상태는 `terminal` 목록 외에 Jira의 `statusCategory=done`도 항상 종료로 취급합니다.

### 6. 의존 도구 관리
```bash
grove setup --install    # 미설치 필수 도구(git, gh, jira-cli) 자동 설치
```



## 📂 브랜치 및 디렉토리 구조

브랜칭 정책은 데이터 기반 `branchTypes` 모델로, 레포별로 커스터마이즈할 수 있습니다 (`grove config branch-types` 참고). 아래는 built-in 기본 정책입니다.

### 브랜치 전략 (`strategy`)
| 전략 | 동작 |
|------|------|
| `from-default` | 레포 기본 브랜치에서 딴다 |
| `from-branch` | 지정/선택한 브랜치에서 딴다 (base 비면 검색·선택) |
| `from-tag` | 지정/선택한 태그에서 딴다 (base 비면 검색·선택) |
| `checkout-existing` | 기존 리모트 브랜치를 체크아웃 (새 브랜치 생성 안 함) |

### 기본 브랜치 타입
| 타입 (`-t`) | 브랜치 형식 | 전략 |
|------|------------|------|
| **feature** | `feature/{이슈키}` | `from-default` |
| **big-feature**| `big-feature/{이슈키}` | `from-default` |
| **sub-feature**| `sub-feature/{이슈키}` | `from-branch` |
| **bugfix** | `bugfix/{이슈키}` | `from-branch` |
| **hotfix** | `hotfix/{이슈키}` | `from-tag` |
| **local-review**| `local-review/{이슈키}` | `checkout-existing` |

> `branch-prefix`로 브랜치 이름 prefix를, `base-branch`로 base ref를 타입별로 고정할 수 있습니다.

### 디렉토리 구조
워크트리는 메인 저장소와 동일한 레벨의 `-worktrees` 폴더 하위에 격리되어 관리됩니다.
```text
~/projects/
├── {repo-name}/                    # 메인 저장소 (Base)
└── {repo-name}-worktrees/
    ├── PROJ-12345/                 # 개별 워크트리 1
    ├── PROJ-12346/                 # 개별 워크트리 2
    └── ...
```



## 🛠 기타 명령어 및 개발자 가이드

### 업데이트
```bash
grove update         # 최신 버전으로 업데이트
grove version        # 현재 버전 확인
```
> 명령 실행 후 새 버전이 있으면 자동으로 알림을 표시합니다.
