# youtube.pipeline

SCP 콘텐츠 기반 YouTube 영상 자동 생성 파이프라인.
LLM 시나리오 생성 → 이미지 생성 → TTS 나레이션 → CapCut 프로젝트 어셈블리를 자동화합니다.

## Quick Start

```bash
# 빌드
make build

# 설정 파일 복사 후 수정
cp config.example.yaml config.yaml

# 서버 실행
./yt-pipe serve --config config.yaml

# 또는 go run으로 직접 실행
go run ./cmd/yt-pipe serve --config config.yaml
```

서버는 기본적으로 `localhost:8080`에서 실행됩니다.

## API Endpoints

모든 응답은 공통 JSON envelope을 사용합니다:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "timestamp": "2026-03-08T12:00:00Z",
  "request_id": "uuid"
}
```

인증이 활성화된 경우 `Authorization: Bearer <api-key>` 헤더가 필요합니다.
`/health`와 `/ready`는 인증 없이 접근 가능합니다.

---

### Health & Readiness

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | 서버 상태 확인. `{"status":"ok","version":"dev"}` 반환 |
| `GET` | `/ready` | DB 연결 및 워크스페이스 디렉토리 확인. 모두 정상이면 `{"status":"ready"}` |

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

---

### Projects (프로젝트 CRUD)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/projects` | 새 프로젝트 생성 |
| `GET` | `/api/v1/projects` | 프로젝트 목록 조회 (필터링/페이징 지원) |
| `GET` | `/api/v1/projects/{id}` | 단일 프로젝트 상세 조회 |
| `DELETE` | `/api/v1/projects/{id}` | 프로젝트 삭제 (`pending`/`complete` 상태만 가능) |

```bash
# 프로젝트 생성
curl -X POST http://localhost:8080/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{"scp_id":"SCP-173","title":"The Sculpture"}'

# 프로젝트 목록 (필터링)
curl "http://localhost:8080/api/v1/projects?state=pending&limit=10&offset=0"

# 단일 프로젝트 조회
curl http://localhost:8080/api/v1/projects/{id}

# 프로젝트 삭제
curl -X DELETE http://localhost:8080/api/v1/projects/{id}
```

---

### Pipeline Control (파이프라인 제어)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/projects/{id}/run` | 파이프라인 실행 시작 (202 Accepted, 비동기) |
| `GET` | `/api/v1/projects/{id}/status` | 파이프라인 실행 상태 조회 (진행률 포함) |
| `POST` | `/api/v1/projects/{id}/cancel` | 실행 중인 파이프라인 취소 |
| `POST` | `/api/v1/projects/{id}/approve` | 시나리오 리뷰 승인 (`scenario_review` → `approved` 전환) |

```bash
# 파이프라인 시작 (특정 스테이지부터)
curl -X POST http://localhost:8080/api/v1/projects/{id}/run \
  -H 'Content-Type: application/json' \
  -d '{"from_stage":"scenario","dry_run":false}'

# 상태 확인
curl http://localhost:8080/api/v1/projects/{id}/status

# 취소
curl -X POST http://localhost:8080/api/v1/projects/{id}/cancel

# 시나리오 승인
curl -X POST http://localhost:8080/api/v1/projects/{id}/approve
```

---

### Asset Management (에셋 관리)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/projects/{id}/images/generate` | 이미지 생성 요청 (특정 씬 지정 가능, 202 Accepted) |
| `POST` | `/api/v1/projects/{id}/tts/generate` | TTS 나레이션 생성 요청 (특정 씬 지정 가능, 202 Accepted) |
| `PUT` | `/api/v1/projects/{id}/scenes/{num}/prompt` | 특정 씬의 이미지 프롬프트 수정 |
| `POST` | `/api/v1/projects/{id}/feedback` | 에셋 품질 피드백 등록 |

```bash
# 특정 씬의 이미지 생성
curl -X POST http://localhost:8080/api/v1/projects/{id}/images/generate \
  -H 'Content-Type: application/json' \
  -d '{"scene_numbers":[1,3,5]}'

# TTS 생성
curl -X POST http://localhost:8080/api/v1/projects/{id}/tts/generate \
  -H 'Content-Type: application/json' \
  -d '{"scene_numbers":[1,2,3]}'

# 프롬프트 수정
curl -X PUT http://localhost:8080/api/v1/projects/{id}/scenes/3/prompt \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"A dark corridor with SCP-173 statue..."}'

# 피드백 등록
curl -X POST http://localhost:8080/api/v1/projects/{id}/feedback \
  -H 'Content-Type: application/json' \
  -d '{"asset_type":"image","scene_number":1,"rating":4,"comment":"Good composition"}'
```

---

### Configuration & Plugins (설정 및 플러그인)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/config` | 현재 설정 조회 (API 키 마스킹됨) |
| `PATCH` | `/api/v1/config` | 런타임 설정 변경 (허용된 필드만) |
| `GET` | `/api/v1/plugins` | 등록된 플러그인 목록 조회 |
| `PUT` | `/api/v1/plugins/{type}/active` | 활성 플러그인 변경 (llm/imagegen/tts/output) |

**PATCH 허용 필드:** `log_level`, `log_format`, `llm.model`, `llm.temperature`, `llm.max_tokens`, `tts.voice`, `tts.speed`

```bash
# 설정 조회
curl http://localhost:8080/api/v1/config

# 설정 변경
curl -X PATCH http://localhost:8080/api/v1/config \
  -H 'Content-Type: application/json' \
  -d '{"log_level":"debug","llm.temperature":0.5}'

# 플러그인 목록
curl http://localhost:8080/api/v1/plugins

# 활성 플러그인 변경
curl -X PUT http://localhost:8080/api/v1/plugins/tts/active \
  -H 'Content-Type: application/json' \
  -d '{"provider":"edge"}'
```

---

## Authentication

`config.yaml`에서 인증을 활성화할 수 있습니다:

```yaml
api:
  auth:
    enabled: true
    key: "your-secret-api-key"
```

또는 환경변수로 설정:

```bash
export YTP_API_AUTH_ENABLED=true
export YTP_API_AUTH_KEY="your-secret-api-key"
```

인증이 활성화되면 모든 `/api/v1/*` 요청에 헤더가 필요합니다:

```bash
curl -H "Authorization: Bearer your-secret-api-key" http://localhost:8080/api/v1/projects
```

## Webhooks

상태 변경 시 외부 시스템에 알림을 보낼 수 있습니다:

```yaml
webhooks:
  urls:
    - "https://your-webhook-endpoint.com/hook"
  timeout_seconds: 10
  retry_max_attempts: 3
```

웹훅 페이로드:

```json
{
  "event": "state_change",
  "project_id": "uuid",
  "scp_id": "SCP-173",
  "previous_state": "pending",
  "new_state": "scenario_review",
  "timestamp": "2026-03-08T12:00:00Z"
}
```

## Project Structure

```
cmd/yt-pipe/          CLI 엔트리포인트
internal/
  api/                REST API 서버 (Epic 7)
  cli/                Cobra CLI 명령어
  config/             설정 관리 (Viper)
  domain/             도메인 모델
  glossary/           SCP 용어 사전
  logging/            구조화된 로깅
  pipeline/           파이프라인 오케스트레이션
  plugin/             플러그인 인터페이스 (LLM, ImageGen, TTS, Output)
  retry/              재시도 로직
  service/            비즈니스 로직 서비스
  store/              SQLite 저장소
  template/           프롬프트 템플릿
  workspace/          프로젝트 워크스페이스 관리
templates/            프롬프트 템플릿 파일
```

## License

Private project.
