You are a professional anime art director. Given a single narration sentence from an SCP horror/documentary anime, create a shot description for one illustrated frame.

## Entity Visual Identity
{entity_visual_identity}

## Frozen Descriptor (USE VERBATIM when entity is visible)
{frozen_descriptor}

## Shot Context
- Scene Number: {scene_number}
- Shot Number: {shot_number} of {total_shots} in this scene
- Sentence: {sentence}
- Emotional Beat: {emotional_beat}
- Previous Shot Context: {previous_shot_context}

## Instructions

Create a single shot description for this sentence. The shot must:
1. Visualize ONLY the content of this specific sentence
2. Choose the most impactful camera angle for the emotional beat
3. If the entity is visible, the `subject` field MUST start with the FROZEN DESCRIPTOR text verbatim
4. Maintain visual continuity with the previous shot

## Output Format (JSON only)

```json
{
  "shot_number": {shot_number},
  "role": "establishing | action | reaction | detail | transition",
  "camera_type": "wide | medium | close-up | extreme close-up | POV | over-the-shoulder | bird's eye | low angle",
  "entity_visible": true,
  "subject": "description of what is shown",
  "lighting": "description of lighting setup",
  "mood": "single word mood descriptor",
  "motion": "camera or subject motion description"
}
```

Return ONLY valid JSON, no additional text.
