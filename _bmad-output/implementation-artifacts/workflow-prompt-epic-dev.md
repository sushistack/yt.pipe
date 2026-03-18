# Epic Development Workflow — Copy-Paste Prompts

에픽 단위로 **새 컨텍스트 윈도우**에 복붙. 모든 스토리를 순차적으로 자동 처리한다.

---

# Epic 13: Prompt Template Management

```
Epic 13 (Prompt Template Management)의 모든 스토리(13-1 ~ 13-5)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-13 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 13 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 13-1: Prompt Template Domain Model & Database Migration
- 13-2: Prompt Template Store — CRUD & Version Management
- 13-3: Prompt Template Service — Business Logic
- 13-4: Default Template Auto-Installation
- 13-5: Prompt Template CLI Commands

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 각 스토리는 이전 스토리 완료 후 시작 (의존성 체인)
- 기존 코드 컨벤션: testify, domain errors, 인터페이스 뒤 외부 의존성
- DB migration: 002_templates.sql (Epic 6의 기존 001 이후)
```

---

# Epic 14: Character ID Card System

```
Epic 14 (Character ID Card System)의 모든 스토리(14-1 ~ 14-6)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-14 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 14 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 14-1: ImageGen Plugin Interface Extension
- 14-2: Character Domain Model & Database Migration
- 14-3: Character Store — CRUD & Alias Search
- 14-4: Character Service — CRUD & Scene Text Matching
- 14-5: Image Service Integration — Character Auto-Reference
- 14-6: Character CLI Commands

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 14-1: ImageGen 인터페이스 확장 시 기존 SiliconFlow 구현체 하위 호환성 필수
- DB migration: 003_characters.sql (Epic 13의 002 이후)
```

---

# Epic 15: TTS Mood Presets

```
Epic 15 (TTS Mood Presets)의 모든 스토리(15-1 ~ 15-5)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-15 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 15 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 15-1: TTS Plugin Interface Extension
- 15-2: Mood Preset Domain Model & Database Migration
- 15-3: Mood Preset Store — CRUD & Scene Assignment
- 15-4: Mood Service — Preset Management & LLM Auto-Mapping
- 15-5: TTS Service Integration & CLI Commands

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 15-1: TTS 인터페이스 확장 시 기존 DashScope 구현체 하위 호환성 필수
- 15-4: LLM 플러그인을 통한 mood auto-mapping — 기존 LLM 인터페이스 활용
- DB migration: 004_mood_presets.sql (Epic 14의 003 이후)
```

---

# Epic 17: BGM Preset Library

```
Epic 17 (BGM Preset Library)의 모든 스토리(17-1 ~ 17-6)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-17 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 17 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 17-1: OutputAssembler Plugin Interface Extension
- 17-2: BGM Domain Model & Database Migration
- 17-3: BGM Store — CRUD, Tag Search & Scene Assignment
- 17-4: BGM Service — Management & LLM Auto-Recommendation
- 17-5: CapCut Assembler Integration — BGM Placement & Credits
- 17-6: BGM CLI Commands

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 17-1: OutputAssembler 인터페이스 확장 시 기존 CapCut 구현체 하위 호환성 필수
- 17-5: 기존 CC-BY-SA 크레딧 로직과 BGM 크레딧 병합
- DB migration: 005_bgms.sql (Epic 15의 004 이후)
```

---

# Epic 16: Scene Approval Workflow (최후 실행)

```
Epic 16 (Scene Approval Workflow)의 모든 스토리(16-1 ~ 16-5)를 순차적으로 구현해줘.

⚠️ 최고 리스크 에픽: 상태 머신 변경 + 파이프라인 오케스트레이터 통합

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-16 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 16 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 16-1: State Machine Extension & Scene Approval Domain Model
- 16-2: Scene Approval Store
- 16-3: Approval Service — Per-Scene Workflow Orchestration
- 16-4: Pipeline Orchestrator Integration
- 16-5: Scene Asset Mapping Dashboard

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 16-1: 상태 머신 변경 — --skip-approval 플래그로 기존 파이프라인 동작 보존 필수
- 16-4: checkpoint/resume(Epic 5), progress dashboard(Epic 12)와 호환성 검증 필수
- 모든 기존 테스트가 --skip-approval 경로로 통과해야 함
- DB migration: 006_scene_approvals.sql (Epic 17의 005 이후)
```

---

# Epic 18: n8n-Ready API Execution Layer

```
Epic 18 (n8n-Ready API Execution Layer)의 모든 스토리(18-1 ~ 18-7)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-18 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 18 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 18-1: Server Plugin Injection and Service Initialization
- 18-2: Job Lifecycle Management with DB Persistence
- 18-3: Image and TTS Generation Handler Execution Logic
- 18-4: Assembly Endpoint and Prompt Update Persistence
- 18-5: Webhook Event Extension
- 18-6: Pipeline Run Handler with Stage-Based Execution
- 18-7: Scene Dashboard Enhancement for n8n Polling

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 18-1: serve 커맨드에서 CLI의 run 커맨드와 동일한 플러그인 초기화 패턴 적용
- 18-2: 기존 in-memory jobManager와 DB 간 sync, 서버 재시작 시 stale job 처리
- 18-3: 동일 프로젝트 동시 생성 요청 시 409 CONFLICT 반환 필수
- 18-4: 프롬프트 저장은 workspace manager 경유, POST /assemble 신규 엔드포인트
- 18-5: webhook 페이로드는 n8n HTTP Request 노드에서 파싱 가능한 flat JSON
- 18-6: POST /run 기본=시나리오 생성+scenario_review 정지, mode:"full"로 전체 실행
- 18-7: reject → regenerate → approve 사이클 정합성 검증 필수
- DB migration 없음 (기존 Job/SceneApproval 테이블 활용)
```

---

# 병렬 실행 가이드 (Epic 19-22)

```
의존성 그래프:

  Epic 19 (Quick Wins) ─────────────────── 독립
  Epic 20 (Image Validation) ──→ Epic 21 (Approval)
  Epic 22 (FFmpeg Rendering) ────────────── 독립

병렬 가능 조합:
  ┌─ 세션 A: Epic 19 (4 stories)    ← 독립, 즉시 착수
  ├─ 세션 B: Epic 20 (5 stories)    ← 독립, 즉시 착수
  └─ 세션 C: Epic 22 (4 stories)    ← 독립, 즉시 착수

  → Epic 21 (4 stories)는 Epic 20 완료 후 착수

최대 동시 실행: 3 세션 (Epic 19 + 20 + 22)
총 소요 라운드: 2 라운드
  - 라운드 1: Epic 19 + Epic 20 + Epic 22 (병렬)
  - 라운드 2: Epic 21 (Epic 20 완료 대기)

에픽 내부 병렬:
  - Epic 19: 19-1(Chapters)은 19-2~4(Glossary)와 완전 독립 → 병렬 가능
  - Epic 20: 20-1 → 20-2 → 20-3 → 20-4 → 20-5 순차 필수
  - Epic 21: 21-1 독립 | 21-2 → 21-3, 21-4 (프리뷰 후 CLI/API 병렬)
  - Epic 22: 22-1 → 22-2, 22-3 (병렬) → 22-4
```

---

# Epic 19: YouTube Optimization Quick Wins

```
Epic 19 (YouTube Optimization Quick Wins)의 모든 스토리(19-1 ~ 19-4)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-19 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 19 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md` (EFR1, EFR2 섹션)
- Enhancement PRD: `_bmad-output/planning-artifacts/prd-enhancement.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 19-1: YouTube Chapters Generation from Scene Timings (EFR1)
- 19-2: Glossary Suggestion Domain Model & Storage
- 19-3: LLM-Based Glossary Term Extraction & Suggestion (EFR2)
- 19-4: Glossary Suggestion Approval & Integration

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 19-1: 기존 service/timing.go에 GenerateChapters() 추가 (~30줄). 독립 기능이므로 19-2~4와 무관
- 19-2: DB migration 014_glossary_suggestions.sql (Epic 18까지 013 사용)
- 19-3: LLM 응답 JSON 파싱 실패 시 부분 데이터 저장 금지
- 19-4: glossary.json 파일 쓰기 시 기존 엔트리 보존 필수. Glossary.AddEntry() 메서드 사용
- Phase 1 MVP 추가 — 기존 파이프라인에 영향 없는 독립 기능
```

---

# Epic 20: AI Image Quality Validation

```
Epic 20 (AI Image Quality Validation)의 모든 스토리(20-1 ~ 20-5)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-20 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 20 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md` (EFR3, LLM Vision Extension 섹션)
- Enhancement PRD: `_bmad-output/planning-artifacts/prd-enhancement.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 20-1: LLM Vision Interface Extension
- 20-2: Image Validation Domain Model & Storage
- 20-3: Image Validator Service Core (EFR3)
- 20-4: Validation-Regeneration Loop
- 20-5: Image Generation Pipeline Integration & Config

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- 20-1: LLM 인터페이스에 CompleteWithVision() 추가. 기존 Complete() 시그니처 변경 금지. ErrNotSupported 패턴 따름 (imagegen.Edit() 참조). mockery 재생성 필수 (make generate)
- 20-2: DB migration 015_validation_score.sql. 테이블명은 코드베이스의 실제 shot 관련 테이블 확인 후 결정 (shot_manifests 또는 scene_manifests)
- 20-3: 평가 프롬프트는 JSON 응답 강제. 캐릭터 없는 씬은 CharacterMatch=-1, 가중 평균에서 제외
- 20-4: regenerateFn 콜백 패턴으로 ImageValidator↔ImageGen 순환 의존성 회피. 최대 3회 재생성 후 best-scoring 이미지 유지
- 20-5: image_validation.enabled=false 기본값 — 기존 파이프라인 동작 변경 없음
- Vision 미지원 프로바이더에서 ErrNotSupported 반환 시 검증 스킵 + 경고 로그
```

---

# Epic 21: Automated Approval & Batch Review

```
Epic 21 (Automated Approval & Batch Review)의 모든 스토리(21-1 ~ 21-4)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-21 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 21 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md` (EFR4, EFR5 섹션)
- Enhancement PRD: `_bmad-output/planning-artifacts/prd-enhancement.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 21-1: Auto-Approve by Validation Score (EFR4)
- 21-2: Batch Preview Data Assembly
- 21-3: Batch Approve with Selective Flagging — CLI (EFR5)
- 21-4: Batch Preview & Approve API Endpoints

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- Epic 20 완료 전제 (EFR3 검증 점수 필요)
- 21-1: 기존 service/approval.go에 AutoApproveByScore() 추가. auto_approval.enabled=true + image_validation.enabled=false 시 경고 로그 + 비활성화
- 21-1: validation_score==NULL인 씬은 리뷰 큐 유지 (자동 승인 불가)
- 21-2: EFR3 미활성 시에도 배치 프리뷰 정상 동작 (ValidationScore=nil)
- 21-3: CLI `yt-pipe review batch <scp-id> --asset image` 구현
- 21-4: API 엔드포인트 GET /api/v1/projects/{id}/preview, POST /api/v1/projects/{id}/batch-approve. n8n 호환 flat JSON (NFR11)
- DB migration 없음 (Epic 20에서 추가된 validation_score 컬럼 활용)
```

---

# Epic 22: FFmpeg Direct Video Rendering

```
Epic 22 (FFmpeg Direct Video Rendering)의 모든 스토리(22-1 ~ 22-4)를 순차적으로 구현해줘.

각 스토리마다 아래 사이클을 반복:
1. 스토리 생성: /bmad-bmm-create-story 실행 → 구현 명세 파일 생성
2. 스토리 개발: /bmad-bmm-dev-story 실행 → 코드 구현 + 테스트
3. 코드 리뷰: /bmad-bmm-code-review 실행 → 이슈 식별
4. 리뷰 반영: 지적 사항 수정 → make test && make lint 통과 확인
5. 완료 처리: sprint-status.yaml에서 해당 스토리를 done으로 업데이트

모든 스토리 완료 후:
6. 에픽 회고: /bmad-bmm-retrospective 실행 → epic-22 done, 회고 파일 생성

## 참조 문서
- 에픽/스토리 상세: `_bmad-output/planning-artifacts/epics.md` (Epic 22 섹션)
- 아키텍처: `_bmad-output/planning-artifacts/architecture.md` (EFR6, Docker Base Image Change 섹션)
- Enhancement PRD: `_bmad-output/planning-artifacts/prd-enhancement.md`
- 스프린트 상태: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## 스토리 목록 (순서대로)
- 22-1: Docker Base Image Migration & FFmpeg Availability Check (ENFR3)
- 22-2: FFmpeg Concat & Subtitle File Generation
- 22-3: BGM Mixing Filter Generation
- 22-4: FFmpegAssembler Integration, Registry & Output Selection (ENFR1)

## 구현 명세 파일명 패턴
`_bmad-output/implementation-artifacts/{story-id}-{kebab-case-title}.md`

## 제약사항
- Epic 20과 병렬 착수 가능 (독립)
- 22-1: Dockerfile 변경 — scratch → alpine:3.21 + apk add ffmpeg ca-certificates tzdata. 비root 유저 appuser(UID 65534). checkFFmpegAvailable() 구현
- 22-2: FFmpeg concat demuxer 포맷 (images.txt), concat protocol 포맷 (audio_concat.txt), SRT 표준 포맷. 빈 씬 리스트 시 에러 반환
- 22-3: BGM 볼륨(config), fade-in/out(기본 2s), 덕킹(기본 -12dB). BGM 없으면 나레이션만 통과
- 22-4: output.Assembler 인터페이스 구현. output.provider "capcut"|"ffmpeg"|"both" 설정. "both" 모드 시 service/assembler.go의 Assemble() 수정 (~10줄). 플러그인 레지스트리 등록. FFmpegConfig struct: Preset, CRF, AudioBitrate, Resolution, FPS, SubtitleFontSize
- 통합 테스트는 빌드 태그 ffmpegtest로 분리 (CI에서 FFmpeg 미설치 환경 대응)
- ENFR1: 10씬 MP4 출력 3분 이내 (1080p, 2 vCPU, 4GB RAM)
- DB migration 없음
```
