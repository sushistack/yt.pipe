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
