---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: 'complete'
completedAt: '2026-03-08'
lastIncrementalUpdate: '2026-03-18'
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/prd-validation-report.md
  - _bmad-output/brainstorming/brainstorming-session-2026-03-07-1200.md
  - _bmad-output/planning-artifacts/prd-enhancement.md
  - _bmad-output/brainstorming/brainstorming-session-2026-03-17-1000.md
  - _bmad-output/planning-artifacts/brainstorming-feasibility-analysis-2026-03-18.md
workflowType: 'architecture'
project_name: 'youtube.pipeline'
user_name: 'Jay'
date: '2026-03-07'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
61개 FR이 14개 카테고리로 구성. 핵심은 SCP ID 입력 → CapCut 프로젝트 출력의 end-to-end 파이프라인이며, CLI + REST API 이중 인터페이스로 동일한 서비스 레이어를 공유한다. 씬(Scene)이 파이프라인의 기본 처리 단위이자 자기 완결적 에셋 번들(이미지, 오디오, 자막, 메타데이터)로, 증분 빌드와 부분 재생성의 근간이 된다. FR45-61(2026-03-09 추가)은 프롬프트 템플릿 관리, 캐릭터 ID카드, TTS 분위기 프리셋, 씬별 승인 워크플로우, BGM 프리셋 라이브러리, 프롬프트 마이그레이션 6개 영역을 포함한다.

**Non-Functional Requirements:**
24개 NFR이 7개 카테고리로 구성. 아키텍처를 형성하는 핵심 NFR: 플러그인 인터페이스 표준화(NFR9), 모듈 간 결합도 최소화(NFR20), 기존 코드 변경 없는 플러그인 추가(NFR21), 외부 API 없이 테스트 가능(NFR23), Docker 패키징(NFR13), 비정상 종료 시 데이터 무결성(NFR8).

**PRD 구현 누출 → 아키텍처 결정 추적:**
PRD 검증에서 지적된 7건의 구현 누출(FR5 팩트 태깅 포맷, NFR9 어댑터 인터페이스, NFR20/21 어댑터 패턴, NFR23 Mock 구현 등)은 이 아키텍처 문서에서 정식 결정으로 수용하거나 대안을 제시할 예정이다.

**Scale & Complexity:**

- Primary domain: CLI Tool + API Backend (워크플로우 오케스트레이션)
- Complexity level: Medium
- Estimated architectural components: ~17 (FR45-61 추가로 4개 증가)
- 가장 복잡한 컴포넌트: 파이프라인 오케스트레이터 — 상태 머신, 증분 빌드 판단, 단계 간 의존성 관리, 체크포인트를 모두 담당하며 아키텍처에서 가장 신중하게 다뤄져야 할 부분

### Technical Constraints & Dependencies

- **외부 AI API 의존** — LLM(시나리오), 이미지 생성(SiliconFlow 기본), TTS 3종의 외부 API. 네트워크 필수, 각각 독립적 장애 가능
- **CapCut 비공식 포맷** — 공식 API/문서 없음. 기존 video.pipeline의 검증된 템플릿(버전 360000/151.0.0) 기반 역공학. 업데이트 시 호환성 깨질 수 있음
- **CapCut 포맷 취약성** — 출력 조립을 추상화 계층 뒤에 배치하여 포맷 변경 시 대체 구현으로 전환 가능하게 설계. MVP 전 PoC 검증 게이트 필수
- **SCP 구조화 데이터** — 422개 초기 데이터(facts.json, meta.json, main.txt). 스키마 버전 관리 필요
- **1인 개발/운영** — 개인 홈서버 Docker 배포, 복잡한 인프라 불필요
- **기존 에셋 마이그레이션** — video.pipeline의 프롬프트 라이브러리, CapCut 템플릿, Frozen Descriptor 패턴 활용

### Cross-Cutting Concerns Identified

1. **에러 처리 & 재시도 전략** — 외부 API 호출 전반에 일관된 재시도(최대 3회, 점진적 지연), 실패 항목만 선택적 재시도
2. **구조화된 로깅** — JSON 포맷, 모든 모듈 공통, n8n 파싱 호환
3. **설정 우선순위 체인** — CLI 플래그 > 환경변수 > 프로젝트별 YAML > 글로벌 YAML > 기본값. 부트스트랩 시 1회 확정, 이후 불변 원칙
4. **프로젝트 격리** — `project/{scp-id}-{timestamp}/scenes/{scene-num}/` 구조로 산출물 격리
5. **용어 사전 시스템** — SCP 전문 용어 사전이 TTS 발음 교정, 자막 정확도, 시나리오 생성에 횡단 적용되는 진정한 교차 관심사
6. **데이터 무결성** — 비정상 종료 시 기존 데이터 손상 방지, 단계별 체크포인트
7. **플러그인 수명주기** — 로딩, 검증, 설정, 실행의 일관된 플러그인 관리
8. **이벤트 발행/구독** — 프로젝트 상태 변경 시 웹훅 알림(FR30) + 폴링(GET /api/jobs) 동시 지원을 위한 이벤트 발행 메커니즘
9. **테스트 가능성을 위한 의존성 역전** — NFR23 충족을 위해 모든 외부 의존성(LLM, TTS, 이미지 생성, CapCut 포맷)이 인터페이스 뒤에 위치
10. **API 호출 추적** — 모든 외부 API 호출 시 estimated cost 필드를 로그에 포함. MVP에서는 로그 수준, Phase 3에서 대시보드로 확장
11. **비동기 작업 수명주기** — 장시간 작업(시나리오/이미지/TTS/조립)의 생성 → 진행 → 완료/실패 → 조회 패턴을 표준화. 폴링과 웹훅을 동시 지원
12. **입력 데이터 검증** — 모든 외부 입력(SCP 데이터, LLM 출력, 이미지 생성 결과)에 대해 로딩 성공과 데이터 유효성을 분리 검증. 유효하지 않은 데이터의 다음 단계 진입 차단

### Architectural Notes

- **팩트 검증 시스템**은 교차 관심사가 아닌 시나리오 생성 모듈 내부의 품질 게이트로 분류. 용어 사전과 분리하여 모듈 경계를 명확히 유지
- **씬(Scene)은 자기 완결적 에셋 번들** — 이미지, 오디오 세그먼트, 자막 세그먼트, 메타데이터를 포함하는 독립 단위. 이 개념이 증분 빌드, 부분 재생성, 프로젝트 저장 구조의 근간
- **오케스트레이션 패턴 후보:** 상태 머신 + 커맨드 패턴 (단계 순차, 씬 병렬). PRD 상태 머신(FR22)과 자연스럽게 정렬
- **플러그인 설계 수준 후보:** 공통 기반(초기화, 검증, 재시도, 타임아웃) + 타입별 특화 인터페이스. 핫스왑 제외, 설정 변경 후 재시작 방식
- **데이터 무결성 전략 후보:** 임시파일 + 원자적 rename(파일 단위) + 씬별 매니페스트(진행 상태). 매니페스트가 증분 빌드 판단 근거로 이중 활용
- **시나리오 출력 구조화** — LLM 시나리오는 자유 형식이 아닌 씬 단위로 분할 가능한 구조화된 포맷이어야 함. 이후 이미지 프롬프트 생성, 씬 분할, 타이밍 계산의 안정성 기반
- **타이밍 데이터는 파이프라인의 척추** — TTS 오디오 길이와 워드 타이밍이 이미지 전환, 자막 동기화, CapCut 타임라인을 결정. 타이밍 해석기(Timing Resolver) 컴포넌트로 분리하여 TTS 플러그인 변경 시에도 소비자 영향 차단
- **씬 의존성 체인** — 씬 내 에셋 간 의존성(시나리오 → 이미지 프롬프트 → 이미지, 시나리오 → TTS → 자막 타이밍 → 조립)을 매니페스트에 기록. 상위 변경 시 하위 자동 무효화로 증분 빌드 정합성 보장
- **프롬프트 안전화(sanitization)** — SCP 도메인의 공포/폭력 요소로 이미지 생성 NSFW 필터링 빈발 가능. 프롬프트 전처리 단계에서 안전 수식어 적용 고려
- **품질의 다차원성** — 팩트 정확도(자동 검증), 시각적 일관성(반자동), 시나리오 품질(수동 리뷰), TTS 자연스러움(수동 리뷰). "80% 자동, 20% 수동"의 경계를 아키텍처가 인식
- **CLI-API 차이의 정확한 이해** — 동등한 1급 시민이 아닌, 동일 서비스 레이어의 다른 어댑터. 서비스 레이어는 동기 + 진행률 이벤트 발행, API 어댑터가 비동기 job으로 래핑. 진행률 옵저버 인터페이스 필요
- **씬 모델(Scene Model)이 공유 도메인 모델** — 각 파이프라인 단계가 씬 모델에 데이터를 점진적으로 추가하는 파이프-필터 패턴. 씬 모델이 매니페스트와 결합하여 증분 빌드 판단 근거로 활용
- **시나리오 출력 스키마가 모듈 간 계약** — 시나리오의 구조화된 출력(narration, visualDescription, factTags, mood)이 이미지 프롬프트, TTS, 팩트 검증, 자막 4개 소비자의 입력 계약
- **검증 게이트 인터페이스** — 단계 간 검증 게이트가 공통 인터페이스(validate() → pass/fail/warn)를 따르되, 구현은 단계별 특화(스키마 검증, 팩트 커버리지, 산출물 무결성)
- **스타일 설정의 횡단 영향** — 글로벌 스타일 프리셋이 이미지 프롬프트, TTS, CapCut, 시나리오 4개 모듈에 영향. 설정 구조에 style 네임스페이스 확보
- **이미지 생성과 TTS는 순차 실행으로 변경** — FR54-55 씬별 승인 워크플로우 도입으로 이미지 생성(image_review) → TTS 생성(tts_review) 순차 실행. 각 단계에서 크리에이터가 씬 단위로 승인/재생성을 판단하므로 병렬 실행 불가. 상태 머신: `approved → image_review → tts_review → assembling → complete`
- **씬 모델 MVP 단순화** — MVP에서 1씬 = 1이미지 + 1나레이션 구간. Phase 2 확장을 위해 씬 모델에 확장 포인트(예: imageCount) 보존
- **플러그인 4종** — LLM, TTS, ImageGen, OutputAssembler. CapCut은 OutputAssembler의 기본 구현체, FFmpeg/JSON 타임라인이 대체 구현
- **MVP 동시성 제약** — MVP는 단일 파이프라인 실행만 보장. 동시 실행은 Phase 2 배치 프로세싱에서 해결. n8n에서 동시 트리거 시 큐잉 또는 거부 정책 필요

## Starter Template Evaluation

### Primary Technology Domain

CLI Tool (primary) + API Backend (secondary), 워크플로우 오케스트레이션 기반. Go 언어 선택.

### Technical Preferences

- **Language:** Go — 단일 바이너리 배포, 네이티브 동시성(goroutine), Docker 친화적(scratch 베이스 가능), 강타입
- **Database:** SQLite (파일 기반) — modernc.org/sqlite (CGO-free, Pure Go)
- **Deployment:** Docker + docker-compose, 개인 홈서버

### Starter Options Considered

| Framework | Category | Considered | Decision | Rationale |
|-----------|----------|:----------:|:--------:|-----------|
| Cobra | CLI | Yes | Selected | 중첩 서브커맨드, Viper 통합, kubectl/docker/helm 검증 |
| urfave/cli | CLI | Yes | Rejected | 복잡한 서브커맨드 구조에 약함 |
| Chi | API Router | Yes | Selected | net/http 100% 호환, 클린 아키텍처 친화, 경량 |
| Gin | API Framework | Yes | Rejected | 자체 Context, 서비스 레이어 프레임워크 종속 위험 |
| Fiber | API Framework | Yes | Rejected | fasthttp 기반(net/http 비호환), 표준 미들웨어 불가 |
| Echo | API Framework | Yes | Rejected | 불필요한 무게, API가 부차적 인터페이스 |
| mattn/go-sqlite3 | DB Driver | Yes | Rejected | CGO 필요, Docker 빌드 복잡화 |
| modernc.org/sqlite | DB Driver | Yes | Selected | CGO-free, scratch 베이스 Docker 가능 |
| Ginkgo | Testing | Yes | Rejected | BDD 스타일, 1인 프로젝트에 과잉 |
| testify | Testing | Yes | Selected | assert/mock 제공, 생태계 최대 |

### Selected Stack

**Initialization Command:**

```bash
mkdir youtube-pipeline && cd youtube-pipeline
go mod init github.com/jay/youtube-pipeline
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/go-chi/chi/v5@latest
go get modernc.org/sqlite@latest
go get github.com/stretchr/testify@latest
go install github.com/vektra/mockery/v2@latest
```

**Architectural Decisions Provided by Stack:**

**Language & Runtime:**
- Go (latest stable) with modules
- 단일 바이너리 컴파일, CGO 불필요
- goroutine 기반 씬 병렬 처리

**CLI Framework (Cobra + Viper):**
- 중첩 서브커맨드: `yt-pipe run`, `yt-pipe scenario generate/approve`, `yt-pipe image generate --scene 3,5`
- Viper 네이티브 5단계 설정 우선순위: CLI 플래그 > 환경변수(`YTP_` prefix) > 프로젝트 YAML > 글로벌 YAML(`$HOME/.yt-pipe/`) > 기본값
- 자동 help/completion 생성, 종료 코드 규약(0/1/2/3)

**API Router (Chi):**
- net/http 완전 호환 — `http.Handler`/`http.HandlerFunc` 직접 사용
- 표준 `httptest` 패키지로 API 테스트, 프레임워크 전용 테스트 유틸리티 불필요
- 모든 net/http 미들웨어 호환 (인증, CORS, 로깅)
- 서비스 레이어가 프레임워크에 종속되지 않음

**Database (modernc.org/sqlite):**
- CGO-free Pure Go — Docker 멀티스테이지 빌드에서 `FROM scratch` 가능
- 프로젝트 상태 머신, job 큐, 설정 캐시 등 상태/메타데이터 저장
- 에셋 파일은 파일시스템에 별도 관리

**Structured Logging (log/slog):**
- Go 1.21+ 표준 라이브러리 — 외부 의존성 없이 JSON 구조화 로깅
- NFR19(JSON 포맷, n8n 파싱 호환) 충족

**Testing (testing + testify + mockery):**
- 표준 `testing` 패키지 + testify assert/mock
- `mockery` 도구로 Go 인터페이스에서 mock 자동 생성 (`go generate`)
- 플러그인 4종 + 서비스 인터페이스 mock 자동화

**Code Organization:**

```
youtube.pipeline/
├── cmd/
│   └── yt-pipe/
│       └── main.go              # 진입점
├── internal/
│   ├── cli/                     # Cobra 커맨드 정의
│   │   ├── root.go
│   │   ├── run.go
│   │   ├── scenario.go
│   │   ├── image.go
│   │   ├── tts.go
│   │   ├── assemble.go
│   │   ├── status.go
│   │   └── init_cmd.go
│   ├── api/                     # Chi 라우터 + 핸들러
│   │   ├── server.go
│   │   ├── routes.go
│   │   ├── handlers/
│   │   └── middleware/
│   ├── service/                 # 코어 서비스 레이어 (CLI/API 공유)
│   │   ├── pipeline.go          # 오케스트레이터
│   │   ├── scenario.go
│   │   ├── image.go
│   │   ├── tts.go
│   │   ├── subtitle.go
│   │   ├── assembler.go
│   │   └── timing.go           # 타이밍 해석기
│   ├── domain/                  # 도메인 모델
│   │   ├── scene.go             # 씬 모델 (공유 도메인 모델)
│   │   ├── project.go           # 프로젝트 상태 머신
│   │   └── manifest.go          # 씬 매니페스트
│   ├── plugin/                  # 플러그인 인터페이스 + 구현
│   │   ├── base.go              # 공통 기반 인터페이스
│   │   ├── llm/
│   │   ├── tts/
│   │   ├── imagegen/
│   │   └── output/              # OutputAssembler (CapCut, FFmpeg)
│   ├── config/                  # Viper 설정 관리
│   ├── store/                   # SQLite 기반 상태/메타데이터 저장
│   │   ├── store.go             # DB 초기화, 마이그레이션
│   │   └── project.go           # 프로젝트 상태, job 상태
│   ├── workspace/               # 파일시스템 기반 에셋 관리
│   │   ├── project.go           # 프로젝트 디렉토리 구조
│   │   ├── scene.go             # 씬별 에셋 읽기/쓰기
│   │   └── manifest.go          # 매니페스트 파일 관리
│   ├── validation/              # 검증 게이트 인터페이스 + 구현
│   ├── event/                   # 이벤트 발행/구독
│   └── glossary/                # SCP 용어 사전
├── config.example.yaml
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

**Development Experience:**
- `go run ./cmd/yt-pipe` — 빠른 개발 실행
- `go test ./...` — 전체 테스트
- `go generate ./...` — mockery mock 자동 생성
- Docker 멀티스테이지 빌드 — 최소 이미지 크기

**Note:** 프로젝트 초기화는 첫 번째 구현 스토리로 실행.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- SQLite Option B (aggressive): 프로젝트 상태 + 씬 매니페스트 + 실행 이력 + 비용 로그 통합
- Job 테이블 기반 비동기 작업 관리 (Option A)
- 플러그인 4종 인터페이스 정의 (LLM, TTS, ImageGen, OutputAssembler)
- 상태 머신 오케스트레이션 (pending → scenario_review → approved → image_review → tts_review → assembling → complete) — FR22 확장: `image_review`, `tts_review` 상태 추가로 씬별 승인 워크플로우 지원

**Important Decisions (Shape Architecture):**
- Store(SQLite 메타데이터) / Workspace(파일시스템 에셋) 분리
- API 에러 응답 형식 (PRD 정의 구조)
- API key 인증 (X-API-Key 헤더)
- 스키마 마이그레이션 (store.go 내 버전 체크 함수)

**Deferred Decisions (Post-MVP):**
- CI/CD 파이프라인 → Phase 2
- 동시 파이프라인 실행 → Phase 2 배치 프로세싱
- 비용 대시보드 → Phase 3 (MVP는 slog 로그)
- 이벤트 pub/sub 시스템 → Phase 2 웹훅 알림
- 검증 게이트 공통 인터페이스 → 패턴 확정 후 추출
- 스타일 프리셋 시스템 → 하드코딩 기본값으로 시작

### Data Architecture

- **Database:** SQLite Option B (aggressive) — modernc.org/sqlite (CGO-free)
  - 저장 대상: 프로젝트 상태, job 큐, 씬 매니페스트, 실행 이력, API 비용 로그
  - Rationale: 1인 프로젝트에서 단일 DB 파일로 통합이 오히려 단순화
- **Storage Split:** `store/` (SQLite 메타데이터) + `workspace/` (파일시스템 에셋)
- **Schema Migration:** `store.go` 내 버전 체크 + SQL 실행 함수 (별도 프레임워크 불필요)
  - `go:embed`로 SQL 파일 임베드, `schema_version` 테이블로 버전 추적
- **SCP Data:** 읽기 전용 외부 볼륨, 직접 파일시스템 읽기

### Authentication & Security

- **API 인증:** X-API-Key 헤더, Chi 미들웨어 1개로 구현
- **바인딩:** localhost:8080 기본 (홈서버 내부망)
- **시크릿 보호:** slog에서 API 키/토큰 자동 필터링
- **Rationale:** 1인 홈서버 운영, 최소한의 보안으로 충분

### API & Communication Patterns

- **비동기 작업:** SQLite Job 테이블 — row insert → goroutine 실행 → status UPDATE
  - 폴링: `GET /api/jobs/{id}` (SELECT)
  - 별도 pub/sub 추상화 없음, 서비스 레이어 직접 호출
- **응답 형식:** PRD 정의 구조 (`{ success, data, error: { code, message } }`)
- **CLI 출력:** JSON stdout, 진행률 stderr
- **에러 코드:** PRD FR33 정의 표준 에러 코드 적용

### Infrastructure & Deployment

- **Docker:** multi-stage 빌드 (golang → scratch), 단일 바이너리
- **docker-compose:** 볼륨 마운트 4개
  - `/data/raw` — SCP 원본 데이터 (읽기 전용)
  - `/data/projects` — 프로젝트 워크스페이스
  - `/data/db` — SQLite DB 파일
  - `/config` — YAML 설정
- **CI/CD:** MVP 불필요, `go build` + `docker build` 수동 실행
- **모니터링:** slog JSON 로그 → n8n 파싱 (별도 모니터링 스택 없음)

### Cross-Cutting Concerns (7개 — MVP 최적화)

| # | 관심사 | 구현 방식 |
|---|--------|-----------|
| 1 | 에러 처리 & 재시도 | 외부 API: 최대 3회, exponential backoff. 공통 retry 헬퍼 함수 |
| 2 | 구조화 로깅 | log/slog JSON 포맷. API cost 필드 포함 |
| 3 | 설정 우선순위 | Viper 네이티브: CLI > env(`YTP_`) > project YAML > global YAML > defaults |
| 4 | 프로젝트 격리 | `workspace/{scp-id}-{timestamp}/scenes/{num}/` |
| 5 | 용어 사전 | TTS 발음, 자막, 시나리오에 횡단 적용 |
| 6 | 데이터 무결성 | temp file + atomic rename + 씬 매니페스트 |
| 7 | 의존성 역전 | 플러그인 4종 + 서비스 인터페이스 mock 자동 생성 |

### Simplified Package Structure (9개)

```
internal/
├── cli/          # Cobra 커맨드
├── api/          # Chi 라우터 + 핸들러 + 미들웨어
├── service/      # 코어 서비스 레이어 (검증 로직 포함)
├── domain/       # 씬 모델, 프로젝트 상태, 매니페스트
├── plugin/       # 플러그인 인터페이스 + 구현
├── config/       # Viper 설정 관리
├── store/        # SQLite 저장소 + 마이그레이션
├── workspace/    # 파일시스템 에셋 관리
└── glossary/     # SCP 용어 사전
```

### Decision Impact Analysis

**Implementation Sequence:**
0. CapCut PoC 검증 — 기존 video.pipeline 템플릿 기반 출력 가능 여부 확인
1. 프로젝트 스캐폴딩 + go mod init
2. `domain/` — 씬 모델, 프로젝트 상태 정의
3. `store/` — SQLite 스키마 + 마이그레이션 함수
4. `config/` — Viper 설정 로딩
5. `plugin/` — 4종 인터페이스 정의 + mock
6. `service/` — 파이프라인 오케스트레이터 + 상태 머신
7. `workspace/` — 프로젝트/씬 디렉토리 + 매니페스트
8. `cli/` — Cobra 커맨드 연결
9. `api/` — Chi 라우터 + Job 핸들러
10. Docker + docker-compose

**Cross-Component Dependencies:**
- `service/` → `domain/`, `store/`, `workspace/`, `plugin/`, `config/`, `glossary/`
- `cli/`, `api/` → `service/` (어댑터 패턴, 서비스 레이어만 의존)
- `plugin/` → `domain/` (씬 모델 공유)
- `store/`, `workspace/` → `domain/` (모델 참조)

## Implementation Patterns & Consistency Rules

### Naming Patterns

**Database (SQLite):**
- 테이블: `snake_case` 복수형 — `projects`, `jobs`, `scene_manifests`, `execution_logs`
- 컬럼: `snake_case` — `project_id`, `created_at`, `scene_num`, `estimated_cost_usd`
- 인덱스: `idx_{table}_{column}` — `idx_jobs_project_id`, `idx_scene_manifests_project_id`
- FK: `{referenced_table_singular}_id` — `project_id`, `job_id`

**API:**
- 엔드포인트: 복수형 kebab-case — `/api/projects`, `/api/projects/{projectId}/scenes/{sceneNum}`
- 경로 파라미터: Chi `{param}` 스타일, camelCase — `{projectId}`, `{sceneNum}`
- 쿼리 파라미터: `snake_case` — `?scp_id=SCP-173&status=approved`
- JSON 필드: `snake_case` — `{ "project_id": "...", "created_at": "..." }`
- 에러 코드: 대문자 `SNAKE_CASE` — `INVALID_SCP_ID`, `LLM_API_ERROR`, `SCENE_NOT_FOUND`

**Go Code:**
- 패키지: 소문자 단일 단어 — `store`, `domain`, `plugin`, `glossary`
- 타입/인터페이스: `PascalCase` — `SceneManifest`, `LLMPlugin`, `TransitionError`
- 함수/메서드: `PascalCase` (exported) / `camelCase` (unexported)
- 파일: `snake_case.go` — `scene_manifest.go`, `pipeline_service.go`
- 상수: `PascalCase` (exported) — `StatusPending`, `StatusApproved`

### Structure Patterns

**테스트:**
- 유닛 테스트: 동일 패키지 co-located — `store/project_test.go`
- 통합 테스트: `tests/integration/` (SQLite `:memory:` 사용)
- 테스트 픽스처: `testdata/` 디렉토리 (Go 빌드 자동 제외)
- 테스트 헬퍼: 각 패키지 내 `helpers_test.go`
- Mock 파일: `internal/mocks/` (mockery 자동 생성)
- 테스트 네이밍: `Test{Function}_{Scenario}` — `TestCreateProject_InvalidSCPID`, `TestGenerateScenario_LLMTimeout`

**서비스 레이어:**
- 1 파일 = 1 도메인 개념 — `service/scenario.go`, `service/image.go`
- New() 생성자 필수, 모든 의존성은 파라미터로 — Option 패턴 금지
- 서비스 간 직접 호출 금지 — `pipeline.go` 오케스트레이터만 서비스 조율
- `glossary/` 직접 import 허용 (읽기 전용 유틸리티, 의존성 주입 불필요)

```go
// 생성자 패턴 예시
func NewScenarioService(
    store store.Store,
    llm plugin.LLM,
    glossary *glossary.Glossary,
    logger *slog.Logger,
) *ScenarioService
```

**플러그인:**
- `plugin/{type}/interface.go` — 인터페이스 정의
- `plugin/{type}/{impl_name}.go` — 구현체
- `plugin/base.go` — 공통 Config, Timeout 헬퍼 (retry는 `internal/retry/`로 분리)

### Format Patterns

**API 응답:**
```json
// 성공
{ "success": true, "data": { ... } }

// 에러
{ "success": false, "error": { "code": "SCENARIO_NOT_FOUND", "message": "..." } }

// 목록
{ "success": true, "data": { "items": [...], "total": 42 } }
```

**날짜/시간:** ISO 8601 UTC — `"2026-03-08T12:00:00Z"` (JSON, SQLite 모두)

### Process Patterns

**에러 처리:**
- Go 표준 `error` 반환 + `fmt.Errorf("scenario generate: %w", err)` 래핑
- 커스텀 에러 4종 (`domain/errors.go`): `NotFoundError`, `ValidationError`, `PluginError`, `TransitionError`
- API 레이어 매핑: `ValidationError` → 400, `NotFoundError` → 404, `TransitionError` → 409, 나머지 → 500
- 서비스 레이어는 HTTP 개념 없음

**재시도:**
- 공통 `retry(ctx, maxAttempts, backoff, fn)` 헬퍼
- 재시도 대상: network timeout, 429, 5xx
- 재시도 불가: 400, 401, 403
- 외부 API만 재시도, 내부 로직 재시도 없음

**Context 전파:**
- 모든 서비스/플러그인 함수 첫 번째 파라미터: `context.Context`
- 모든 외부 API 호출에 `ctx` 전달
- CLI Ctrl+C → context cancel → 진행 중 API 호출 취소 체인

**로깅:**
```go
slog.Info("scene image generated",
    "project_id", projectID,
    "scene_num", sceneNum,
    "plugin", "siliconflow",
    "duration_ms", elapsed,
    "estimated_cost_usd", 0.003,
)
// 에러: "err" 키 사용
slog.Error("llm api failed", "err", err, "project_id", projectID)
```

**상태 머신:**
- `domain/project.go`에 허용 전이 맵 정의
- 전이 실패 → `TransitionError` 반환
- 모든 전이는 SQLite 트랜잭션 내 실행

### Enforcement Guidelines

**AI 에이전트 필수 규칙:**
1. 새 DB 테이블/컬럼 추가 시 `store/migrations/` SQL 파일 추가 + 버전 증가
2. 새 외부 API 호출 시 반드시 retry 헬퍼 + ctx 전달
3. 서비스 함수 시그니처: `func (s *XxxService) Method(ctx context.Context, ...) (..., error)`
4. 플러그인 인터페이스 변경 시 `go generate ./...` 실행하여 mock 재생성
5. API 핸들러는 비즈니스 로직 없음 — 파싱 + 서비스 호출 + 응답 포맷팅만
6. 새 서비스 추가 시 New() 생성자 필수, 의존성은 인터페이스 타입으로

**Anti-Patterns (금지):**
- 서비스 레이어에서 `http.Request` 참조
- 서비스 간 직접 호출 (오케스트레이터 우회)
- Option 패턴, 글로벌 변수, init() 함수
- 테스트에서 실제 외부 API 호출

## Project Structure & Boundaries

### Complete Project Directory Structure

```
youtube-pipeline/
├── cmd/
│   └── yt-pipe/
│       └── main.go                          # 진입점: Cobra root + API server 부트스트랩
├── internal/
│   ├── cli/                                 # Cobra 커맨드 (CLI 어댑터)
│   │   ├── root.go                          # 루트 커맨드 + 글로벌 플래그
│   │   ├── run.go                           # yt-pipe run {scp-id}
│   │   ├── scenario.go                      # yt-pipe scenario generate/approve/edit
│   │   ├── image.go                         # yt-pipe image generate/regenerate
│   │   ├── tts.go                           # yt-pipe tts generate
│   │   ├── assemble.go                      # yt-pipe assemble
│   │   ├── status.go                        # yt-pipe status
│   │   ├── config_cmd.go                    # yt-pipe config show/validate
│   │   └── init_cmd.go                      # yt-pipe init
│   ├── api/                                 # Chi 라우터 (API 어댑터)
│   │   ├── server.go                        # HTTP 서버 생성 + graceful shutdown
│   │   ├── routes.go                        # 라우트 등록
│   │   ├── response.go                      # 공통 응답 헬퍼 (Success/Error 래퍼)
│   │   ├── handlers/
│   │   │   ├── project.go                   # POST /api/projects, GET /api/projects/{id}
│   │   │   ├── scenario.go                  # POST/PUT /api/projects/{id}/scenario
│   │   │   ├── asset.go                     # POST /api/projects/{id}/assets/generate
│   │   │   ├── job.go                       # GET /api/jobs/{id}
│   │   │   └── health.go                    # GET /api/health
│   │   └── middleware/
│   │       ├── auth.go                      # X-API-Key 검증
│   │       ├── logging.go                   # 요청/응답 로깅
│   │       └── recovery.go                  # 패닉 복구
│   ├── service/                             # 코어 서비스 레이어
│   │   ├── pipeline.go                      # 오케스트레이터: 상태 머신 + 단계 조율
│   │   ├── scenario.go                      # 시나리오 생성 + 검증
│   │   ├── image.go                         # 이미지 프롬프트 생성 + 이미지 생성
│   │   ├── tts.go                           # TTS 생성 + 타이밍 추출
│   │   ├── subtitle.go                      # 자막 생성 (TTS 타이밍 기반)
│   │   ├── assembler.go                     # 최종 출력 조립 조율
│   │   ├── timing.go                        # 타이밍 해석기: TTS → 이미지/자막/타임라인
│   │   ├── job.go                           # Job 생성/조회/상태 관리
│   │   └── project.go                       # 프로젝트 CRUD + 상태 전이
│   ├── domain/                              # 도메인 모델 (순수 데이터 구조, 외부 의존성 없음)
│   │   ├── scene.go                         # Scene 모델 (공유 도메인 모델)
│   │   ├── project.go                       # Project 모델 + 상태 전이 맵
│   │   ├── manifest.go                      # SceneManifest (증분 빌드 추적)
│   │   ├── job.go                           # Job 모델
│   │   ├── scenario.go                      # ScenarioOutput 구조 (모듈 간 계약)
│   │   └── errors.go                        # NotFoundError, ValidationError, PluginError, TransitionError
│   ├── plugin/                              # 플러그인 인터페이스 + 구현
│   │   ├── base.go                          # 공통: Config, Timeout 헬퍼
│   │   ├── llm/
│   │   │   ├── interface.go                 # LLM 인터페이스
│   │   │   └── openai.go                    # OpenAI 호환 구현
│   │   ├── tts/
│   │   │   ├── interface.go                 # TTS 인터페이스
│   │   │   ├── openai_tts.go                # OpenAI TTS
│   │   │   ├── google_tts.go                # Google Cloud TTS
│   │   │   └── edge_tts.go                  # Edge TTS (무료)
│   │   ├── imagegen/
│   │   │   ├── interface.go                 # ImageGen 인터페이스
│   │   │   └── siliconflow.go               # SiliconFlow FLUX
│   │   └── output/
│   │       ├── interface.go                 # OutputAssembler 인터페이스
│   │       └── capcut.go                    # CapCut 프로젝트 생성
│   ├── config/                              # Viper 설정 관리
│   │   ├── config.go                        # 설정 로딩 + 5단계 우선순위
│   │   └── types.go                         # 설정 구조체 정의
│   ├── store/                               # SQLite 저장소
│   │   ├── store.go                         # DB 초기화 + 마이그레이션 실행
│   │   ├── project.go                       # 프로젝트 CRUD
│   │   ├── job.go                           # Job CRUD
│   │   ├── manifest.go                      # 씬 매니페스트 CRUD
│   │   ├── execution_log.go                 # 실행 이력 + API 비용 로그
│   │   └── migrations/
│   │       ├── 001_initial.sql              # 초기 스키마
│   │       └── embed.go                     # go:embed SQL 파일
│   ├── workspace/                           # 파일시스템 에셋 관리
│   │   ├── project.go                       # 프로젝트 디렉토리 생성/조회
│   │   ├── scene.go                         # 씬별 에셋 읽기/쓰기 (atomic write)
│   │   └── scp_data.go                      # SCP 원본 데이터 읽기
│   ├── glossary/                            # SCP 용어 사전
│   │   └── glossary.go                      # 외부 JSON 파일 런타임 로딩 + 조회
│   ├── retry/                               # 범용 재시도 헬퍼
│   │   └── retry.go                         # retry(ctx, maxAttempts, backoff, fn)
│   └── mocks/                               # mockery 자동 생성 mock
│       └── .gitkeep
├── tests/
│   └── integration/                         # 통합 테스트 (SQLite :memory:)
│       ├── pipeline_test.go
│       └── helpers_test.go
├── testdata/                                # 테스트 픽스처
│   ├── scp-173/                             # 샘플 SCP 데이터
│   │   ├── facts.json
│   │   ├── meta.json
│   │   └── main.txt
│   └── scenarios/                           # 샘플 시나리오 출력
│       └── sample_scenario.json
├── config.example.yaml                      # 설정 예시 (용어 사전 경로 포함)
├── Dockerfile                               # 멀티스테이지 빌드 (golang → scratch)
├── docker-compose.yml                       # 볼륨 마운트 + 서비스 정의
├── Makefile                                 # build, test, generate, docker, lint
├── .gitignore                               # bin/, *.db, internal/mocks/ 등
├── go.mod
└── go.sum
```

**패키지 수:** 10개 (`cli`, `api`, `service`, `domain`, `plugin`, `config`, `store`, `workspace`, `glossary`, `retry`)

**Makefile 타겟:**
- `make build` → `go build -o bin/yt-pipe ./cmd/yt-pipe`
- `make test` → `go test ./...`
- `make generate` → `go generate ./...`
- `make docker` → `docker build -t yt-pipe .`
- `make run` → `go run ./cmd/yt-pipe serve`
- `make lint` → `go vet ./...`

**마이그레이션 파일 네이밍:** `{NNN}_{description}.sql` (3자리 zero-pad) — `001_initial.sql`, `002_add_cost_tracking.sql`

### Architectural Boundaries

**API 경계:**
- `cli/` → `service/` : Cobra 커맨드가 서비스 직접 호출, 동기 실행
- `api/handlers/` → `service/` : HTTP 핸들러가 서비스 호출, Job으로 비동기 래핑
- `api/middleware/` : 인증/로깅은 핸들러 진입 전 처리
- 경계 규칙: `cli/`와 `api/`는 서로 참조 금지, 반드시 `service/`를 통해서만

**서비스 경계:**
- `service/pipeline.go` : 유일한 오케스트레이터, 다른 서비스 조율
- 개별 서비스 (`scenario`, `image`, `tts` 등) : 독립적, 서로 직접 호출 금지
- 의존 방향: `service/` → `store/`, `workspace/`, `plugin/`, `domain/`, `glossary/`, `retry/`

**데이터 경계:**
- `store/` : SQLite 전용, SQL 쿼리는 이 패키지 밖으로 노출 안 됨
- `workspace/` : 파일 I/O 전용, 파일 경로 구성은 이 패키지가 책임
- `domain/` : 순수 데이터 구조, 외부 의존성 없음 (import cycle 방지의 기반)

**의존성 방향 (import cycle 방지):**
```
domain/ ← (모든 패키지가 참조)
retry/  ← service/, plugin/
store/, workspace/ → domain/
plugin/ → domain/, retry/
service/ → domain/, store/, workspace/, plugin/, config/, glossary/, retry/
cli/, api/ → service/ (+ domain/ for request/response 타입)
```

### Requirements to Structure Mapping

| PRD FR 카테고리 | 주요 패키지 | 핵심 파일 |
|-----------------|------------|----------|
| SCP 데이터 처리 (FR1-5) | `workspace/`, `glossary/` | `scp_data.go`, `glossary.go` |
| 시나리오 생성 (FR6-11) | `service/`, `plugin/llm/` | `scenario.go`, `openai.go` |
| 이미지 생성 (FR12-16) | `service/`, `plugin/imagegen/` | `image.go`, `siliconflow.go` |
| TTS 생성 (FR17-19) | `service/`, `plugin/tts/` | `tts.go`, `timing.go` |
| 출력 조립 (FR20-21) | `service/`, `plugin/output/` | `assembler.go`, `capcut.go` |
| 프로젝트 관리 (FR22-25) | `service/`, `store/`, `domain/` | `project.go`, `manifest.go` |
| CLI 인터페이스 (FR26-29) | `cli/` | `run.go`, `scenario.go` 등 |
| API 인터페이스 (FR30-33) | `api/` | `handlers/*.go`, `routes.go` |
| 설정 관리 (FR34-37) | `config/` | `config.go`, `types.go` |
| 증분 빌드 (FR38-40) | `service/`, `domain/` | `pipeline.go`, `manifest.go` |

**교차 관심사 → 위치:**

| 관심사 | 위치 |
|--------|------|
| 에러 타입 | `domain/errors.go` |
| 재시도 헬퍼 | `retry/retry.go` |
| 구조화 로깅 | 각 패키지에서 `*slog.Logger` 주입 |
| 설정 우선순위 | `config/config.go` |
| 데이터 무결성 | `workspace/scene.go` (atomic write) |
| 용어 사전 | `glossary/glossary.go` (외부 JSON 런타임 로딩) |

### Data Flow

```
SCP Data (filesystem) → workspace/scp_data.go
    → service/scenario.go (+ plugin/llm/) → 시나리오 생성
        → service/image.go (+ plugin/imagegen/) → 이미지 생성 ─┐
        → service/tts.go (+ plugin/tts/) → TTS 생성            ├→ (병렬)
            → service/timing.go → 타이밍 해석                   │
            → service/subtitle.go → 자막 생성                   ┘
                → service/assembler.go (+ plugin/output/) → CapCut 프로젝트
```

모든 중간 산출물: `workspace/{scp-id}-{timestamp}/scenes/{num}/`
모든 메타데이터: `store/` (SQLite)

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
- Go + Cobra + Chi + SQLite(modernc.org) — 모두 CGO-free, 단일 바이너리 호환
- Viper는 Cobra 네이티브 통합, 설정 우선순위 5단계 지원
- testify + mockery는 Go 표준 testing과 완전 호환
- log/slog는 Go 1.21+ 표준, 추가 의존성 없음
- 모든 기술 선택이 "단일 바이너리 → scratch Docker" 목표와 정렬

**Pattern Consistency:**
- DB `snake_case` ↔ JSON `snake_case` ↔ Go struct tag — 일관
- API 엔드포인트 복수형 ↔ DB 테이블 복수형 — 일관
- 에러 타입 4종 → HTTP 코드 매핑 명확 (400/404/409/500)
- New() 생성자 패턴 + context.Context 첫 파라미터 — 전 서비스 통일

**Structure Alignment:**
- 10개 패키지가 의존 방향을 지키고, import cycle 없음
- `domain/`이 순수 데이터 구조로 cycle 방지의 기반
- `retry/` 분리로 `plugin/` ↔ `service/` 의존 방향 깨끗

### Requirements Coverage Validation ✅

**FR 커버리지 (61개 FR — 전수 확인):**

| FR 범위 | 아키텍처 커버리지 | 상태 |
|---------|------------------|------|
| FR1-3 SCP 데이터 | `workspace/scp_data.go` + `glossary/` | ✅ |
| FR4-8 시나리오 | `service/scenario.go` + `plugin/llm/` + `domain/scenario.go` | ✅ |
| FR9-12 이미지 | `service/image.go` + `plugin/imagegen/` | ✅ |
| FR13-16 TTS & 자막 | `service/tts.go` + `service/timing.go` + `plugin/tts/` | ✅ |
| FR17-19 출력 조립 | `service/assembler.go` + `plugin/output/capcut.go` | ✅ |
| FR20-30 파이프라인 제어 | `domain/project.go` + `store/` + `service/pipeline.go` | ✅ |
| FR31-36 설정 | `config/` (Viper 5단계) + `cli/init_cmd.go` | ✅ |
| FR37-40 API | `api/handlers/` + `middleware/` | ✅ |
| FR42-44 모니터링 | slog 로깅, 성공률 집계 | ✅ |
| FR45-47 프롬프트 템플릿 | `service/template.go` + `store/template.go` + `domain/template.go` | ✅ |
| FR48-50 캐릭터 ID카드 | `service/character.go` + `store/character.go` + `domain/character.go` | ✅ |
| FR51-52 TTS 분위기 | `service/mood.go` + `store/mood_preset.go` + `plugin/tts/` 확장 | ✅ |
| FR53 VC | Phase 2 — TTS 플러그인 옵셔널 인터페이스 예약 | ⏳ |
| FR54-56 씬별 승인 | `service/approval.go` + `store/scene_approval.go` + 상태 머신 확장 | ✅ |
| FR57-60 BGM | `service/bgm.go` + `store/bgm.go` + `plugin/output/capcut.go` 확장 | ✅ |
| FR61 마이그레이션 | `cli/init_cmd.go` + `store/template.go` 시딩 | ✅ |

**NFR 커버리지 (핵심 7개):**

| NFR | 아키텍처 대응 | 상태 |
|-----|-------------|------|
| NFR8 데이터 무결성 | atomic write + 매니페스트 | ✅ |
| NFR9 플러그인 표준화 | 4종 인터페이스 + base.go | ✅ |
| NFR13 Docker | multi-stage → scratch | ✅ |
| NFR19 로깅 | slog JSON + cost 필드 | ✅ |
| NFR20 결합도 최소화 | 서비스 레이어 분리 + DI | ✅ |
| NFR21 플러그인 추가 | 인터페이스 기반 + config 선택 | ✅ |
| NFR23 테스트 가능성 | mockery mock + `:memory:` SQLite | ✅ |

**PRD 구현 누출 7건:** 전수 아키텍처 결정으로 해소 ✅

### Implementation Readiness Validation ✅

**Decision Completeness:** 기술 스택 전 항목 명시, Critical/Important/Deferred 분류 완료, 교차 관심사 12→7개 MVP 최적화
**Structure Completeness:** 파일 수준 ~60개, FR→패키지 매핑 테이블, Data Flow 다이어그램
**Pattern Completeness:** DB/API/Go 네이밍 컨벤션 + 에러 처리 + 테스트 패턴 + Anti-Patterns

### Gap Analysis Results

**Critical Gaps:** 없음

**Addressed Gaps (검증 중 해소):**
- CapCut PoC 검증 → Implementation Sequence 0번에 추가 완료
- `make run` 타겟 → Makefile 타겟에 추가 완료
- 용어 사전 경로 → config.example.yaml에 `glossary_path` 포함 명시 (glossary는 외부 런타임 로딩)

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] Project context 분석 (Party Mode + 5건 Advanced Elicitation 포함)
- [x] Scale/complexity 평가 (Medium, ~13 컴포넌트)
- [x] Technical constraints 식별 (6건)
- [x] Cross-cutting concerns 매핑 (7건 MVP 최적화)

**✅ Architectural Decisions**
- [x] Critical decisions 문서화 (4건)
- [x] Technology stack 완전 명시 (Go + 7개 라이브러리)
- [x] Integration patterns 정의 (어댑터 패턴, 오케스트레이터)
- [x] Deferred decisions 명시 (6건 Post-MVP)

**✅ Implementation Patterns**
- [x] Naming conventions 확립 (DB, API, Go Code)
- [x] Structure patterns 정의 (테스트, 서비스, 플러그인)
- [x] Process patterns 문서화 (에러, 재시도, Context, 로깅, 상태 머신)
- [x] Enforcement guidelines + Anti-Patterns

**✅ Project Structure**
- [x] Complete directory structure (~60 파일)
- [x] Component boundaries (API, 서비스, 데이터)
- [x] 의존성 방향 (import cycle 방지)
- [x] Requirements → Structure 매핑 완비

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** HIGH

**Key Strengths:**
- 1인 MVP에 최적화된 단순한 아키텍처 (Party Mode 오버엔지니어링 검토 반영)
- Go 생태계의 관례를 충실히 따르는 패턴
- 명확한 의존 방향과 경계로 AI 에이전트 간 충돌 방지
- 플러그인 인터페이스로 확장성 확보하면서 MVP는 최소 구현

**Areas for Future Enhancement:**
- Phase 2: 이벤트 pub/sub, 동시 파이프라인, CI/CD
- Phase 3: 비용 대시보드, 스타일 프리셋, 검증 게이트 프레임워크

### Implementation Handoff

**AI 에이전트 필수 지침:**
1. 이 문서의 모든 아키텍처 결정을 정확히 따를 것
2. Implementation Patterns의 네이밍/구조/프로세스 패턴을 일관되게 적용할 것
3. Project Structure의 경계와 의존 방향을 준수할 것
4. Anti-Patterns에 명시된 4개 금지 규칙을 위반하지 말 것

**First Implementation Priority:**
0. CapCut PoC 검증 (기존 video.pipeline 템플릿 기반)
1. 프로젝트 스캐폴딩 + `go mod init` + 디렉토리 구조 생성

---

## Incremental Update: FR45-FR61 (2026-03-09)

> 이 섹션은 PRD에 FR45-FR61이 추가된 후 아키텍처 증분 업데이트를 기록한다.
> 핵심 아키텍처 결정(Go, Cobra, Chi, SQLite, 플러그인 패턴, 의존 방향)은 변경 없음.
> 기존 구조 위에 모듈 추가/확장으로 해결.

### State Machine Update (FR22 확장)

**기존:**
```
pending → scenario_review → approved → generating_assets → assembling → complete
```

**변경:**
```
pending → scenario_review → approved → image_review → tts_review → assembling → complete
```

- `image_review`: 씬별 이미지 생성-미리보기-승인/재생성 워크플로우 (FR54)
- `tts_review`: 씬별 TTS 합성-미리듣기-승인/재합성 워크플로우 (FR55)
- 각 상태에서 개별 씬의 승인 상태를 `scene_approvals` 테이블로 추적
- 모든 씬이 승인되면 다음 상태로 자동 전이

**씬 승인 상태 모델:**
```
per scene: pending → generated → approved | rejected → (regenerated → approved)
```

### New SQLite Tables

기존 마이그레이션에 이어 추가:

**`002_templates.sql`** (FR45-47, FR61):
```sql
CREATE TABLE prompt_templates (
    id          TEXT PRIMARY KEY,
    category    TEXT NOT NULL CHECK(category IN ('scenario','image','tts','caption')),
    name        TEXT NOT NULL,
    content     TEXT NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE prompt_template_versions (
    id          TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES prompt_templates(id),
    version     INTEGER NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL
);
-- 최근 10개 버전만 보존, 초과 시 가장 오래된 버전 삭제

CREATE TABLE project_template_overrides (
    project_id  TEXT NOT NULL,
    template_id TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, template_id)
);

CREATE INDEX idx_templates_category ON prompt_templates(category);
CREATE INDEX idx_template_versions_template_id ON prompt_template_versions(template_id);
```

**`003_characters.sql`** (FR48-50):
```sql
CREATE TABLE characters (
    id                TEXT PRIMARY KEY,
    scp_id            TEXT NOT NULL,
    canonical_name    TEXT NOT NULL,
    aliases           TEXT,          -- JSON array: ["SCP-173", "조각상", "The Sculpture"]
    visual_descriptor TEXT NOT NULL, -- 외형 묘사 텍스트
    style_guide       TEXT,          -- 스타일 가이드
    image_prompt_base TEXT,          -- 참조 이미지 프롬프트 기반
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE INDEX idx_characters_scp_id ON characters(scp_id);
```

**`004_mood_presets.sql`** (FR51-52):
```sql
CREATE TABLE mood_presets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT,
    speed       REAL,     -- TTS 속도 배율
    emotion     TEXT,     -- 감정 파라미터 (Qwen3-TTS 기반)
    pitch       REAL,     -- 피치 조절
    params_json TEXT,     -- 추가 TTS 파라미터 (JSON)
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE scene_mood_assignments (
    project_id  TEXT NOT NULL,
    scene_num   INTEGER NOT NULL,
    preset_id   TEXT NOT NULL REFERENCES mood_presets(id),
    auto_mapped INTEGER NOT NULL DEFAULT 0, -- 1=LLM 자동 매핑, 0=수동 지정
    confirmed   INTEGER NOT NULL DEFAULT 0, -- 크리에이터 확인 여부
    PRIMARY KEY (project_id, scene_num)
);
```

**`005_bgms.sql`** (FR57-60):
```sql
CREATE TABLE bgms (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    file_path     TEXT NOT NULL,
    mood_tags     TEXT NOT NULL,  -- JSON array: ["horror", "tension", "mystery"]
    duration_ms   INTEGER,
    license_type  TEXT NOT NULL,  -- "royalty_free", "cc_by", etc.
    license_source TEXT,
    credit_text   TEXT NOT NULL,  -- 영상 설명에 포함될 크레딧
    created_at    TEXT NOT NULL
);

CREATE TABLE scene_bgm_assignments (
    project_id  TEXT NOT NULL,
    scene_num   INTEGER NOT NULL,
    bgm_id      TEXT NOT NULL REFERENCES bgms(id),
    volume_db   REAL NOT NULL DEFAULT 0,
    fade_in_ms  INTEGER NOT NULL DEFAULT 2000,
    fade_out_ms INTEGER NOT NULL DEFAULT 2000,
    ducking_db  REAL NOT NULL DEFAULT -12,
    auto_recommended INTEGER NOT NULL DEFAULT 0,
    confirmed   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, scene_num)
);

CREATE INDEX idx_bgms_mood_tags ON bgms(mood_tags);
```

**`006_scene_approvals.sql`** (FR54-55):
```sql
CREATE TABLE scene_approvals (
    project_id  TEXT NOT NULL,
    scene_num   INTEGER NOT NULL,
    asset_type  TEXT NOT NULL CHECK(asset_type IN ('image', 'tts')),
    status      TEXT NOT NULL CHECK(status IN ('pending','generated','approved','rejected')),
    attempts    INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, scene_num, asset_type)
);

CREATE INDEX idx_scene_approvals_project ON scene_approvals(project_id, asset_type);
```

### New & Modified Files

**새 도메인 모델 (`domain/`):**
```
domain/
├── template.go          # PromptTemplate, TemplateVersion, TemplateCategory 타입
├── character.go         # Character 모델 (canonical_name, aliases, visual_descriptor)
├── mood_preset.go       # MoodPreset 모델 (speed, emotion, pitch, params)
├── bgm.go              # BGM 모델 (mood_tags, license, credit)
└── scene_approval.go   # SceneApproval 모델 (asset_type, status, attempts)
```

**새 저장소 (`store/`):**
```
store/
├── template.go          # 템플릿 CRUD + 버전 관리 + 롤백 + 오버라이드
├── character.go         # 캐릭터 ID카드 CRUD + 별칭 검색
├── mood_preset.go       # 분위기 프리셋 CRUD + 씬 할당
├── bgm.go              # BGM CRUD + 태그 검색 + 씬 할당
├── scene_approval.go   # 씬별 승인 상태 CRUD
└── migrations/
    ├── 002_templates.sql
    ├── 003_characters.sql
    ├── 004_mood_presets.sql
    ├── 005_bgms.sql
    └── 006_scene_approvals.sql
```

**새 서비스 (`service/`):**
```
service/
├── template.go          # 템플릿 CRUD + 버전 관리 + 프로젝트 오버라이드
├── character.go         # ID카드 CRUD + 씬 텍스트에서 개체명 매칭
├── mood.go             # 분위기 프리셋 관리 + LLM 기반 자동 매핑
├── bgm.go              # BGM 관리 + LLM 기반 자동 추천
└── approval.go         # 씬별 승인 워크플로우 오케스트레이션
```

**새 CLI 커맨드 (`cli/`):**
```
cli/
├── prompt.go            # yt-pipe prompt list/show
├── character.go         # yt-pipe character create/list
└── bgm.go              # yt-pipe bgm list/assign
```

**수정되는 기존 파일:**

| 파일 | 변경 내용 |
|------|----------|
| `domain/project.go` | 상태 전이 맵에 `image_review`, `tts_review` 추가 |
| `plugin/tts/interface.go` | `MoodPreset` 파라미터 추가 — `Generate(ctx, text, mood *MoodPreset) (Audio, error)` |
| `plugin/imagegen/interface.go` | `CharacterRefs` 파라미터 추가 — `Generate(ctx, prompt, chars []CharacterRef) (Image, error)` |
| `plugin/output/capcut.go` | BGM 트랙 배치 로직 추가 (볼륨, 페이드, 덕킹) + BGM 크레딧 포함 |
| `service/pipeline.go` | 오케스트레이터에 `image_review` → `tts_review` 단계 추가, 승인 대기 로직 |
| `service/image.go` | 캐릭터 ID카드 자동 참조 로직 (개체명 매칭) 추가 |
| `service/tts.go` | 분위기 프리셋 적용 로직 추가 |
| `service/assembler.go` | BGM 배치 + 라이선스 크레딧 포함 |
| `cli/init_cmd.go` | 기본 프롬프트 템플릿 시딩 로직 추가 (FR61) |
| `config/types.go` | BGM 기본 설정 추가 (ducking_db, fade_ms) |

### Plugin Interface Changes

**TTS Interface (확장):**
```go
// 기존
type TTS interface {
    Generate(ctx context.Context, text string) (*Audio, error)
}

// 변경
type TTS interface {
    Generate(ctx context.Context, text string, opts *TTSOptions) (*Audio, error)
}

type TTSOptions struct {
    MoodPreset *MoodPreset // nil이면 기본 톤 사용
}

type MoodPreset struct {
    Speed   float64
    Emotion string
    Pitch   float64
    Params  map[string]any // TTS 엔진별 추가 파라미터
}
```

**ImageGen Interface (확장):**
```go
// 기존
type ImageGen interface {
    Generate(ctx context.Context, prompt string) (*Image, error)
}

// 변경
type ImageGen interface {
    Generate(ctx context.Context, prompt string, opts *ImageGenOptions) (*Image, error)
}

type ImageGenOptions struct {
    CharacterRefs []CharacterRef // 빈 슬라이스면 캐릭터 참조 없음
}

type CharacterRef struct {
    Name            string
    VisualDescriptor string
    ImagePromptBase  string
}
```

**OutputAssembler Interface (확장):**
```go
// 기존
type OutputAssembler interface {
    Assemble(ctx context.Context, project *Project) (string, error)
}

// 변경
type OutputAssembler interface {
    Assemble(ctx context.Context, project *Project, opts *AssembleOptions) (string, error)
}

type AssembleOptions struct {
    BGMAssignments []BGMAssignment
    Credits        []CreditEntry // CC-BY-SA + BGM 크레딧
}

type BGMAssignment struct {
    SceneNum  int
    FilePath  string
    VolumeDB  float64
    FadeInMs  int
    FadeOutMs int
    DuckingDB float64
}
```

### Character Matching Algorithm (FR50)

씬별 이미지 생성 시 캐릭터 자동 참조 로직:

```
1. characters 테이블에서 프로젝트 SCP ID에 해당하는 캐릭터 + 글로벌 캐릭터 로드
2. 각 캐릭터의 canonical_name + aliases 목록 구성
3. 씬 시나리오 텍스트에서 각 이름/별칭 문자열 매칭
4. 매칭된 캐릭터의 CharacterRef를 ImageGenOptions에 포함
5. 이미지 생성 플러그인이 visual_descriptor + image_prompt_base를 프롬프트에 합성
```

### Updated Requirements to Structure Mapping

| PRD FR 카테고리 | 주요 패키지 | 핵심 파일 |
|-----------------|------------|----------|
| SCP 데이터 처리 (FR1-3) | `workspace/`, `glossary/` | `scp_data.go`, `glossary.go` |
| 시나리오 생성 (FR4-8) | `service/`, `plugin/llm/` | `scenario.go`, `openai.go` |
| 이미지 생성 (FR9-12) | `service/`, `plugin/imagegen/` | `image.go`, `siliconflow.go` |
| TTS & 자막 (FR13-16) | `service/`, `plugin/tts/` | `tts.go`, `timing.go` |
| CapCut 조립 (FR17-19) | `service/`, `plugin/output/` | `assembler.go`, `capcut.go` |
| 파이프라인 제어 (FR20-30, 42-44) | `service/`, `store/`, `domain/` | `pipeline.go`, `project.go` |
| 설정 & 플러그인 (FR31-36) | `config/`, `cli/` | `config.go`, `init_cmd.go` |
| API 인터페이스 (FR37-40) | `api/` | `handlers/*.go`, `routes.go` |
| **프롬프트 템플릿 (FR45-47, 61)** | **`service/`, `store/`, `domain/`, `cli/`** | **`template.go`, `prompt.go`** |
| **캐릭터 ID카드 (FR48-50)** | **`service/`, `store/`, `domain/`, `cli/`** | **`character.go`** |
| **TTS 분위기 (FR51-52)** | **`service/`, `store/`, `domain/`, `plugin/tts/`** | **`mood.go`, `mood_preset.go`** |
| **씬별 승인 (FR54-56)** | **`service/`, `store/`, `domain/`** | **`approval.go`, `scene_approval.go`** |
| **BGM 관리 (FR57-60)** | **`service/`, `store/`, `domain/`, `cli/`, `plugin/output/`** | **`bgm.go`** |

### Updated Data Flow

```
SCP Data (filesystem) → workspace/scp_data.go
    → service/scenario.go (+ plugin/llm/) → 시나리오 생성
        → service/character.go → 씬별 캐릭터 매칭
        → service/mood.go (+ plugin/llm/) → 씬별 분위기 자동 매핑 → 크리에이터 확인
        → service/approval.go [image_review]
            → service/image.go (+ plugin/imagegen/ + CharacterRefs) → 씬별 이미지 생성-승인
        → service/approval.go [tts_review]
            → service/tts.go (+ plugin/tts/ + MoodPreset) → 씬별 TTS 생성-승인
                → service/timing.go → 타이밍 해석
                → service/subtitle.go → 자막 생성
        → service/bgm.go (+ plugin/llm/) → BGM 자동 추천 → 크리에이터 확인
        → service/assembler.go (+ plugin/output/ + BGMAssignments) → CapCut 프로젝트
```

### Updated Package Count & Dependencies

**패키지 수:** 10개 → 10개 (변경 없음, 기존 패키지 내 파일 추가)

**새로운 의존 관계 (기존 규칙 준수):**
```
service/template.go   → store/, domain/
service/character.go  → store/, domain/
service/mood.go       → store/, domain/, plugin/llm/ (분위기 분석용)
service/bgm.go        → store/, domain/, plugin/llm/ (BGM 추천용)
service/approval.go   → store/, domain/
service/image.go      → store/character (캐릭터 참조 조회)
service/tts.go        → store/mood_preset (분위기 프리셋 조회)
service/assembler.go  → store/bgm (BGM 할당 조회)
```

모든 새 의존 관계는 기존 의존 방향(`service/` → `store/`, `domain/`, `plugin/`)을 준수한다.

### Deferred Decisions Update

기존 Deferred에 추가:
- FR53 VC(Voice Cloning) → Phase 2 (TTS 플러그인 인터페이스에 옵셔널 `VoiceCloner` 인터페이스 예약)
- 프롬프트 A/B 테스트 → Phase 2
- BGM 자동 작곡/생성 → Phase 3

---

## Incremental Update: EFR1-EFR6 (2026-03-18)

> 이 섹션은 PRD Enhancement(`prd-enhancement.md`)의 EFR1-EFR6이 추가된 후 아키텍처 증분 업데이트를 기록한다.
> 핵심 아키텍처 결정(Go, Cobra, Chi, SQLite, 플러그인 패턴, 의존 방향)은 **변경 없음**.
> 기존 구조 위에 모듈 추가/확장 + Docker 베이스 이미지 변경으로 해결.
>
> **입력 문서:**
> - `_bmad-output/planning-artifacts/prd-enhancement.md` (EFR1-6 + 로드맵 R1-R15)
> - `_bmad-output/brainstorming/brainstorming-session-2026-03-17-1000.md`
> - `_bmad-output/planning-artifacts/brainstorming-feasibility-analysis-2026-03-18.md`

### Decision Priority Analysis (EFR)

**Critical Decisions (Block Implementation):**
- LLM 인터페이스 Vision 확장 (`CompleteWithVision()` 메서드 추가) — EFR3 전제
- Docker 베이스 이미지 `scratch` → `alpine` 변경 — EFR6 전제
- FFmpeg Assembler 구현체 (`plugin/output/ffmpeg/`) — EFR6 핵심

**Important Decisions (Shape Architecture):**
- 용어사전 제안 저장: SQLite `glossary_suggestions` 테이블 — EFR2
- 자동 승인 로직: `ApprovalService` 확장 + `auto_approve_threshold` 설정 — EFR4
- 배치 프리뷰 데이터 구조 — EFR5

**Deferred Decisions (Post-Enhancement):**
- R1 SFX 자동 삽입 → Phase 2 (BGMService 패턴 복제)
- R2 동적 자막 스타일링 → Phase 2 (TextMaterial 스타일 매핑)
- R3 문장 단위 이미지-자막 동기화 → Phase 2 (Shot 데이터 이미 존재, 조립 로직 수정)
- R4 썸네일+제목+설명 자동 생성 → Phase 2 (LLM + 이미지 합성)
- R5 스타일 프리셋 시스템 → Phase 2 (Config 구조 확장)
- R6 Qwen 이미지 프로바이더 → Phase 2 (ImageGen 구현체 추가)
- R7 에셋 레지스트리 → Phase 2 (SQLite + 파일시스템)
- R8 CapCut 완전 대체 → Phase 3 (EFR6 의존)
- R9 핵심 씬 i2v → Phase 3 (VideoGen 플러그인)
- R10 AI 이미지 트랜지션 → Phase 3 (보간 모델)
- R11 역방향 파이프라인 → Phase 3 (Google API + Qwen-VL)
- R12 승인 전방 이동 → Phase 3 (EFR6 의존)
- R13 CI/CD 패턴 → Phase 3 (EFR4 + EFR6 + FR30)
- R14 듀얼 프로파일 → Phase 3 (Phase 2 기능 전제)
- R15 콘텐츠 캘린더 → Phase 3 (R13 의존)

### LLM Interface Vision Extension (EFR3 전제)

**결정:** 기존 `LLM` 인터페이스에 `CompleteWithVision()` 메서드를 추가한다. 기존 `ImageGen.Edit()`의 `ErrNotSupported` 패턴을 따른다.

**Rationale:**
- Vision은 LLM의 능력이므로 LLM 플러그인 안에 두는 것이 자연스러움
- 기존 `Complete()` 시그니처 변경 없이 호환성 유지
- 구현체가 Vision을 지원하지 않으면 `ErrNotSupported` 반환

```go
// plugin/llm/interface.go — 추가

// VisionMessage represents a multimodal message with text and images.
type VisionMessage struct {
    Role    string         // "system", "user", "assistant"
    Content []ContentPart  // mixed text + image parts
}

type ContentPart struct {
    Type     string // "text" or "image_url"
    Text     string // Type=="text" 일 때
    ImageURL string // Type=="image_url" 일 때 (base64 data URI or URL)
}

// CompleteWithVision sends multimodal messages to a vision-capable LLM.
// Returns ErrNotSupported if the provider does not support vision.
CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error)
```

**구현체 매핑:**
- `OpenAICompatibleProvider` → Qwen-VL, GPT-4V 등 OpenAI 호환 Vision API 지원. 기존 HTTP 호출 로직 재사용, `messages` 구조만 변경
- 비전 미지원 프로바이더 → `ErrNotSupported` 반환

**테스트 전략:**
- `CompleteWithVision()` 인터페이스 mock 테스트
- `ErrNotSupported` 반환 시 graceful fallback 검증
- Vision 메시지 직렬화 포맷 검증 (OpenAI multimodal format)

### Docker Base Image Change (EFR6 전제)

**결정:** Docker 베이스 이미지를 `scratch` → `alpine`으로 변경한다.

**Rationale:**
- FFmpeg 바이너리가 필요하므로 `scratch` 불가
- `alpine`은 `apk add ffmpeg`으로 간편 설치 (~80MB)
- 1인 홈서버에서 이미지 크기 우선순위 낮음
- Phase 3에서 추가 외부 바이너리(i2v 모델 등) 필요 가능성 고려

```dockerfile
# Dockerfile 변경

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates tzdata
RUN adduser -D -u 65534 appuser
COPY --from=builder /yt-pipe /yt-pipe
COPY --from=builder /templates /templates
USER appuser
EXPOSE 8080
ENTRYPOINT ["/yt-pipe"]
```

**ENFR3 준수:** FFmpeg 미설치 시 명확한 에러 메시지 출력

```go
// plugin/output/ffmpeg/ffmpeg.go
func checkFFmpegAvailable() error {
    if _, err := exec.LookPath("ffmpeg"); err != nil {
        return fmt.Errorf("ffmpeg binary not found in PATH: install ffmpeg or use Docker image with ffmpeg included")
    }
    return nil
}
```

### EFR1: YouTube Chapters — 아키텍처

**위치:** `service/timing.go`에 `GenerateChapters()` 메서드 추가

**Rationale:** 타이밍 데이터(`Timeline`)를 직접 사용하며 ~30줄 수준. 별도 파일 불필요.

```go
// service/timing.go — 추가

// ChapterEntry represents a single YouTube chapter.
type ChapterEntry struct {
    TimestampSec float64
    Title        string
}

// GenerateChapters converts a Timeline into YouTube chapter format.
// Output: "0:00 Title\n1:23 Title\n..."
func (s *TimingService) GenerateChapters(timeline *Timeline) []ChapterEntry

// SaveChaptersFile writes chapters to {projectPath}/chapters.txt
func (s *TimingService) SaveChaptersFile(chapters []ChapterEntry, projectPath string) error
```

**챕터 제목 생성 규칙:**
- 첫 번째 씬: "Intro" (고정)
- 이후 씬: `Scene.Mood` + `Scene.VisualDesc` 앞 30자 조합
- 포맷: `M:SS` (1시간 미만), `H:MM:SS` (1시간 이상)

**CLI:**
```
yt-pipe chapters <scp-id>   # chapters.txt 생성
```

**테스트 전략:**
- `GenerateChapters()` 단위 테스트: 다양한 타이밍 입력 → 챕터 포맷 검증
- 0초 시작, 1시간 이상 타이밍, 단일 씬 엣지 케이스
- `SaveChaptersFile()`: 임시 디렉토리에 파일 쓰기 → 내용 검증

### EFR2: 용어 사전 자동 확장 — 아키텍처

**새 서비스:** `service/glossary.go`

**새 테이블:** `007_glossary_suggestions.sql`

```sql
CREATE TABLE glossary_suggestions (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL,
    term          TEXT NOT NULL,
    pronunciation TEXT NOT NULL,
    definition    TEXT,
    category      TEXT,
    status        TEXT NOT NULL CHECK(status IN ('pending','approved','rejected')),
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    UNIQUE(term, project_id)
);

CREATE INDEX idx_glossary_suggestions_status ON glossary_suggestions(status);
CREATE INDEX idx_glossary_suggestions_project ON glossary_suggestions(project_id);
```

**서비스 인터페이스:**

```go
// service/glossary.go

type GlossaryService struct {
    store    store.Store
    llm      llm.LLM
    glossary *glossary.Glossary
    logger   *slog.Logger
}

// SuggestTerms extracts SCP terms from scenario text via LLM,
// diffs against existing glossary, and stores new suggestions.
func (s *GlossaryService) SuggestTerms(ctx context.Context, projectID string, scenarioText string) ([]domain.GlossarySuggestion, error)

// ApproveSuggestion approves a suggestion and adds it to glossary.json.
func (s *GlossaryService) ApproveSuggestion(ctx context.Context, suggestionID string) error

// RejectSuggestion marks a suggestion as rejected.
func (s *GlossaryService) RejectSuggestion(ctx context.Context, suggestionID string) error

// ListPendingSuggestions returns all pending suggestions for a project.
func (s *GlossaryService) ListPendingSuggestions(ctx context.Context, projectID string) ([]domain.GlossarySuggestion, error)
```

**도메인 모델:** `domain/glossary_suggestion.go`

```go
type GlossarySuggestion struct {
    ID            string
    ProjectID     string
    Term          string
    Pronunciation string
    Definition    string
    Category      string
    Status        string // pending, approved, rejected
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**LLM 프롬프트 전략:**
- 시나리오 텍스트 전체를 LLM에 전달
- 출력: JSON 배열 `[{term, pronunciation, definition, category}]`
- 기존 glossary 엔트리 목록을 컨텍스트에 포함하여 중복 방지

**CLI:**
```
yt-pipe glossary suggest <scp-id>   # LLM 추출 → pending 저장 → 결과 출력
yt-pipe glossary approve <scp-id>   # pending 항목 승인 → glossary.json 추가
```

**테스트 전략:**
- `SuggestTerms()`: LLM mock → JSON 파싱 → 기존 glossary diff 검증
- `ApproveSuggestion()`: pending → approved 전이 + glossary.json 쓰기 검증
- 중복 term 제안 시 UNIQUE 제약 처리 검증
- 빈 시나리오, LLM 에러, 잘못된 JSON 응답 엣지 케이스

### EFR3: 멀티모달 LLM 이미지 품질 자동 검증 — 아키텍처

**새 서비스:** `service/image_validator.go`

**서비스 설계:**

```go
// service/image_validator.go

type ImageValidatorService struct {
    llm    llm.LLM    // CompleteWithVision() 사용
    store  store.Store
    logger *slog.Logger
}

// ValidationResult represents the quality assessment of a generated image.
type ValidationResult struct {
    Score           int      // 0~100
    PromptMatch     int      // 프롬프트 대비 일치도 (0~100)
    CharacterMatch  int      // 캐릭터 외형 일관성 (0~100, 캐릭터 없으면 -1)
    TechnicalScore  int      // 기술적 결함 없음 (0~100)
    Reasons         []string // 점수 사유
    ShouldRegenerate bool    // Score < threshold
}

// ValidateImage evaluates a generated image against its prompt and character refs.
func (s *ImageValidatorService) ValidateImage(
    ctx context.Context,
    imagePath string,
    originalPrompt string,
    characterRefs []imagegen.CharacterRef,
) (*ValidationResult, error)

// ValidateAndRegenerate runs validation loop: validate → regenerate if below threshold (max attempts).
func (s *ImageValidatorService) ValidateAndRegenerate(
    ctx context.Context,
    projectID string,
    sceneNum int,
    shotNum int,
    maxAttempts int,
    threshold int,
) (*ValidationResult, error)
```

**검증 프롬프트 구조:**
```
[System] You are an image quality evaluator for SCP content.
Evaluate the image against the following criteria:
1. Prompt consistency (0-100): Does the image match the visual description?
2. Character appearance (0-100): Does the character match the ID card? (-1 if no character)
3. Technical quality (0-100): Are there distortions, artifacts, or rendering errors?

Return JSON: {"prompt_match": N, "character_match": N, "technical_score": N, "reasons": ["..."]}

[User]
Original prompt: {prompt}
Character references: {character descriptions}
[Image: {generated image}]
```

**설정:**
```yaml
image_validation:
  enabled: false          # 기본 비활성 (EFR3 옵트인)
  threshold: 70           # 이 점수 미만 시 재생성
  max_attempts: 3         # 최대 재생성 횟수
  model: "qwen-vl-max"   # Vision LLM 모델
```

**파이프라인 통합:**
- `service/image_gen.go`의 `GenerateShotImage()` 후 검증 루프 호출
- `image_validation.enabled == false`면 검증 스킵 (기존 동작 유지)
- 검증 결과를 `scene_manifests`에 `validation_score` 컬럼으로 기록

**새 테이블 컬럼:** `scene_manifests`에 `validation_score INTEGER` 추가 (마이그레이션 `008_validation_score.sql`)

```sql
ALTER TABLE scene_manifests ADD COLUMN validation_score INTEGER;
```

**테스트 전략:**
- `ValidateImage()`: Vision LLM mock → JSON 파싱 → 점수 계산 검증
- `ValidateAndRegenerate()`: threshold 미만 → 재생성 호출 검증, max attempts 초과 시 중단 검증
- `ErrNotSupported` 반환 시 검증 스킵 + 경고 로그 검증
- 캐릭터 없는 씬 (character_match = -1) → 가중 평균 계산 검증
- 잘못된 JSON, 점수 범위 초과, 이미지 파일 미존재 엣지 케이스

### EFR4: AI 품질 점수 기반 선택적 리뷰/자동 승인 — 아키텍처

**위치:** `service/approval.go` 확장

```go
// service/approval.go — 추가

// AutoApproveByScore auto-approves scenes with validation scores above threshold.
// Returns lists of auto-approved and review-required scenes.
func (s *ApprovalService) AutoApproveByScore(
    ctx context.Context,
    projectID string,
    assetType string,
    threshold int,
) (autoApproved []int, reviewRequired []int, err error)
```

**동작 흐름:**
1. 프로젝트의 모든 씬에 대해 `scene_manifests.validation_score` 조회
2. `score >= threshold` → `ApproveScene()` 호출 + 로그 "auto-approved (score: N)"
3. `score < threshold` → 리뷰 큐에 유지 (generated 상태)
4. `validation_score == NULL` (검증 미실행) → 리뷰 큐에 유지

**설정:**
```yaml
auto_approval:
  enabled: false                # 기본 비활성 (EFR4 옵트인)
  threshold: 80                 # 자동 승인 기준 점수
  initial_threshold: 90         # 초기 보수적 설정 권장
```

**EFR3 선행 의존성:**
- `auto_approval.enabled == true` && `image_validation.enabled == false` → 시작 시 경고 + 자동 승인 비활성화
- 설정 검증을 `config/` 로딩 시점에 수행

**테스트 전략:**
- threshold 경계값 테스트 (79, 80, 81)
- validation_score NULL 처리 검증
- 자동 승인 로그 메시지 포맷 검증
- EFR3 미활성 + EFR4 활성 시 경고 동작 검증
- 전체 씬 자동 승인 시 다음 상태 전이 트리거 검증

### EFR5: 배치 프리뷰 단일 승인 — 아키텍처

**새 서비스 메서드:** `service/approval.go` 확장

```go
// service/approval.go — 추가

// BatchPreviewItem contains preview data for a single scene.
type BatchPreviewItem struct {
    SceneNum        int
    ImagePath       string
    NarrationFirst  string   // 나레이션 첫 문장
    Mood            string
    ValidationScore *int     // EFR3 점수 (nil if not validated)
    Status          string   // generated, auto-approved, etc.
}

// GetBatchPreview returns preview data for all scenes in a project.
func (s *ApprovalService) GetBatchPreview(
    ctx context.Context,
    projectID string,
    assetType string,
) ([]BatchPreviewItem, error)

// BatchApprove approves all scenes except flagged ones.
// flaggedScenes: scene numbers that need rework.
func (s *ApprovalService) BatchApprove(
    ctx context.Context,
    projectID string,
    assetType string,
    flaggedScenes []int,
) (approved int, flagged int, err error)
```

**API 핸들러:**
```
GET  /api/projects/{projectId}/preview?asset_type=image   → BatchPreviewItem 목록
POST /api/projects/{projectId}/batch-approve               → { "asset_type": "image", "flagged_scenes": [3, 7] }
```

**CLI:**
```
yt-pipe review batch <scp-id> --asset image   # 배치 프리뷰 → 씬 목록 출력 → 플래그 입력 → 일괄 승인
```

**효율 추적:**
- 배치 승인 시 `total_scenes`, `flagged_count`, `auto_approved_count`를 slog에 기록
- 시간 경과에 따른 플래그 비율 추세로 자동 승인 효율 측정

**테스트 전략:**
- `GetBatchPreview()`: 씬 데이터 조립 정확성 검증
- `BatchApprove()`: 플래그 없는 씬 전부 승인, 플래그된 씬은 generated 유지 검증
- 빈 flaggedScenes → 전체 승인 검증
- 전체 씬 플래그 → 승인 0건 검증
- 존재하지 않는 sceneNum 플래그 시 에러 검증

### EFR6: FFmpeg 직접 영상 렌더링 — 아키텍처

**새 구현체:** `plugin/output/ffmpeg/ffmpeg.go`

기존 `output.Assembler` 인터페이스를 구현하는 새 구현체.

```go
// plugin/output/ffmpeg/ffmpeg.go

type FFmpegAssembler struct {
    ffmpegPath string     // FFmpeg 바이너리 경로
    logger     *slog.Logger
}

// New creates a new FFmpegAssembler after verifying ffmpeg availability.
func New(logger *slog.Logger) (*FFmpegAssembler, error)

// Assemble implements output.Assembler.
// Renders scene images + TTS audio + subtitles + BGM → MP4.
func (a *FFmpegAssembler) Assemble(ctx context.Context, input output.AssembleInput) (*output.AssembleResult, error)

// Validate implements output.Assembler.
func (a *FFmpegAssembler) Validate(ctx context.Context, outputPath string) error
```

**렌더링 파이프라인:**

```
1. 이미지 목록 생성 → images.txt (FFmpeg concat demuxer 포맷)
   file 'scene01_shot01.png'
   duration 3.5
   file 'scene01_shot02.png'
   duration 2.1
   ...

2. 오디오 결합 → audio_concat.txt (FFmpeg concat protocol)
   file 'scene01.wav'
   file 'scene02.wav'
   ...

3. 자막 파일 → subtitles.srt (기존 SubtitleService 출력 활용)

4. BGM 믹싱 → BGM 트랙 볼륨/페이드/덕킹 적용

5. FFmpeg 최종 명령:
   ffmpeg -f concat -safe 0 -i images.txt \
          -f concat -safe 0 -i audio_concat.txt \
          -i bgm_mixed.wav \
          -vf "subtitles=subtitles.srt:force_style='FontSize=24'" \
          -c:v libx264 -preset medium -crf 23 \
          -c:a aac -b:a 192k \
          -shortest -y output.mp4
```

**설정:**
```yaml
ffmpeg:
  preset: "medium"          # libx264 preset (ultrafast~veryslow)
  crf: 23                   # 품질 (0=무손실, 51=최저)
  audio_bitrate: "192k"
  resolution: "1920x1080"   # 출력 해상도
  fps: 30
  subtitle_font_size: 24
```

**플러그인 레지스트리 등록:**
```go
// plugin/registry.go init 또는 main.go
registry.Register("output", "ffmpeg", ffmpegFactory)
registry.Register("output", "capcut", capcutFactory)  // 기존
```

**출력 선택 설정:**
```yaml
output:
  provider: "capcut"   # "capcut" | "ffmpeg" | "both"
```

`provider: "both"` → CapCut 프로젝트와 MP4를 모두 생성

**파일 구조:**
```
plugin/output/
├── interface.go          # 기존 Assembler 인터페이스
├── capcut/
│   ├── capcut.go         # 기존 CapCut 구현체
│   └── types.go          # 기존 CapCut 타입
└── ffmpeg/
    ├── ffmpeg.go         # FFmpegAssembler 구현체
    ├── concat.go         # 이미지/오디오 concat 파일 생성
    ├── subtitle.go       # SRT 포맷 생성 (기존 데이터 → SRT 변환)
    └── bgm.go            # BGM 믹싱 FFmpeg 필터 생성
```

**ENFR1 준수:** 10씬 기준 MP4 출력 3분 이내 (1080p, 2 vCPU, 4GB RAM)
- `libx264 -preset medium -crf 23`은 이미지 슬라이드쇼 + 오디오에서 충분히 빠름
- 병목: 이미지 스케일링 + 자막 렌더링. 필요 시 `-preset fast`로 조정

**테스트 전략:**
- `checkFFmpegAvailable()`: PATH에 ffmpeg 없을 때 에러 메시지 검증
- `Assemble()`: 통합 테스트 — testdata 이미지/오디오로 실제 MP4 생성 (빌드 태그 `ffmpegtest`)
- concat 파일 생성 단위 테스트: 이미지 순서, duration 정확성
- SRT 변환 단위 테스트: 타이밍 포맷, 인코딩
- BGM 믹싱 필터 생성 단위 테스트: 볼륨/페이드/덕킹 파라미터
- `provider: "both"` 설정 시 두 구현체 모두 호출 검증
- 빈 씬, BGM 없는 프로젝트, 자막 없는 프로젝트 엣지 케이스

### New SQLite Tables & Migrations

**`007_glossary_suggestions.sql`** (EFR2):
```sql
CREATE TABLE glossary_suggestions (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL,
    term          TEXT NOT NULL,
    pronunciation TEXT NOT NULL,
    definition    TEXT,
    category      TEXT,
    status        TEXT NOT NULL CHECK(status IN ('pending','approved','rejected')),
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    UNIQUE(term, project_id)
);

CREATE INDEX idx_glossary_suggestions_status ON glossary_suggestions(status);
CREATE INDEX idx_glossary_suggestions_project ON glossary_suggestions(project_id);
```

**`008_validation_score.sql`** (EFR3):
```sql
ALTER TABLE scene_manifests ADD COLUMN validation_score INTEGER;
```

### New & Modified Files

**새 도메인 모델 (`domain/`):**
```
domain/
└── glossary_suggestion.go   # GlossarySuggestion 모델 (term, pronunciation, status)
```

**새 저장소 (`store/`):**
```
store/
├── glossary_suggestion.go   # 제안 CRUD + 상태 전이
└── migrations/
    ├── 007_glossary_suggestions.sql
    └── 008_validation_score.sql
```

**새 서비스 (`service/`):**
```
service/
├── glossary.go              # 용어 추출 + 제안 관리 (EFR2)
└── image_validator.go       # 이미지 품질 검증 + 재생성 루프 (EFR3)
```

**새 플러그인 (`plugin/`):**
```
plugin/output/ffmpeg/
├── ffmpeg.go                # FFmpegAssembler 구현체 (EFR6)
├── concat.go                # 이미지/오디오 concat 파일 생성
├── subtitle.go              # SRT 포맷 변환
└── bgm.go                   # BGM 믹싱 필터
```

**새 CLI 커맨드 (`cli/`):**
```
cli/
├── chapters.go              # yt-pipe chapters <scp-id> (EFR1)
├── glossary_cmd.go          # yt-pipe glossary suggest/approve (EFR2)
└── review.go                # yt-pipe review batch (EFR5)
```

**수정되는 기존 파일:**

| 파일 | 변경 내용 |
|------|----------|
| `plugin/llm/interface.go` | `VisionMessage`, `ContentPart` 타입 + `CompleteWithVision()` 메서드 추가 |
| `plugin/llm/openai.go` | `CompleteWithVision()` 구현 — OpenAI multimodal format 요청 |
| `plugin/llm/fallback.go` | `CompleteWithVision()` fallback chain 지원 |
| `service/timing.go` | `GenerateChapters()`, `SaveChaptersFile()` 추가 (EFR1) |
| `service/approval.go` | `AutoApproveByScore()`, `GetBatchPreview()`, `BatchApprove()` 추가 (EFR4, EFR5) |
| `service/image_gen.go` | 검증 루프 통합 — `image_validation.enabled` 시 `ValidateAndRegenerate()` 호출 (EFR3) |
| `config/types.go` | `ImageValidation`, `AutoApproval`, `FFmpeg` 설정 구조체 추가 |
| `api/routes.go` | 배치 프리뷰/승인 엔드포인트 등록 (EFR5) |
| `api/handlers/approval.go` | 배치 프리뷰/승인 핸들러 추가 (EFR5) |
| `Dockerfile` | `scratch` → `alpine` + FFmpeg 설치 (EFR6) |
| `plugin/registry.go` | `"output", "ffmpeg"` 팩토리 등록 |

### Plugin Interface Changes Summary

**LLM Interface (확장):**
```go
type LLM interface {
    // 기존
    Complete(ctx context.Context, messages []Message, opts CompletionOptions) (*CompletionResult, error)
    GenerateScenario(ctx context.Context, ...) (*domain.ScenarioOutput, error)
    RegenerateSection(ctx context.Context, ...) (*domain.SceneScript, error)

    // 추가 (EFR3)
    CompleteWithVision(ctx context.Context, messages []VisionMessage, opts CompletionOptions) (*CompletionResult, error)
}
```

**OutputAssembler Interface — 변경 없음.** FFmpeg는 기존 인터페이스의 새 구현체.

### Updated Config Structure

```yaml
# config.example.yaml — 추가 섹션

# EFR3: Image Quality Validation
image_validation:
  enabled: false
  threshold: 70
  max_attempts: 3
  model: "qwen-vl-max"

# EFR4: Auto Approval
auto_approval:
  enabled: false
  threshold: 80

# EFR6: FFmpeg Rendering
ffmpeg:
  preset: "medium"
  crf: 23
  audio_bitrate: "192k"
  resolution: "1920x1080"
  fps: 30
  subtitle_font_size: 24

# Output provider selection
output:
  provider: "capcut"   # "capcut" | "ffmpeg" | "both"
```

### Updated Data Flow

```
SCP Data (filesystem) → workspace/scp_data.go
    → service/scenario.go (+ plugin/llm/) → 시나리오 생성
        → service/glossary.go (+ plugin/llm/) → 용어사전 자동 확장 제안 [EFR2]
        → service/character.go → 씬별 캐릭터 매칭
        → service/mood.go (+ plugin/llm/) → 씬별 분위기 자동 매핑
        → service/approval.go [image_review]
            → service/image_gen.go (+ plugin/imagegen/ + CharacterRefs) → 씬별 이미지 생성
                → service/image_validator.go (+ plugin/llm/ Vision) → 품질 검증 [EFR3]
                    → (score < threshold) → 재생성 (max 3회)
            → service/approval.go → AutoApproveByScore() [EFR4]
            → service/approval.go → GetBatchPreview() + BatchApprove() [EFR5]
        → service/approval.go [tts_review]
            → service/tts.go (+ plugin/tts/ + MoodPreset) → 씬별 TTS 생성-승인
                → service/timing.go → 타이밍 해석
                    → service/timing.go → GenerateChapters() [EFR1]
                → service/subtitle.go → 자막 생성
        → service/bgm.go (+ plugin/llm/) → BGM 자동 추천
        → service/assembler.go → 출력 경로 선택:
            ├→ plugin/output/capcut/ → CapCut 프로젝트 (기존)
            └→ plugin/output/ffmpeg/ → MP4 직접 렌더링 [EFR6]
```

### Updated Dependencies

**새 의존 관계 (기존 규칙 준수):**
```
service/glossary.go         → store/, domain/, plugin/llm/, glossary/
service/image_validator.go  → store/, domain/, plugin/llm/ (Vision)
plugin/output/ffmpeg/       → domain/ (씬 모델 참조)
```

모든 새 의존 관계는 기존 의존 방향(`service/` → `store/`, `domain/`, `plugin/`)을 준수한다.

### TDD Implementation Strategy

**테스트 우선 순서 (각 EFR별):**

1. **도메인 모델 테스트** — 순수 구조체 + 유효성 검증
2. **저장소 테스트** — SQLite `:memory:` 기반 CRUD
3. **서비스 테스트** — 인터페이스 mock + 비즈니스 로직 검증
4. **플러그인 테스트** — 외부 호출 mock + 직렬화/역직렬화 검증
5. **CLI/API 테스트** — 통합 테스트 (서비스 mock)
6. **E2E 테스트** — 빌드 태그 분리 (`liveapi`, `ffmpegtest`)

**Mock 전략:**
- `mockery`로 LLM, Store 인터페이스 mock 자동 생성
- `CompleteWithVision()` mock: 고정 JSON 응답 반환
- FFmpeg 테스트: 실제 바이너리 필요한 테스트는 `//go:build ffmpegtest` 태그로 분리

**빌드 태그:**
- `ffmpegtest` — FFmpeg 바이너리가 필요한 통합 테스트
- `liveapi` — 실제 외부 API 호출 테스트 (기존)

### EFR Requirements Coverage

| EFR | 아키텍처 커버리지 | 상태 |
|-----|------------------|------|
| EFR1 YouTube Chapters | `service/timing.go` + `cli/chapters.go` | ✅ |
| EFR2 용어사전 자동확장 | `service/glossary.go` + `store/glossary_suggestion.go` + `cli/glossary_cmd.go` | ✅ |
| EFR3 이미지 품질 검증 | `service/image_validator.go` + `plugin/llm/` Vision 확장 | ✅ |
| EFR4 자동 승인 | `service/approval.go` 확장 + `config/` | ✅ |
| EFR5 배치 프리뷰 | `service/approval.go` + `api/handlers/approval.go` | ✅ |
| EFR6 FFmpeg 렌더링 | `plugin/output/ffmpeg/` + `Dockerfile` 변경 | ✅ |

| ENFR | 아키텍처 대응 | 상태 |
|------|-------------|------|
| ENFR1 10씬 MP4 3분 이내 | libx264 preset medium, CRF 23, 이미지 슬라이드쇼 최적화 | ✅ |
| ENFR2 이미지 검증 5초/장 | LLM API 응답 시간, timeout 설정 | ✅ |
| ENFR3 FFmpeg Docker 포함 | alpine + apk add ffmpeg, `checkFFmpegAvailable()` | ✅ |

### Implementation Sequence (EFR)

```
Phase 1 (MVP 추가):
  1. EFR1 YouTube Chapters — timing.go 확장 (독립, 최소 변경)
  2. EFR2 용어사전 자동확장 — 새 서비스 + 테이블 (독립)

Phase 2:
  3. LLM Vision 확장 — CompleteWithVision() (EFR3 전제)
  4. EFR3 이미지 품질 검증 — ImageValidatorService (3 의존)
  5. EFR4 자동 승인 — ApprovalService 확장 (EFR3 의존)
  6. EFR5 배치 프리뷰 — ApprovalService + API (EFR3 선택적 의존)
  7. EFR6 FFmpeg 렌더링 — 새 Assembler + Dockerfile (독립)
```

### Deferred Decisions Update (EFR)

기존 Deferred에 추가:
- R1 SFX → Phase 2 (BGMService 패턴 복제, SFX 라이브러리 외부 구축 선행)
- R2 동적 자막 → Phase 2 (CapCut TextMaterial 스타일 매핑)
- R3 문장 단위 동기화 → Phase 2 (Shot 데이터 존재, 조립 로직 수정)
- R4 썸네일 자동 생성 → Phase 2 (LLM + 이미지 합성)
- R5 스타일 프리셋 → Phase 2 (Config 확장)
- R6 Qwen 이미지 → Phase 2 (ImageGen 구현체 추가, DashScope API)
- R7 에셋 레지스트리 → Phase 2 (SQLite + 파일시스템)
- R8~R15 → Phase 3 (의존성 그래프는 PRD Enhancement 참조)
