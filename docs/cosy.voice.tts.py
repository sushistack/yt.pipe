# coding=utf-8

import dashscope
from dashscope.audio.tts_v2 import *

# If the API Key is not configured in the environment variable, your-api-key needs to be replaced with your own API Key
# dashscope.api_key = "your-api-key"

model = "cosyvoice-v3-plus"
voice = "longanyang"

synthesizer = SpeechSynthesizer(model=model, voice=voice)
audio = synthesizer.call("今天天气怎么样？")

with open('output.mp3', 'wb') as f:
    f.write(audio)