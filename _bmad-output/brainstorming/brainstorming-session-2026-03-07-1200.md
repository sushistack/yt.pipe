---
stepsCompleted: [1, 2, 3, 4]
inputDocuments: []
session_topic: '기존 video.pipeline을 참고한 완전히 새로운 체계적인 영상 생성 파이프라인 구상'
session_goals: '새 파이프라인에 포함할 기능과 특징에 대한 포괄적인 아이디어 목록 도출'
selected_approach: 'ai-recommended'
techniques_used: ['SCAMPER Method', 'Cross-Pollination', 'Morphological Analysis']
ideas_generated: [38]
context_file: '$HOME/projects/video.pipeline'
technique_execution_complete: true
---

# Brainstorming Session Results

**Facilitator:** Jay
**Date:** 2026-03-07

## Session Overview

**Topic:** 기존 video.pipeline을 참고한 완전히 새로운 체계적인 영상 생성 파이프라인 구상
**Goals:** 새 파이프라인에 포함할 기능과 특징에 대한 포괄적인 아이디어 목록 도출
**Domain:** SCP Foundation 콘텐츠 전문 영상 생성 파이프라인
**Data Source:** 422개 SCP 크롤링 데이터 (/mnt/data/raw/) - main.txt, meta.json, facts.json

### Context Guidance

_기존 video.pipeline 프로젝트 분석 결과:_
- Reflex 기반 웹 UI로 영상 제작 워크플로우 관리
- 핵심 파이프라인: STT 자막 추출 -> 자막 편집/번역 -> 시나리오 생성 -> TTS 음성 합성 -> CapCut 프로젝트 생성
- 이미지 파이프라인: Image Prompter (shot breakdown + Frozen Descriptor) -> Image Generator (Janus-Pro-7B via SiliconFlow)
- TTS: Qwen3-TTS, 다국어(ko/en/ja), 프리셋 스피커, 속도 조절
- Scene Detector: PySceneDetect 기반 컷 감지 + 키프레임 추출
- 외부 도구: GPT-SoVITS (TTS), Gemini API (프롬프트), CapCut 템플릿

### Session Setup

- **접근 방식:** AI 추천 기법
- **스킬 레벨:** Intermediate

## Technique Selection

**Approach:** AI-Recommended Techniques
**Analysis Context:** 새로운 영상 생성 파이프라인 with focus on 기능 아이디어 목록

**Recommended Techniques:**

- **SCAMPER Method:** 기존 video.pipeline의 각 기능을 7가지 렌즈로 체계적 분석
- **Cross-Pollination:** 타 도메인(게임, 음악, 영화, 출판, DevOps)의 솔루션 이식
- **Morphological Analysis:** 아이디어를 핵심 파라미터별로 분류/조합

---

## Technique Execution Results

### SCAMPER Method

**S - Substitute (대체)**
- 파일 업로드 -> SCP 구조화 데이터 기반 입력으로 대체
- CapCut 의존 -> 하이브리드 출력(FFmpeg 80% + 수동 20%)
- 수동 팩트체크 -> AI 도메인 특화 팩트 검증

**C - Combine (결합)**
- SCP 의존성 그래프로 시리즈 자동 기획
- 레이어 분리 오디오 아키텍처 (나레이션/BGM/효과음 독립 관리)

**A - Adapt (적응)**
- 인기 SCP 유튜버 영상 구조를 템플릿으로 표준화
- 뉴스룸의 데스크 검토 패턴 -> 자동 품질 게이트
- 게임 모딩의 모듈화 -> 플러그인 아키텍처

**M - Modify (변형)**
- 배치 프로세싱 (다건 동시 처리)
- 멀티 길이 시나리오 (숏/미드/롱폼)
- 비주얼 스타일 프리셋 + 비주얼 ID카드로 continuation 해결

**P - Put to Other Uses (다른 용도)**
- 오디오 전용 모드 (팟캐스트/오디오북)

**E - Eliminate (제거)**
- SCP ID 기반 자동 프로젝트 초기화 (수동 선택 제거)
- 원클릭 자동 체이닝 (단계 간 수동 전환 제거)
- 글로벌 설정 프로필 (중복 설정 제거)

**R - Reverse (역전)**
- 수요 기반 콘텐츠 추천 (제작자 -> 시청자 관점 역전)

### Cross-Pollination

**게임 산업:** 증분 빌드 (CI/CD), 점진적 에셋 라이브러리
**음악 프로덕션:** 타임라인 에디터 (DAW), 자동 포스트프로세싱 (마스터링)
**영화:** 스토리보드 자동 생성, 컬러 스크립트
**출판/저널리즘:** 편집 앵글 시스템
**DevOps:** 파이프라인 대시보드, 버전 관리 & 롤백, 데이터 리니지
**이커머스:** A/B 테스트 썸네일, 콘텐츠 퍼널 전략
**교육:** 학습 경로 (시리즈 커리큘럼)
**ML Ops:** 모델 레지스트리, 프롬프트 라이브러리 & 버저닝
**크리에이터 이코노미:** 자동 썸네일, SEO 자동 최적화, API 비용 추적

---

## Project Architecture Vision

### 핵심 설계 원칙

youtube.pipeline은 **"코어 + 모듈" 생태계**로 설계한다:

```
youtube.pipeline (Core)
  |-- 콘텐츠 생성 파이프라인 (이 프로젝트의 핵심)
  |-- 표준 메타데이터/상태 API 노출
  |
  +-- [Future Modules]
       |-- content-calendar: 콘텐츠 캘린더 & 스케줄 관리
       |-- analytics-dashboard: 크리에이터 분석 & 성과 추적
       |-- learning-engine: AI 학습 피드백 루프
       +-- ...
```

- **코어 파이프라인의 책임:** SCP 데이터 -> 완성 영상 에셋 생성에 집중
- **코어가 제공하는 것:** 표준 메타데이터 출력, 프로젝트 상태 API, 에셋 내보내기
- **코어가 하지 않는 것:** 채널 운영, 성과 분석, 업로드 스케줄링
- **모듈 연동:** 코어의 API를 통해 운영 모듈이 데이터를 가져다 씀

---

## Organized Idea Catalog

### Tier 1: Core Pipeline (MVP)

| # | 아이디어 | 카테고리 | 설명 |
|---|---------|---------|------|
| 1 | SCP 데이터 기반 입력 | Input | SCP ID 선택 -> facts.json/meta.json/main.txt 자동 로딩 |
| 7 | 영상 구조 템플릿 | Scenario | 검증된 SCP 영상 구조(인트로->격리->설명->사건->결론) 표준화 |
| 9 | 모듈식 플러그인 아키텍처 | Architecture | TTS/이미지/LLM 엔진을 설정으로 교체 가능한 플러그인 구조 |
| 12 | 비주얼 스타일 프리셋 | Visual | 사실적/만화/공포/다큐 프리셋 + SCP별 비주얼 ID카드 |
| 14 | 자동 프로젝트 초기화 | UX | SCP ID 입력만으로 프로젝트 폴더 + 데이터 자동 세팅 |
| 15 | 원클릭 자동 체이닝 | UX | 시나리오->프롬프트->이미지->TTS 자동 순차 실행 |
| 16 | 글로벌 설정 프로필 | Config | TTS/이미지/스타일 글로벌 설정, 프로젝트별 오버라이드 |
| 17 | 수요 기반 콘텐츠 추천 | Strategy | 유튜브 트렌드/검색량 기반 SCP 추천 엔진 |
| 18 | 증분 빌드 | Architecture | 변경된 씬만 재생성 (전체 재빌드 방지) |
| 19 | 점진적 에셋 라이브러리 | Visual | AI 생성 -> 사용자 선택 -> 에피소드마다 축적 |
| 23 | 스토리보드 프리뷰 | Visual | 저해상도 러프 스토리보드 먼저 -> 승인 후 고화질 |
| 25 | 파이프라인 대시보드 | UX | 실시간 진행률, 에러 상태, 배치 모니터링 |
| 33 | 자동 썸네일 생성 | Publishing | visual_elements + 제목 기반 유튜브 최적화 썸네일 |
| 34 | SEO 자동 최적화 | Publishing | 제목/설명/태그/챕터 마커 자동 생성 |
| 8 | 자동 품질 게이트 | Quality | 단계별 팩트 커버리지, 프롬프트 일치도 자동 검증 |

### Tier 2: High Value (Post-MVP)

| # | 아이디어 | 카테고리 | 설명 |
|---|---------|---------|------|
| 4 | 팩트 검증 시스템 | Quality | facts.json 기준 시나리오/스크립트 정확도 검증 |
| 5 | SCP 의존성 그래프 | Strategy | 관련 SCP 순회로 시리즈/테마 자동 추천 |
| 10 | 배치 프로세싱 | Architecture | 다건 SCP 큐 + 병렬/순차 자동 처리 |
| 11 | 멀티 길이 시나리오 | Scenario | 숏폼(60초)/미드폼(5분)/롱폼(15분) 자동 생성 |
| 20 | 타임라인 에디터 | UX | DAW식 멀티트랙 타임라인 (나레이션/이미지/자막/BGM) |
| 22 | 편집 앵글 시스템 | Scenario | 공포/과학/인터뷰/역사 등 다른 앵글로 시나리오 생성 |
| 26 | 버전 관리 & 롤백 | Architecture | 각 단계 산출물 버전 관리, 이전 버전 복원 |
| 27 | A/B 테스트 썸네일 | Publishing | 같은 SCP에 2-3개 썸네일 변형 자동 생성 |
| 30 | 데이터 리니지 추적 | Architecture | 최종 에셋의 출처/변환 과정 시각화 |
| 31 | 품질 점수 정량화 | Quality | 팩트 커버리지 %, 프롬프트 일치도 % 등 수치화 |
| 36 | 프롬프트 라이브러리 | Quality | 검증된 프롬프트 버저닝 & 재사용 |
| 3 | 하이브리드 출력 | Output | FFmpeg 자동 조합 + 필요시 편집기 내보내기 |

### Tier 3: Nice to Have

| # | 아이디어 | 카테고리 | 설명 |
|---|---------|---------|------|
| 6 | 레이어 분리 오디오 | Audio | 나레이션/BGM/효과음 독립 트랙 관리 |
| 13 | 오디오 전용 모드 | Output | TTS + BGM만으로 팟캐스트/오디오북 출력 |
| 21 | 자동 포스트프로세싱 | Quality | 오디오 노멀라이징, 색감 통일, 타이밍 조정 |
| 24 | 컬러 스크립트 모드 | Visual | 감정 곡선 -> 색상 팔레트 자동 매핑 |
| 28 | 콘텐츠 퍼널 전략 | Strategy | 숏폼 유입 -> 미드폼 관심 -> 롱폼 팬 전환 |
| 29 | 시리즈 커리큘럼 | Strategy | 의존성 기반 시청 순서/학습 경로 |
| 35 | 모델 레지스트리 | Architecture | AI 모델 버전 + 설정 경량 추적 |
| 38 | API 비용 추적 | Architecture | API 호출당 비용 로깅 |

### Future Modules (별도 프로젝트)

| 모듈 | 용도 | 코어 연동 |
|------|------|----------|
| **content-calendar** | 업로드 스케줄, 콘텐츠 캘린더, 제작 진행률 통합 뷰 | 코어의 프로젝트 상태 API 소비 |
| **analytics-dashboard** | 유튜브 성과 분석, 조회수/유지율 추적, 스타일별 성과 비교 | 코어의 메타데이터 내보내기 소비 |
| **learning-engine** | 성과 데이터 -> 제작 파라미터 최적화 피드백 루프 | analytics 데이터 -> 코어 설정 추천 |

---

## Key Decisions & Insights

1. **SCP 전문화:** 범용이 아닌 SCP 특화 파이프라인이 더 강력하다
2. **생성에 집중:** 코어는 콘텐츠 생성에만 집중, 운영/분석은 별도 모듈
3. **"제안은 AI, 결정은 사람":** 자동화하되 최종 제어권은 사용자에게
4. **점진적 축적:** 에셋/프롬프트를 에피소드마다 축적하는 성장형 시스템
5. **증분 빌드:** 전체 재생성 대신 변경분만 처리하는 효율적 파이프라인
6. **채널 하나에 집중:** 다국어 확장은 나중에, 지금은 ko 채널 성장에 집중
7. **수요 기반:** 제작자가 아닌 시청자 관점에서 콘텐츠 선택

## Creative Facilitation Narrative

Jay님은 기존 video.pipeline에 대한 깊은 이해와 SCP 데이터 준비(422개 크롤링, facts.json의 구조화된 팩트)라는 강력한 기반을 갖고 있었습니다. 세션을 통해 "범용 영상 도구"에서 "SCP 전문 콘텐츠 공장"으로 비전이 구체화되었고, 특히 플러그인 아키텍처 + 증분 빌드 + 품질 게이트라는 엔지니어링 원칙이 자연스럽게 도출되었습니다. 콘텐츠 전략(수요 기반 추천, A/B 테스트)과 제작 효율(원클릭 체이닝, 배치 모드)의 균형이 잘 잡힌 비전입니다.
