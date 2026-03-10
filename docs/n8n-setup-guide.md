# n8n Integration & Configuration Guide

## API Keys (3 platforms, separate keys required)

| Service | Platform | Env Variable | Issuer |
|---------|----------|-------------|--------|
| LLM (gemini-3-flash-preview) | Google AI Studio | `YTP_LLM_API_KEY` | https://aistudio.google.com |
| ImageGen (Qwen/Qwen-Image-Edit) | SiliconFlow | `YTP_IMAGEGEN_API_KEY` | https://cloud.siliconflow.cn |
| TTS (qwen3-tts) | DashScope (Alibaba) | `YTP_TTS_API_KEY` | https://dashscope.console.aliyun.com |
| API Server Auth | Self-generated | `YTP_API_KEY` | `openssl rand -hex 32` |

> **Note**: SiliconFlow and DashScope are separate platforms. Keys are NOT interchangeable even for Qwen-branded models.

## .env Setup

```bash
YTP_LLM_API_KEY=<google-ai-studio-key>
YTP_IMAGEGEN_API_KEY=<siliconflow-key>
YTP_TTS_API_KEY=<dashscope-key>
YTP_API_KEY=<self-generated-random-key>
```

## n8n Credential Setup (Header Auth)

1. n8n sidebar → **Credentials** → **Add Credential**
2. Select **Header Auth** type
3. Configure:
   - Name: `yt-pipe API`
   - Header Name: `X-API-Key`
   - Header Value: same value as `YTP_API_KEY` in `.env`
4. In HTTP Request nodes → **Authentication** → **Generic Credential Type** → **Header Auth** → select `yt-pipe API`
5. Reuse the same credential across all yt-pipe API call nodes

## config.yaml Key Settings

```yaml
llm:
  provider: "gemini"
  model: "gemini-3-flash-preview"
  max_tokens: 16384

imagegen:
  provider: "siliconflow"
  model: "Qwen/Qwen-Image-Edit"

tts:
  provider: "dashscope"
  model: "qwen3-tts"

output:
  provider: "capcut"
```
