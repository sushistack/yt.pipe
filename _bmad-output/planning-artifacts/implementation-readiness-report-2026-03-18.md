# Implementation Readiness Assessment Report — Enhancement Epics 19-22

**Date:** 2026-03-18
**Project:** youtube.pipeline
**Scope:** Epic 19-22 (EFR1-EFR6) — Enhancement PRD 기반 신규 에픽

---

## Document Inventory

**stepsCompleted:** [step-01-document-discovery, step-02-prd-analysis, step-03-epic-coverage-validation, step-04-ux-alignment, step-05-epic-quality-review, step-06-final-assessment]

### Documents Included in Assessment:

| Document | File | Size | Last Modified |
|----------|------|------|---------------|
| PRD Enhancement | prd-enhancement.md | 10,828 bytes | 2026-03-18 19:40 |
| Architecture | architecture.md | 86,474 bytes | 2026-03-18 19:53 |
| Epics & Stories | epics.md | 217,038 bytes | 2026-03-18 20:16 |
| PRD (원본, 참조용) | prd.md | 47,399 bytes | 2026-03-18 19:37 |

### Documents Not Applicable:

| Document | Reason |
|----------|--------|
| UX Design | N/A — CLI + API 프로젝트, UI 컴포넌트 없음 |

### Duplicate Conflicts: None

---

## PRD Analysis (Enhancement PRD)

### Enhancement Functional Requirements (EFR)

| ID | Category | Requirement |
|----|----------|-------------|
| EFR1 | Quick Win (Phase 1) | YouTube Chapters 자동 생성 — 씬 타이밍 데이터에서 YouTube 챕터 포맷 자동 생성, 프로젝트 출력 디렉토리에 챕터 텍스트 파일 출력 |
| EFR2 | Quick Win (Phase 1) | 용어 사전 자동 확장 — LLM 활용 SCP 전문 용어 자동 추출, 발음 가이드 제안, 기존 glossary.json diff 후 신규 항목만 제시, 승인 항목 자동 추가 |
| EFR3 | 인간 병목 해소 (Phase 2) | 멀티모달 LLM 이미지 품질 자동 검증 — Qwen-VL 등으로 프롬프트 일치도, 캐릭터 일관성, 기술적 결함 평가. 점수 0~100, 임계값 미만 시 자동 재생성 (최대 3회) |
| EFR4 | 인간 병목 해소 (Phase 2) | AI 품질 점수 기반 선택적 리뷰/자동 승인 — 고점수 씬 자동 승인, 저점수 씬만 인간 리뷰. auto_approve_threshold 설정 가능. EFR3 선행 필수 |
| EFR5 | 인간 병목 해소 (Phase 2) | 배치 프리뷰 단일 승인 — 전체 씬 프리뷰(이미지+나레이션+무드+AI점수) 후 문제 씬만 플래그, 나머지 일괄 승인. API 대시보드 연동 |
| EFR6 | CapCut 의존성 탈피 (Phase 2) | FFmpeg 직접 영상 렌더링 — 씬별 이미지+TTS+자막+BGM을 MP4로 직접 렌더링. 기존 출력 인터페이스의 새 구현체, CapCut과 병행 가능. 1920x1080 기본 |

**Total EFRs: 6** (EFR1-EFR6)

### Enhancement Non-Functional Requirements (ENFR)

| ID | Category | Requirement |
|----|----------|-------------|
| ENFR1 | 성능 | 직접 영상 렌더링 시 10씬 기준 MP4 출력 3분 이내 (1080p, Docker 2 vCPU, 4GB RAM) |
| ENFR2 | 성능 | 이미지 품질 자동 검증 시 이미지 1장당 5초 이내 (멀티모달 LLM API 응답 시간) |
| ENFR3 | 배포 | FFmpeg 외부 바이너리 Docker 이미지 포함, 로컬 미설치 시 명확한 에러 메시지 |

**Total ENFRs: 3** (ENFR1-ENFR3)

### Additional Requirements

| Category | Requirement |
|----------|-------------|
| 기존 PRD 관계 | 기존 FR1~FR61, NFR1~NFR24 변경 없이 유지. EFR 접두어로 구분 |
| 성공 지표 | 수동 개입 20% → 10% 이하, AI 자동 승인 80%+, 영상 출력 이중 경로, Chapters 자동 생성 |
| Phase 구분 | Phase 1 MVP 추가 (EFR1, EFR2), Phase 2 (EFR3-EFR6) |
| CLI 추가 | `yt-pipe chapters`, `yt-pipe glossary suggest/approve`, `yt-pipe render`, `yt-pipe review batch` |
| 의존성 | EFR3 → EFR4, EFR3 → EFR5 (선택적), EFR6 → R8/R12/R13 (Phase 3 언블록) |
| 로드맵 | R1~R15 (Phase 2~3) — 방향성 확인, 구현 시점에 개별 FR로 구체화 |

### PRD Enhancement Completeness Assessment

- **강점:** 기존 PRD와의 관계가 명확히 정의됨 (EFR 접두어 분리). 의존성 그래프가 시각화되어 구현 순서 파악 용이. ENFR이 측정 가능한 기준으로 정의됨. 리스크 완화 전략 포함.
- **관찰 사항:** 로드맵 항목 R1~R15는 에픽으로 구체화되지 않았으나, Enhancement PRD에서 "실제 구현 시점에 개별 FR로 구체화"라고 명시하여 적절함.

---

## UX Alignment Assessment

### UX Document Status

**Not Found** — N/A by design.

CLI + API 프로젝트로 UI 컴포넌트 없음. 이전 평가(2026-03-08)와 동일. Enhancement EFR도 CLI 명령어 + REST API 엔드포인트로 구성되어 UX 문서 불필요.

### Alignment Issues

None. Enhancement PRD의 CLI 명령어(`yt-pipe chapters`, `yt-pipe glossary suggest/approve`, `yt-pipe render`, `yt-pipe review batch`)와 API 엔드포인트(`GET /preview`, `POST /batch-approve`)가 기존 패턴과 일관됨.

---

## Epic Coverage Validation (EFR)

### Coverage Matrix

| EFR | PRD Requirement | Epic Coverage | Stories | Status |
|-----|----------------|---------------|---------|--------|
| EFR1 | YouTube Chapters 자동 생성 | Epic 19 | 19.1 | ✓ Covered |
| EFR2 | 용어 사전 자동 확장 | Epic 19 | 19.2, 19.3, 19.4 | ✓ Covered |
| EFR3 | 멀티모달 LLM 이미지 품질 자동 검증 | Epic 20 | 20.1, 20.2, 20.3, 20.4, 20.5 | ✓ Covered |
| EFR4 | AI 품질 점수 기반 자동 승인 | Epic 21 | 21.1 | ✓ Covered |
| EFR5 | 배치 프리뷰 단일 승인 | Epic 21 | 21.2, 21.3, 21.4 | ✓ Covered |
| EFR6 | FFmpeg 직접 영상 렌더링 | Epic 22 | 22.1, 22.2, 22.3, 22.4 | ✓ Covered |

### ENFR Coverage Matrix

| ENFR | PRD Requirement | Epic Coverage | Verification Point | Status |
|------|----------------|---------------|--------------------|--------|
| ENFR1 | 10씬 MP4 3분 이내 | Epic 22 | Story 22.4 AC에 명시 | ✓ Covered |
| ENFR2 | 이미지 검증 5초/장 | Epic 20 | Story 20.3 (LLM API 응답 시간) | ✓ Covered |
| ENFR3 | FFmpeg Docker 포함 | Epic 22 | Story 22.1 AC에 명시 | ✓ Covered |

### Missing Requirements

None — All 6 EFRs and 3 ENFRs are covered in epics 19-22.

### Observations

- **EFR 분배:** 4개 에픽에 걸쳐 6 EFR이 논리적으로 그룹화됨
  - Epic 19 (Quick Wins): EFR1, EFR2 (2 EFRs, 4 stories)
  - Epic 20 (Image Validation): EFR3 (1 EFR, 5 stories)
  - Epic 21 (Approval): EFR4, EFR5 (2 EFRs, 4 stories)
  - Epic 22 (FFmpeg): EFR6 (1 EFR, 4 stories)
- **EFR3의 높은 스토리 수:** 5개 스토리로 가장 세분화됨 — LLM Vision 인터페이스 확장, 도메인 모델, 서비스 코어, 재생성 루프, 파이프라인 통합으로 구성. 복잡도 대비 적절한 분할.
- **CLI/API 이중 경로:** EFR5의 배치 리뷰가 CLI(21.3)와 API(21.4)로 분리되어 기존 프로젝트의 이중 인터페이스 패턴과 일관됨.

### Coverage Statistics

- **Total Enhancement FRs:** 6 (EFR1-EFR6)
- **EFRs covered in epics:** 6
- **Coverage percentage:** 100%
- **Total Enhancement NFRs:** 3 (ENFR1-ENFR3)
- **ENFRs covered in epics:** 3
- **ENFR coverage percentage:** 100%

---

## Epic Quality Review

### Epic Structure Validation

#### A. User Value Focus Check

| Epic | Title | User-Centric? | Assessment |
|------|-------|:---:|------------|
| Epic 19 | YouTube Optimization Quick Wins | ✓ | "크리에이터가 YouTube 챕터를 자동 생성하고, 용어 사전을 자동 확장" — 명확한 사용자 가치 |
| Epic 20 | AI Image Quality Validation | ⚠️ Partial | 제목은 시스템 관점("AI Image Quality Validation"). 목표는 사용자 가치("이미지 리뷰 부담을 대폭 줄일 수 있다")이나 제목이 기술적 |
| Epic 21 | Automated Approval & Batch Review | ✓ | "수동 개입을 20% → 10% 이하로 축소" — 수치화된 사용자 가치 |
| Epic 22 | FFmpeg Direct Video Rendering | ✓ | "CapCut 없이... 자동화된 영상 출력 경로를 확보" — 명확한 사용자 가치 |

#### B. Epic Independence Validation

| Epic | Dependencies | Direction | Status |
|------|-------------|-----------|--------|
| Epic 19 | 없음 | — | ✓ Standalone |
| Epic 20 | 없음 (LLM Vision 확장을 첫 스토리로 포함) | — | ✓ Self-contained |
| Epic 21 | Epic 20 (EFR3 검증 점수) | Backward | ✓ Valid |
| Epic 22 | 없음 | — | ✓ Standalone |

**No forward dependencies detected.** Epic 21만 Epic 20에 후방 의존하며, Epic 19와 Epic 22는 완전 독립. Epic 20과 22는 병렬 착수 가능.

### Story Quality Assessment

#### A. Story Sizing & Persona

**Total Stories: 17** across 4 epics (4+5+4+4)

| Persona | Story Count | Stories |
|---------|:-----------:|---------|
| "As a content creator" | 7 | 19.1, 19.3, 19.4, 20.5, 21.1, 21.3, 22.4 |
| "As a system" | 7 | 19.2, 20.1, 20.2, 20.3, 20.4, 22.2, 22.3 |
| "As a system operator" | 1 | 22.1 |
| "As a n8n workflow orchestrator" | 1 | 21.4 |
| "As a content creator" (implied) | 1 | 21.2 |

**관찰:** 17개 스토리 중 7개(41%)가 "As a system" 페르소나 사용. 기존 에픽 1~18에서도 기술 기반 스토리에 "As a developer"를 사용했으나, Enhancement 에픽에서는 "As a system"이 더 빈번. 대부분 도메인 모델/저장소/서비스 코어 스토리로 기술적 기반 작업이므로 이해할 수 있으나, 가능하면 사용자 가치 관점으로 리프레이밍 권장.

#### B. Acceptance Criteria Review

**강점:**
- 모든 17개 스토리가 Given/When/Then BDD 포맷 사용
- 에러 조건, 엣지 케이스 포함 (빈 입력, 존재하지 않는 씬, 잘못된 JSON 등)
- 측정 가능한 결과 (점수 범위 0~100, 임계값 명시 등)
- EFR 추적성이 핵심 스토리에 명시 ("this satisfies EFR1", "this satisfies EFR2" 등)

### Dependency Analysis

#### Within-Epic Dependencies

- **Epic 19:** 19.1 독립 | 19.2 → 19.3 → 19.4 순차 (도메인→추출→승인)
- **Epic 20:** 20.1 → 20.2 → 20.3 → 20.4 → 20.5 순차 (인터페이스→모델→서비스→루프→통합)
- **Epic 21:** 21.1 독립 | 21.2 → 21.3, 21.4 (프리뷰→CLI, API 병렬)
- **Epic 22:** 22.1 → 22.2, 22.3 → 22.4 (Docker→concat/BGM→통합)

**No forward references within epics detected.** 모든 스토리가 선행 스토리 위에 구축.

#### Database/Entity Creation Timing

| Story | Migration | Table/Column | Assessment |
|-------|-----------|--------------|------------|
| 19.2 | 014_glossary_suggestions.sql | `glossary_suggestions` 테이블 | ✓ 필요 시점에 생성 |
| 20.2 | 015_validation_score.sql | `shot_manifests.validation_score` 컬럼 | ⚠️ 테이블명 불일치 (아래 참조) |

### Quality Findings by Severity

#### 🔴 Critical Violations

**None.**

#### 🟠 Major Issues

**1. 테이블명 불일치: `shot_manifests` vs `scene_manifests`**

Story 20.2와 20.4는 `shot_manifests` 테이블을 참조하나, 아키텍처 증분 업데이트(architecture.md:1455)는 `scene_manifests`에 `validation_score` 컬럼을 추가하는 마이그레이션을 정의:

```
아키텍처: ALTER TABLE scene_manifests ADD COLUMN validation_score INTEGER;
Story 20.2: "`shot_manifests` table gains a `validation_score INTEGER` column"
```

또한 Story 21.1의 아키텍처 참조도 `scene_manifests.validation_score` 조회를 기술.

**Impact:** 구현 시 어떤 테이블에 컬럼을 추가할지 혼란. `shot_manifests`와 `scene_manifests`가 다른 테이블이라면 데이터 저장 위치가 달라짐.

**Recommendation:** 프로젝트 코드베이스의 실제 테이블 구조를 확인하여 정확한 테이블명으로 통일. 아키텍처 또는 에픽 중 하나를 수정.

**2. 마이그레이션 번호 불일치: 아키텍처 vs 에픽**

| 항목 | 아키텍처 번호 | 에픽 번호 |
|------|:----------:|:-------:|
| glossary_suggestions | 007 | 014 |
| validation_score | 008 | 015 |

아키텍처 증분 업데이트가 기존 에픽 1~18의 마이그레이션을 고려하지 않고 독립적으로 번호를 부여함. 에픽의 014, 015가 올바른 번호로 추정.

**Impact:** 아키텍처 문서를 참조하여 마이그레이션을 작성하면 기존 마이그레이션과 충돌 가능.

**Recommendation:** 아키텍처 문서의 마이그레이션 번호를 에픽과 일치시키도록 업데이트 (007→014, 008→015).

#### 🟡 Minor Concerns

**1. "As a system" 페르소나 과다 사용 (7/17 = 41%)**

기존 에픽에서 "As a developer"를 5/38 = 13%로 사용한 것에 비해 높은 비율. 도메인 모델/저장소 스토리는 기술적이지만, 다음과 같이 리프레이밍 가능:
- 19.2 → "As a creator, I want glossary suggestions tracked so I can approve them later"
- 20.1 → "As a creator, I want the pipeline to understand images so it can evaluate quality"

**2. Epic 20 제목 리프레이밍 제안**

현재: "AI Image Quality Validation" (기술 관점)
제안: "Creator Can Trust AI to Catch Bad Images" 또는 "Automated Image Quality Gates"

**3. Story 22.4 범위 — 기존 `assembler.go` 수정 포함**

Story 22.4가 FFmpegAssembler 구현 외에 기존 `service/assembler.go`의 `Assemble()` 메서드도 수정 (~10줄). 새 Assembler 플러그인 구현과 기존 코드 수정이 같은 스토리에 있음. 변경 범위가 작아(~10줄) 분리 불필요하나 주의 필요.

**4. Story 20.4 — 아키텍처 시그니처와 콜백 패턴 차이**

아키텍처의 `ValidateAndRegenerate()` 시그니처에는 `regenerateFn` 콜백이 없으나, Story 20.4는 순환 의존성 방지를 위해 콜백 패턴을 명시. 스토리의 설계가 더 구체적이며 올바른 방향이나, 아키텍처 업데이트 필요.

### Best Practices Compliance Checklist

| Criteria | Epic 19 | Epic 20 | Epic 21 | Epic 22 |
|----------|:-------:|:-------:|:-------:|:-------:|
| Delivers user value | ✓ | ⚠️ | ✓ | ✓ |
| Functions independently | ✓ | ✓ | ✓ | ✓ |
| Stories appropriately sized | ✓ | ✓ | ✓ | ✓ |
| No forward dependencies | ✓ | ✓ | ✓ | ✓ |
| DB tables created when needed | ✓ | ⚠️* | ✓ | ✓ |
| Clear acceptance criteria | ✓ | ✓ | ✓ | ✓ |
| EFR traceability maintained | ✓ | ✓ | ✓ | ✓ |

*테이블명 불일치 이슈

### Recommendations Summary

1. **[Major]** `shot_manifests` vs `scene_manifests` 테이블명 통일 — 아키텍처 또는 에픽 문서 수정
2. **[Major]** 마이그레이션 번호 통일 — 아키텍처 007/008 → 014/015로 업데이트
3. **[Minor]** Epic 20 제목을 사용자 관점으로 리프레이밍
4. **[Minor]** "As a system" 스토리를 사용자 가치 관점으로 리프레이밍 고려
5. **[Minor]** 아키텍처의 `ValidateAndRegenerate()` 시그니처에 콜백 패턴 반영

---

## Summary and Recommendations

### Overall Readiness Status

## ✅ READY — with corrections recommended

Enhancement 에픽 19~22(EFR1~EFR6)는 **강한 구현 준비 상태**를 보여줍니다. PRD Enhancement, 아키텍처 증분 업데이트, 에픽/스토리가 잘 정렬되어 있으며, 기존 프로젝트 아키텍처를 자연스럽게 확장합니다.

### Assessment Summary

| Area | Finding | Status |
|------|---------|--------|
| Document Completeness | PRD Enhancement, Architecture Incremental, Epics 모두 존재. UX N/A. | ✅ Complete |
| EFR Coverage | 6/6 EFRs (100%) + 3/3 ENFRs (100%) covered across 4 epics | ✅ Full Coverage |
| Epic User Value | 3/4 epics clearly user-centric; Epic 20 제목만 기술적 | ⚠️ Minor |
| Epic Independence | 모든 의존성 후방. Epic 20/22 병렬 가능. 순방향 의존 없음 | ✅ Clean |
| Story Quality | 17 stories with BDD ACs, proper sizing, edge cases covered | ✅ Strong |
| Architecture Alignment | 테이블명 불일치(shot/scene), 마이그레이션 번호 불일치 | 🟠 Needs Fix |
| EFR Traceability | 핵심 스토리에 EFR/ENFR 참조 포함 | ✅ Correct |

### Critical Issues Requiring Immediate Action

**None.** 구현을 차단하는 크리티컬 이슈 없음.

### Issues Requiring Attention Before Implementation

1. **[Major] 테이블명 불일치** — `shot_manifests` (에픽) vs `scene_manifests` (아키텍처). 코드베이스의 실제 테이블명을 확인하여 문서 통일 필요. 잘못된 테이블에 컬럼 추가하면 런타임 에러 발생.

2. **[Major] 마이그레이션 번호 불일치** — 아키텍처(007/008) vs 에픽(014/015). 에픽 번호가 올바른 것으로 추정. 아키텍처 문서 업데이트 필요.

### Optional Improvements

3. **[Minor]** Epic 20 제목을 사용자 관점으로 리프레이밍 ("AI Image Quality Validation" → "Automated Image Quality Gates")
4. **[Minor]** "As a system" 페르소나 스토리(7개) 리프레이밍 고려
5. **[Minor]** 아키텍처 `ValidateAndRegenerate()` 시그니처에 콜백 패턴 반영

### Recommended Next Steps

1. **코드베이스에서 실제 테이블명 확인** — `shot_manifests` 또는 `scene_manifests` 중 올바른 이름 특정 후 문서 통일 (5분)
2. **아키텍처 마이그레이션 번호 업데이트** — 007→014, 008→015 (2분)
3. **구현 착수** — Epic 19 Story 19.1 (YouTube Chapters, 독립/최소 변경)부터 시작, 또는 Epic 19와 Epic 22를 병렬 진행

### Strengths Noted

- **기존 아키텍처와 완벽한 정합** — 플러그인 인터페이스 패턴, `ErrNotSupported` 패턴, 설정 구조 등 기존 설계 원칙을 일관되게 따름
- **점진적 옵트인 설계** — EFR3/EFR4 모두 `enabled: false` 기본값으로 기존 동작에 영향 없음
- **의존성 관리 우수** — LLM Vision 확장을 Epic 20 첫 스토리로 포함하여 자기 완결적
- **콜백 패턴 (Story 20.4)** — `regenerateFn` 콜백으로 ImageValidator↔ImageGen 순환 의존성 회피. 좋은 설계 결정
- **이중 출력 경로** — `output.provider: "both"` 옵션으로 CapCut↔FFmpeg 병행 가능. 점진적 전환 지원
- **엣지 케이스 커버리지** — 모든 스토리에 에러 조건, 비활성 시 동작, 빈 입력 등 엣지 케이스 AC 포함
- **Phase 3 언블록** — EFR6가 R8(CapCut 완전 대체), R12(승인 전방 이동), R13(CI/CD) 세 가지 로드맵 항목을 언블록

### Final Note

이 평가에서 **2개 Major 이슈**(테이블명, 마이그레이션 번호)와 **3개 Minor 이슈**를 식별했습니다. Major 이슈는 모두 **문서 불일치**로 구현 로직 자체에는 영향 없으며, 10분 이내에 해결 가능합니다. **프로젝트는 구현 준비가 완료되었습니다.**

---

*Assessment completed: 2026-03-18*
*Assessor: Implementation Readiness Validator*
*Documents reviewed: prd-enhancement.md, architecture.md (EFR incremental update), epics.md (Epics 19-22)*
*Previous assessment: implementation-readiness-report-2026-03-08.md (Epics 1-7, all passed)*
