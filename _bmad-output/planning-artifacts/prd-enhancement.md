---
stepsCompleted:
  - step-e-01-discovery
  - step-e-02-review
  - step-e-03-edit
classification:
  projectType: cli_tool + api_backend
  projectTypePrimary: cli_tool
  projectTypeSecondary: api_backend
  domain: ai_content_pipeline
  complexity: medium
  projectContext: enhancement
parentPRD: _bmad-output/planning-artifacts/prd.md
inputDocuments:
  - _bmad-output/brainstorming/brainstorming-session-2026-03-17-1000.md
  - _bmad-output/planning-artifacts/brainstorming-feasibility-analysis-2026-03-18.md
workflowType: 'prd'
lastEdited: '2026-03-18'
editHistory:
  - date: '2026-03-18'
    changes: 'Initial creation — brainstorming session (2026-03-17) results. 6 core FRs + 15 roadmap items from 25 confirmed ideas. Feasibility analysis completed.'
---

# Product Requirements Document — youtube.pipeline Enhancement

**Author:** Jay
**Date:** 2026-03-18
**Parent PRD:** `prd.md` (61 FR + 24 NFR)
**Scope:** 기존 파이프라인 위에 자동화 확장 + 영상 퀄리티 향상

## Executive Summary

기존 youtube.pipeline MVP(SCP ID → CapCut 프로젝트)가 동작하는 상태에서, 두 가지 핵심 과제를 해결한다:

1. **인간 병목 최소화** — AI 자동 품질 검증 + 선택적 승인으로 수동 개입을 20% → 10% 이하로 축소
2. **CapCut 의존성 탈피** — FFmpeg 기반 직접 영상 렌더링으로 프리뷰/완성본 출력 경로 확보

부가적으로 YouTube Chapters 자동 생성, 용어 사전 자동 확장 등 Quick Win을 MVP에 포함하고, SFX/동적 자막/썸네일 자동 생성 등은 로드맵으로 관리한다.

### 기존 PRD와의 관계

- 기존 `prd.md`의 FR1~FR61, NFR1~NFR24는 **변경 없이 유지**
- 본 문서의 FR은 `EFR` (Enhancement FR) 접두어로 구분
- 기존 인터페이스/플러그인 아키텍처 위에 확장하는 구조

## 성공 기준

### 추가 성공 지표

| 지표 | 현재 (prd.md 목표) | Enhancement 목표 |
|------|-------------------|-----------------|
| 수동 개입 비율 | 20% 이하 | **10% 이하** (기존 FR44와 동일 방식으로 측정) |
| 이미지 리뷰 시간 | 전수 검사 | **AI 자동 승인 80%+, 인간 리뷰 20% 미만** |
| 영상 출력 경로 | CapCut 단일 | **CapCut + FFmpeg MP4 이중 경로** |
| YouTube 최적화 | 수동 | **Chapters 자동 생성** |

## Functional Requirements — Core (PRD에 FR로 추가)

### Quick Wins (Phase 1 MVP 추가)

- **EFR1: YouTube Chapters 자동 생성** — 시스템은 씬 타이밍 데이터에서 YouTube 챕터 포맷(`0:00 제목\n1:23 제목...`)을 자동 생성할 수 있다. 씬별 시작 시간과 씬 설명(Mood/VisualDesc 기반)을 챕터 타임스탬프와 제목으로 변환하여 프로젝트 출력 디렉토리에 챕터 텍스트 파일로 출력

- **EFR2: 용어 사전 자동 확장** — 시스템은 시나리오 생성 완료 후, LLM을 활용해 시나리오 텍스트에서 SCP 전문 용어를 자동 추출하고 발음 가이드를 제안할 수 있다. 기존 glossary.json과 diff하여 신규 항목만 크리에이터에게 제시하고, 승인된 항목을 glossary에 자동 추가. 기존 FR14(TTS 발음 오버라이드)를 보강

### 인간 병목 해소 (Phase 2)

- **EFR3: 멀티모달 LLM 이미지 품질 자동 검증** — 시스템은 이미지 생성 후 멀티모달 LLM(Qwen-VL 등)으로 생성된 이미지를 자동 평가할 수 있다. 평가 항목: (1) 프롬프트 대비 이미지 일치도, (2) 캐릭터 ID카드 대비 외형 일관성, (3) 기술적 결함(왜곡, 아티팩트). 점수(0~100) + 사유를 반환하며, 설정된 임계값(기본 70, 설정 가능) 미만 시 자동 재생성 (최대 3회). 이미지 생성 인터페이스와 별도의 검증 인터페이스로 구현

- **EFR4: AI 품질 점수 기반 선택적 리뷰/자동 승인** — 시스템은 EFR3의 검증 점수를 기반으로 고점수 씬을 자동 승인하고 저점수 씬만 인간 리뷰 큐에 올릴 수 있다. 설정: `auto_approve_threshold` (기본 80, 설정 가능). 자동 승인된 씬은 승인 로그에 "auto-approved (score: N)" 기록. 초기에는 임계값을 높게(90) 설정하고 점진적으로 조정. EFR3 선행 필수

- **EFR5: 배치 프리뷰 단일 승인** — 크리에이터는 전체 씬을 한번에 프리뷰하고, 문제가 있는 씬만 플래그하여 나머지를 일괄 승인할 수 있다. 프리뷰 목록은 씬별 이미지 경로 + 나레이션 첫 문장 + 무드 + AI 검증 점수(EFR3 활성 시)를 포함. API 대시보드(기존 FR56)와 연동하여 브라우저 기반 프리뷰 제공. 배치 승인 시 전체 씬 대비 플래그된 씬 비율을 추적하여 자동 승인 효율을 측정

### CapCut 의존성 탈피 (Phase 2)

- **EFR6: 직접 영상 렌더링** — 시스템은 씬별 이미지 + TTS 오디오 + 자막 + BGM을 MP4 영상으로 직접 렌더링할 수 있다. 기존 출력 조립 인터페이스의 새 구현체로 제공하며, CapCut 출력과 병행 가능 (설정으로 선택). 출력 해상도 1920x1080, 표준 비디오/오디오 코덱 기본, 설정으로 조정 가능

## Functional Requirements — Roadmap

아래 항목은 방향성이 확인된 기능으로, 실제 구현 시점에 개별 FR로 구체화한다.

### Phase 2 로드맵

| ID | 기능 | 설명 | 선행 의존성 |
|----|------|------|-----------|
| R1 | SFX 자동 삽입 | Scene.Mood 기반 SFX(문삐걱, 경보음 등) 자동 배치. BGMService 패턴 복제. 초기엔 씬 단위 ambient, 추후 문장 단위 정밀화 | SFX 라이브러리 구축 |
| R2 | 동적 자막 스타일링 | Scene.Mood에 따라 자막 색상/크기 변경 (horror→빨간, calm→흰색). CapCut TextMaterial 스타일 매핑 | 없음 |
| R3 | 문장 단위 이미지-자막 동기화 | 씬 단위 → Shot 단위 이미지 전환 정밀화. Shot.StartSec/EndSec 데이터 이미 존재, 조립 로직 수정 | 없음 |
| R4 | 썸네일+제목+설명 자동 생성 | 가장 임팩트 있는 씬 이미지 선별 → 텍스트 오버레이 → YouTube 최적화 메타데이터 LLM 생성 | LLM |
| R5 | 스타일 프리셋 시스템 | 이미지 스타일, 자막 스타일, TTS 무드, 컬러 톤을 하나의 프리셋으로 묶어 관리. 기존 FR34 확장 | 없음 |
| R6 | Qwen 이미지 프로바이더 | SiliconFlow FLUX 대안으로 Qwen 이미지 생성 모델 추가. DashScope API 기반(TTS와 동일 에코시스템) | DashScope API |
| R7 | 에셋 레지스트리 (기본) | 프로젝트 간 공유 에셋(이미지, SFX, BGM, 캐릭터 ID카드) 관리. SQLite 메타데이터 + 파일시스템, 해시 기반 중복 제거 | 없음 |

### Phase 3 로드맵

| ID | 기능 | 설명 | 선행 의존성 |
|----|------|------|-----------|
| R8 | CapCut 완전 대체 | FFmpeg/Remotion으로 트랜지션/이펙트까지 처리하여 CapCut 의존 완전 제거 | EFR6 |
| R9 | 핵심 씬 i2v | Wan Video first frame 방식으로 핵심 이미지 2~3장을 2~4초 동영상 클립으로 변환. Shot.VideoPath 필드 이미 존재 | VideoGen 플러그인 |
| R10 | AI 이미지 트랜지션 | 씬 전환 시 두 이미지 사이 보간 프레임 생성 (FILM, RIFE 등). 실험적 기능 | 보간 모델 |
| R11 | 역방향/이미지 드리븐 파이프라인 | Google 이미지 검색 → 레퍼런스 수집 → 핵심 이미지 생성 → 시나리오 역생성. 파이프라인 흐름 자체 변경 | Google API, Qwen-VL |
| R12 | 승인 전방 이동 (트레일러) | 시나리오 승인 시 30초 트레일러(대표 이미지 + TTS 샘플) 자동 생성으로 조기 방향 검증 | EFR6 |
| R13 | CI/CD 패턴 영상 파이프라인 | SCP ID push → 자동 파이프라인 → 품질 게이트 → staging → approval → production | EFR4, EFR6, FR30 |
| R14 | 듀얼 프로파일 (Quick/Premium) | Quick: 빠른 기본 품질. Premium: i2v, SFX, 컬러 그레이딩 등 전부 적용. `--profile` 플래그 | Phase 2 기능들 |
| R15 | 콘텐츠 캘린더 + 스케줄링 | 주간/월간 콘텐츠 일정 관리 + cron 기반 자동 파이프라인 트리거 | R13 |

## Phase 1 MVP 추가 — CLI 명령어

| 명령어 | 설명 |
|--------|------|
| `yt-pipe chapters <scp-id>` | YouTube Chapters 텍스트 생성 |
| `yt-pipe glossary suggest <scp-id>` | 시나리오 기반 용어 사전 확장 제안 |
| `yt-pipe glossary approve <scp-id>` | 제안된 용어 승인 및 사전 추가 |

## Phase 2 추가 — CLI 명령어

| 명령어 | 설명 |
|--------|------|
| `yt-pipe render <scp-id> [--format mp4\|capcut]` | FFmpeg 렌더링 또는 CapCut 조립 선택 |
| `yt-pipe review batch <scp-id>` | 전체 씬 배치 프리뷰 + 선택적 승인 |

## 비기능 요구사항 — 추가

- **ENFR1:** 직접 영상 렌더링 시 10씬 기준 MP4 출력 시간은 3분 이내 (1080p). Docker 컨테이너 기준(2 vCPU, 4GB RAM)에서 렌더 시작~파일 완성 경과 시간으로 측정
- **ENFR2:** 이미지 품질 자동 검증(EFR3) 시 이미지 1장당 검증 소요 시간은 5초 이내. 멀티모달 LLM API 응답 시간으로 측정
- **ENFR3:** 영상 렌더링에 필요한 외부 바이너리는 Docker 이미지에 포함하며, 로컬 개발 환경에서는 시스템 설치본을 사용. 미설치 시 명확한 에러 메시지 출력

## 의존성 그래프

```
Phase 1 (MVP 추가)
  EFR1 YouTube Chapters ── 독립
  EFR2 용어사전 자동확장 ── 독립

Phase 2
  EFR6 FFmpeg 렌더링 ──→ R8 CapCut 완전 대체 (Phase 3)
      │──→ R12 승인 전방 이동 (Phase 3)
      │──→ R13 CI/CD 패턴 (Phase 3)

  EFR3 Qwen-VL 검증 ──→ EFR4 자동 승인 ──→ R13 CI/CD (Phase 3)
                    ──→ EFR5 배치 프리뷰

  R1 SFX ──→ R14 듀얼 프로파일 (Phase 3)
  R2 동적 자막 ──→ R14
  R5 스타일 프리셋 ──→ R14

Phase 2 권장 구현 순서:
  1. EFR6 FFmpeg (Phase 3 다수 기능 언블록)
  2. R6 Qwen 이미지 + EFR3 Qwen-VL 검증 (동일 API 에코시스템)
  3. EFR4 자동 승인 (EFR3 의존)
  4. R3 문장 동기화 + R2 동적 자막 (관련 조립 변경)
  5. R1 SFX + R5 스타일 프리셋
  6. R4 썸네일 + EFR5 배치 승인 + R7 에셋 레지스트리
```

## 리스크 완화 전략

- **FFmpeg 통합 리스크:** 이미지 슬라이드쇼 + 오디오는 FFmpeg의 가장 기본적인 유스케이스. 기존 `output.Assembler` 인터페이스에 맞춰 플러그인으로 구현하므로 기존 CapCut 경로에 영향 없음
- **Qwen-VL 평가 주관성:** 프롬프트 엔지니어링으로 평가 기준 객관화. 초기 임계값을 높게 설정(90)하고 점진적으로 조정
- **SFX 라이브러리 구축:** 로열티 프리 SFX 수집이 외부 작업. freesound.org 등 활용, 무드별 최소 5개씩 확보 후 진행
