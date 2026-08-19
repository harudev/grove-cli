# /release

grove-cli GitHub Actions 릴리즈 워크플로우를 실행합니다.

## 사용법

```
/release --patch   # 패치 버전 올림 (버그 수정, 기본값)
/release --minor   # 마이너 버전 올림 (기능 추가)
/release --major   # 메이저 버전 올림 (Breaking Change)
/release           # 옵션 없으면 --patch 기본값 사용
```

## 절차

1. **인자 파싱**: `$ARGUMENTS`에서 `--patch` / `--minor` / `--major` 추출. 없으면 `patch` 기본값.

2. **현재 버전 확인** (로컬 태그가 아닌 리모트 릴리즈 기준)
   ```bash
   gh release list --repo harudev/grove-cli --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null || echo "v0.0.0"
   ```

3. **다음 버전 미리보기**: 현재 태그에서 bump level 적용 시 어떤 버전이 될지 계산해서 사용자에게 보여준다.
   - `vMAJOR.MINOR.PATCH` 파싱 후 해당 레벨 증가
   - major bump 시 minor, patch → 0 리셋
   - minor bump 시 patch → 0 리셋

4. **사용자 확인**: 다음과 같이 확인을 받는다.
   ```
   현재: v1.2.3
   다음: v1.2.4 (patch)
   워크플로우를 실행할까요? [y/N]
   ```

5. **워크플로우 실행**
   ```bash
   gh workflow run release.yml \
     --repo harudev/grove-cli \
     --field bump=<level>
   ```

6. **워크플로우 완료 대기**: 실행 직후 run ID를 가져와 완료될 때까지 폴링한다. 워크플로우 트리거 후 run이 등록되기까지 수 초 걸릴 수 있으므로 최대 10초 대기 후 ID 조회.
   ```bash
   # run ID 확인 (트리거 직후 등록 대기)
   sleep 3
   RUN_ID=$(gh run list --workflow=release.yml --limit=1 \
     --repo harudev/grove-cli \
     --json databaseId --jq '.[0].databaseId')

   # 완료까지 대기 (성공/실패 모두 종료)
   gh run watch "$RUN_ID" --repo harudev/grove-cli
   ```

7. **결과 안내**: 완료 후 성공/실패 여부와 릴리즈 URL 표시.
   ```bash
   # 결과 확인
   gh run view "$RUN_ID" --repo harudev/grove-cli \
     --json conclusion,url --jq '"결론: \(.conclusion)\nURL: \(.url)"'

   # 성공 시 새 릴리즈 확인
   gh release list --repo harudev/grove-cli --limit 1
   ```

## 주의사항

- `main` 브랜치에서만 실행 (`git branch --show-current` 확인)
- main이 아니면 경고 후 중단
- `gh` CLI가 인증된 상태여야 함 (`gh auth status`)
- 실행 전 `git status`로 uncommitted 변경사항 없는지 확인 (릴리즈는 클린 상태에서)
