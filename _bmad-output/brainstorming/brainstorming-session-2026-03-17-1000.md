---
stepsCompleted: [1, 2, 3, 4]
inputDocuments: []
session_topic: 'yt.pipe pipeline - additional automation opportunities & video quality improvement'
session_goals: 'Immediately implementable feature ideas + mid/long-term roadmap directions'
selected_approach: 'ai-recommended'
techniques_used: ['SCAMPER Method', 'Morphological Analysis', 'Cross-Pollination']
ideas_generated: [44]
context_file: ''
session_active: false
workflow_completed: true
---

# Brainstorming Session Results

**Facilitator:** Jay
**Date:** 2026-03-17

## Session Overview

**Topic:** yt.pipe 파이프라인 추가 자동화 및 영상 퀄리티 향상
**Goals:** 즉시 구현 가능한 기능 아이디어 + 중장기 로드맵 방향성

### Session Setup

- 포괄적 + 심층적 범위로 자동화 갭과 품질 향상 모두 탐색
- 현재 8단계 파이프라인 (시나리오→이미지→TTS→자막→CapCut) 완성 상태 기반

## Technique Selection

**Approach:** AI-Recommended Techniques
**Techniques:** SCAMPER → Morphological Analysis → Cross-Pollination

## Technique Execution Results

### SCAMPER Method (30 ideas)

**Substitute:** ImageGen fallback chain, TTS A/B 비교, 씬별 LLM 라우팅, FFmpeg 직접 렌더, Remotion, CapCut 대체
**Combine:** 시나리오+이미지 동시생성, TTS+BGM 비트 동기화, 이미지 생성+품질평가 루프, 자막-이미지 문장동기화, 썸네일+메타 자동생성
**Adapt:** CI/CD 패턴, A/B 테스트, LOD 점진적 렌더링, 넷플릭스 추천 패턴
**Modify:** 캐릭터 비전 검증, 동적 자막 스타일링, 용어사전 자동확장, 이미지 트랜지션
**Eliminate:** 시나리오 리뷰 조건부 스킵, CapCut 의존성 제거, 전체 프리뷰 1회 승인
**Reverse:** 역방향 파이프라인(이미지→시나리오), 레퍼런스 기반 설정 역산

### Morphological Analysis (5 ideas)

5축(입력소스 × 영상생성 × 품질보장 × 오디오레이어 × 퍼블리싱) 교차 조합
핵심 발견: SFX 레이어 부재, 듀얼 프로파일(Quick/Premium) 필요성

### Cross-Pollination (9 ideas)

게임(에셋 레지스트리, 레이어드 오디오), 오디오북(문장별 감정 TTS), 팟캐스트(YouTube Chapters),
영화(컬러 그레이딩, 오디오 크로스페이드), 뉴스(콘텐츠 캘린더, 위키 감지), SaaS(스타일 프리셋)

## Idea Organization and Prioritization

### Theme 1: 영상 출력 혁신 (CapCut → Programmatic)
- #4 FFmpeg 직접 영상 렌더링
- #6 Remotion 프로그래밍 기반 영상
- #27 CapCut 의존성 제거

### Theme 2: AI 자동 품질 보장
- #8 AI 품질 점수 기반 선택적 리뷰
- #15 이미지 생성+품질 평가 자동 루프 (Qwen-VL)
- #22 캐릭터 일관성 비전 자동 검증
- #26 시나리오 리뷰 조건부 스킵

### Theme 3: 인간 병목 최소화
- #7 승인 시점 전방 이동 (30초 트레일러)
- #28 전체 프리뷰 1회 승인 (예외 보고식)

### Theme 4: 오디오 레이어 고도화
- #12 BGM 무드 프리셋 풀
- #33 씬 무드 기반 SFX 자동 삽입
- #41 씬 전환 오디오 크로스페이드

### Theme 5: 비주얼 품질 향상
- #9 Qwen 이미지 프로바이더 추가
- #11 핵심 이미지 i2v (Wan Video first frame)
- #23 씬 무드 기반 동적 자막 스타일링
- #25 AI 이미지 트랜지션/보간 프레임
- #40 씬 무드 기반 컬러 그레이딩

### Theme 6: 파이프라인 확장
- #16 자막-이미지 문장 단위 동기화
- #17 썸네일+제목+설명 자동 생성
- #24 용어 사전 자동 확장
- #29 역방향 파이프라인 (이미지→시나리오)
- #31 이미지 드리븐 파이프라인 (Google 검색 기반)
- #39 YouTube Chapters 자동 생성

### Theme 7: 운영/인프라
- #18 CI/CD 패턴 영상 파이프라인
- #35 듀얼 프로파일 (Quick/Premium)
- #36 에셋 레지스트리 (크로스 프로젝트)
- #42 콘텐츠 캘린더 + 스케줄링
- #44 스타일 프리셋 시스템

### Prioritization Results

**Tier 1 — Quick Wins (빠르게 적용 + 높은 임팩트)**
- #39 YouTube Chapters 자동 생성 (씬 타이밍 데이터 이미 존재)
- #28 전체 프리뷰 1회 승인 (UX 변경만)
- #24 용어 사전 자동 확장
- #41 오디오 크로스페이드 (FFmpeg 기본 기능)
- #12 BGM 무드 프리셋 풀 (데이터 구성)
- #40 컬러 그레이딩 (FFmpeg 필터)

**Tier 2 — Core Features (중간 난이도 + 핵심 가치)**
- #4 FFmpeg 직접 렌더링
- #15+22 이미지 품질 자동 검증 (Qwen-VL)
- #8+26 AI 기반 선택적 리뷰/자동 승인
- #33 SFX 자동 삽입
- #23 동적 자막 스타일링
- #16 문장 단위 이미지-자막 동기화
- #17 썸네일+메타 자동 생성
- #44 스타일 프리셋 시스템
- #9 Qwen 이미지 프로바이더

**Tier 3 — Game Changers (높은 난이도 + 차별화)**
- #6+27 Remotion/FFmpeg CapCut 완전 대체
- #11 핵심 씬 i2v (Wan Video)
- #25 AI 이미지 트랜지션
- #29+31 역방향/이미지 드리븐 파이프라인
- #7 승인 전방 이동 (트레일러)
- #18 CI/CD 패턴
- #35 듀얼 프로파일
- #36 크로스 프로젝트 에셋 레지스트리
- #42 콘텐츠 캘린더

### Deferred to idea.md
- #13 시나리오+이미지 동시생성 (현재 구조 파괴 리스크)
- #19 A/B 테스트 시나리오 변형
- #20 LOD 점진적 렌더링
- #21 시청자 선호 학습 (넷플릭스 패턴)
- #30 레퍼런스 기반 프로젝트 초기화
- #34 Zero-Touch 완전 무인 파이프라인
- #37 4레이어 오디오 믹싱 (SFX 라이브러리 필요)
- #38 문장 단위 감정 TTS (DashScope 제약)
- #43 SCP 위키 변경 감지

## Session Summary and Insights

**Key Achievements:**
- 44 ideas generated across 3 techniques (SCAMPER, Morphological Analysis, Cross-Pollination)
- 25 ideas confirmed for implementation, 9 deferred to idea.md
- 7 thematic clusters identified
- 3-tier priority framework established

**Creative Breakthroughs:**
1. **SFX 레이어 발견** — Morphological Analysis에서 오디오 축 탐색 중 현재 파이프라인에 SFX가 완전 부재함을 발견. SCP 콘텐츠에서 사운드 이펙트는 몰입감의 핵심.
2. **역방향 파이프라인** — SCAMPER Reverse에서 "이미지 먼저 → 시나리오 나중" 패러다임 발견. Google 이미지 검색 → 핵심 이미지 생성 → 시나리오 역방향 생성으로 구체화.
3. **인간 병목 재정의** — "승인을 없앤다"가 아니라 "승인 시점을 옮긴다"(전방 이동) + "승인 대상을 줄인다"(예외 보고식) 이중 전략.

**Session Reflections:**
- Jay의 핵심 통찰: "인간 병목 구간만 남았다"는 문제 정의가 세션 전체를 관통
- 기술적 판단이 날카로움: 각 아이디어에 대해 즉시 실현 가능성/비용/가치 평가
- idea.md 분류 기준: "좋은 아이디어지만 지금은 아닌 것"을 명확히 구분
