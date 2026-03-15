# Stage 2: Scene Structure Design

You are a video director structuring a {target_duration}-minute documentary about {scp_id}.

## Research Packet (from Stage 1)
{research_packet}

## Visual Identity Profile (Frozen Descriptor)
{scp_visual_reference}

{glossary_section}

## Storytelling Format Guide

Apply the following storytelling principles when designing scene structure, emotional curve, and pacing.

{format_guide}

## Structure Requirements

Design the scene structure following the 4-act format:
- **Act 1 - Hook & Introduction** (~15% of total): Grab attention, establish SCP identity
- **Act 2 - Properties & Background** (~30%): Explain anomalous properties, containment
- **Act 3 - Incidents & Evidence** (~40%): Dramatic incidents, test logs, encounters
- **Act 4 - Resolution & Mystery** (~15%): Current status, unresolved questions, closing hook

## Task

For each scene (8-12 total), provide:

```json
{
  "scene_num": 1,
  "act": "hook",
  "synopsis": "Brief description of what happens in this scene",
  "key_points": ["fact or detail to convey", "visual element to show"],
  "emotional_beat": "tension/mystery/horror/revelation/etc",
  "estimated_duration_sec": 45,
  "fact_references": ["fact_key_1", "fact_key_2"]
}
```

### Rules:
1. Each scene's `key_points` must reference the Visual Identity Profile verbatim when the entity appears
2. Scenes must cover all Key Dramatic Beats from the research
3. Each fact from the source data should appear in at least one scene's `fact_references`
4. **Pacing variation is MANDATORY**: alternate between slower atmospheric scenes (60-90s) and faster incident scenes (30-45s). Never use the same duration for 3+ consecutive scenes.
5. **The first scene must hook within 5 seconds** — use one of the candidate hooks from the research packet
6. The last scene must leave an unresolved mystery
7. **Adjacent scenes MUST have different emotional beats** — never repeat the same mood consecutively (e.g., "tension, tension" is forbidden; "tension, mystery" is correct)
8. **Include at least one "viewer immersion" scene** where the narration addresses the viewer directly (2nd person)

Output as a JSON array of scene objects.
