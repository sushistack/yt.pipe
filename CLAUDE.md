# CLAUDE.md — yt.pipe (youtube.pipeline)

## 프로젝트 개요

SCP Foundation 유튜브 콘텐츠 자동 제작 파이프라인. SCP ID 입력 → 시나리오 → 이미지 → TTS → 자막 → CapCut 프로젝트 자동 조립.

- **핵심 철학**: "80% 자동화, 20% 수동 마무리"
- **모듈**: `github.com/sushistack/yt.pipe`
- **Go 1.25.7** / cobra CLI + chi REST API / SQLite / Docker

## 빌드 & 테스트

```bash
make build        # bin/yt-pipe 빌드
make test         # go test ./...
make lint         # go vet ./...
make run          # go run ./cmd/yt-pipe serve
make docker       # Docker 이미지 빌드
```

## 디렉토리 구조

```
cmd/yt-pipe/          # 엔트리포인트
internal/
  api/                # REST API 핸들러 (chi, auth middleware, webhook)
  cli/                # cobra CLI 커맨드
  config/             # 설정 로딩 (viper, 우선순위 체인)
  domain/             # 도메인 모델 (project, scene, scenario, manifest, job)
  glossary/           # SCP 용어 사전 (발음 교정, 자막 정확도)
  logging/            # 구조화 로깅 (JSON 포맷)
  pipeline/           # 오케스트레이션 (runner, checkpoint, incremental, dryrun, progress)
  plugin/             # 플러그인 시스템
    llm/              #   LLM (OpenAI 호환 + fallback chain)
    imagegen/         #   이미지 생성 (SiliconFlow FLUX)
    tts/              #   TTS (DashScope CosyVoice)
    output/capcut/    #   CapCut 프로젝트 조립
  retry/              # 재시도 로직
  service/            # 비즈니스 서비스 레이어
  store/              # SQLite 저장소
  template/           # 프롬프트 템플릿 관리
  workspace/          # 프로젝트 워크스페이스 관리
templates/            # 프롬프트 템플릿 파일 (.tmpl)
testdata/             # 테스트 데이터 (SCP-173 샘플)
```

## 아키텍처 핵심 원칙

- **Scene이 기본 처리 단위** — 이미지, 오디오, 자막, 메타데이터를 포함하는 자기 완결적 에셋 번들
- **플러그인 아키텍처** — LLM, TTS, ImageGen, Output 인터페이스 뒤에 구현체 배치
- **증분 빌드** — 변경된 씬만 재생성, 해시 기반 스킵 + 의존성 체인 무효화
- **CLI-API 이중 인터페이스** — 동일 서비스 레이어의 어댑터
- **설정 우선순위**: CLI 플래그 > 환경변수(YTP_) > 프로젝트 YAML > 글로벌 YAML > 기본값
- **상태 머신**: pending → scenario_review → approved → generating → complete

## 코드 컨벤션

- 테스트: `*_test.go` 같은 패키지, `testify` 사용
- 에러: `internal/domain/errors.go`의 도메인 에러 타입
- 외부 의존성은 반드시 인터페이스 뒤에 배치
- 로깅: `internal/logging` 구조화 로거

## 알려진 이슈

- `internal/service/assembler_test.go`가 `internal/mocks` 패키지를 참조하나 해당 패키지 없음 (테스트 실패)

---

# BMAD Method v6.0.4

이 프로젝트는 BMAD(Build Measure Analyze Decide) 방법론으로 관리된다.
BMAD 명령어가 호출되면, 해당 워크플로우/태스크 파일을 로드하고 그 안의 지시사항을 정확히 따라 실행한다.

## BMAD 설정

- **config**: `_bmad/core/config.yaml` (user_name: Jay, communication_language: Korean)
- **BMM config**: `_bmad/bmm/config.yaml` (project_name: youtube.pipeline, planning/implementation artifacts 경로)
- **워크플로우 엔진**: `_bmad/core/tasks/workflow.xml` — 모든 워크플로우 실행의 기반
- **산출물 위치**: `_bmad-output/planning-artifacts/`, `_bmad-output/implementation-artifacts/`

## BMAD 슬래시 명령어

아래 명령어를 사용자가 입력하면, 해당 workflow.yaml(또는 .md)을 로드하고 `_bmad/core/tasks/workflow.xml` 엔진에 따라 실행한다.

### 도움말 & 유틸리티
| 명령어 | 설명 | 파일 |
|--------|------|------|
| `/bmad-help` | 다음 단계 안내, 진행 상황 분석 | `_bmad/core/tasks/help.md` |
| `/bmad-brainstorming` | 브레인스토밍 세션 진행 | `_bmad/core/workflows/brainstorming/workflow.md` |
| `/bmad-party-mode` | 멀티 에이전트 토론 | `_bmad/core/workflows/party-mode/workflow.md` |
| `/bmad-index-docs` | 폴더 문서 인덱스 생성 | `_bmad/core/tasks/index-docs.xml` |
| `/bmad-shard-doc` | 큰 문서 분할 | `_bmad/core/tasks/shard-doc.xml` |
| `/bmad-editorial-review-prose` | 문체 리뷰 | `_bmad/core/tasks/editorial-review-prose.xml` |
| `/bmad-editorial-review-structure` | 구조 리뷰 | `_bmad/core/tasks/editorial-review-structure.xml` |
| `/bmad-review-adversarial-general` | 적대적 비판 리뷰 | `_bmad/core/tasks/review-adversarial-general.xml` |
| `/bmad-review-edge-case-hunter` | 엣지 케이스 탐색 | `_bmad/core/tasks/review-edge-case-hunter.xml` |

### 1단계: 분석 (Analysis)
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-market-research` | 📊 Mary (Analyst) | 시장 조사 | `_bmad/bmm/workflows/1-analysis/research/workflow-market-research.md` |
| `/bmad-bmm-domain-research` | 📊 Mary | 도메인 리서치 | `_bmad/bmm/workflows/1-analysis/research/workflow-domain-research.md` |
| `/bmad-bmm-technical-research` | 📊 Mary | 기술 리서치 | `_bmad/bmm/workflows/1-analysis/research/workflow-technical-research.md` |
| `/bmad-bmm-create-product-brief` | 📊 Mary | 프로덕트 브리프 작성 | `_bmad/bmm/workflows/1-analysis/create-product-brief/workflow.md` |

### 2단계: 기획 (Planning)
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-create-prd` | 📋 John (PM) | PRD 생성 | `_bmad/bmm/workflows/2-plan-workflows/create-prd/workflow-create-prd.md` |
| `/bmad-bmm-validate-prd` | 📋 John | PRD 검증 | `_bmad/bmm/workflows/2-plan-workflows/create-prd/workflow-validate-prd.md` |
| `/bmad-bmm-edit-prd` | 📋 John | PRD 수정 | `_bmad/bmm/workflows/2-plan-workflows/create-prd/workflow-edit-prd.md` |
| `/bmad-bmm-create-ux-design` | 🎨 Sally (UX) | UX 설계 | `_bmad/bmm/workflows/2-plan-workflows/create-ux-design/workflow.md` |

### 3단계: 솔루셔닝 (Solutioning)
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-create-architecture` | 🏗️ Winston (Architect) | 아키텍처 설계 | `_bmad/bmm/workflows/3-solutioning/create-architecture/workflow.md` |
| `/bmad-bmm-create-epics-and-stories` | 📋 John | 에픽/스토리 분해 | `_bmad/bmm/workflows/3-solutioning/create-epics-and-stories/workflow.md` |
| `/bmad-bmm-check-implementation-readiness` | 🏗️ Winston | 구현 준비도 점검 | `_bmad/bmm/workflows/3-solutioning/check-implementation-readiness/workflow.md` |

### 4단계: 구현 (Implementation)
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-sprint-planning` | 🏃 Bob (SM) | 스프린트 계획 | `_bmad/bmm/workflows/4-implementation/sprint-planning/workflow.yaml` |
| `/bmad-bmm-sprint-status` | 🏃 Bob | 스프린트 상태 확인 | `_bmad/bmm/workflows/4-implementation/sprint-status/workflow.yaml` |
| `/bmad-bmm-create-story` | 🏃 Bob | 스토리 생성 | `_bmad/bmm/workflows/4-implementation/create-story/workflow.yaml` |
| `/bmad-bmm-dev-story` | 💻 Amelia (Dev) | 스토리 구현 | `_bmad/bmm/workflows/4-implementation/dev-story/workflow.yaml` |
| `/bmad-bmm-code-review` | 💻 Amelia | 코드 리뷰 | `_bmad/bmm/workflows/4-implementation/code-review/workflow.yaml` |
| `/bmad-bmm-qa-automate` | 🧪 Quinn (QA) | QA 자동 테스트 생성 | `_bmad/bmm/workflows/qa-generate-e2e-tests/workflow.yaml` |
| `/bmad-bmm-retrospective` | 🏃 Bob | 회고 | `_bmad/bmm/workflows/4-implementation/retrospective/workflow.yaml` |
| `/bmad-bmm-correct-course` | 🏃 Bob | 방향 수정 | `_bmad/bmm/workflows/4-implementation/correct-course/workflow.yaml` |

### Quick Flow (간편 모드)
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-quick-spec` | 🚀 Barry (Solo Dev) | 빠른 기술 명세 | `_bmad/bmm/workflows/bmad-quick-flow/quick-spec/workflow.md` |
| `/bmad-bmm-quick-dev` | 🚀 Barry | 빠른 구현 | `_bmad/bmm/workflows/bmad-quick-flow/quick-dev/workflow.md` |

### 기타
| 명령어 | 에이전트 | 설명 | 파일 |
|--------|----------|------|------|
| `/bmad-bmm-document-project` | 📊 Mary | 프로젝트 문서화 | `_bmad/bmm/workflows/document-project/workflow.yaml` |
| `/bmad-bmm-generate-project-context` | 📊 Mary | project-context.md 생성 | `_bmad/bmm/workflows/generate-project-context/workflow.md` |

## BMAD 명령어 실행 규칙

1. 사용자가 위 명령어를 입력하면 해당 파일을 **즉시 전체 로드**한다
2. workflow.yaml인 경우: `_bmad/core/tasks/workflow.xml` 엔진 규칙에 따라 실행
   - config_source에서 변수 로드 (`_bmad/bmm/config.yaml`)
   - instructions 파일 로드 후 단계별 순차 실행
   - template-output 태그마다 저장 후 사용자 확인 대기
3. task(.md/.xml)인 경우: 파일 내 지시사항을 직접 따라 실행
4. **communication_language는 Korean** — 사용자와 한국어로 소통
5. **document_output_language는 English** — 생성 문서는 영어로 작성
6. 각 워크플로우는 **새 컨텍스트 윈도우**에서 실행하는 것을 권장 (컨텍스트 오버플로우 방지)

## BMAD 산출물 (참조 문서)

- **PRD**: `_bmad-output/planning-artifacts/prd.md` — 44 FR + 24 NFR
- **아키텍처**: `_bmad-output/planning-artifacts/architecture.md`
- **에픽/스토리**: `_bmad-output/planning-artifacts/epics.md` — 12개 에픽
- **구현 명세**: `_bmad-output/implementation-artifacts/` — 에픽별 상세 스펙 (60+ 파일)
  - 파일명 패턴: `{epic}-{story}-{title}.md` (예: `8-1-gemini-llm-provider-implementation.md`)
- **구현 준비도 리포트**: `_bmad-output/planning-artifacts/implementation-readiness-report-2026-03-08.md`

## BMAD 에이전트

에이전트 페르소나가 필요한 워크플로우에서 자동 로드됨:

| ID | 이름 | 역할 | 파일 |
|----|------|------|------|
| analyst | Mary | 비즈니스 분석가 | `_bmad/bmm/agents/analyst.md` |
| architect | Winston | 시스템 아키텍트 | `_bmad/bmm/agents/architect.md` |
| dev | Amelia | 시니어 개발자 | `_bmad/bmm/agents/dev.md` |
| pm | John | 프로덕트 매니저 | `_bmad/bmm/agents/pm.md` |
| qa | Quinn | QA 엔지니어 | `_bmad/bmm/agents/qa.md` |
| sm | Bob | 스크럼 마스터 | `_bmad/bmm/agents/sm.md` |
| tech-writer | Paige | 테크니컬 라이터 | `_bmad/bmm/agents/tech-writer/tech-writer.md` |
| ux-designer | Sally | UX 디자이너 | `_bmad/bmm/agents/ux-designer.md` |
| quick-flow-solo-dev | Barry | 퀵 플로우 개발자 | `_bmad/bmm/agents/quick-flow-solo-dev.md` |
| bmad-master | 🧙 BMad Master | 마스터 오케스트레이터 | `_bmad/core/agents/bmad-master.md` |
