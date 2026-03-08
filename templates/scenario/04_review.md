# Stage 4: Fact-Check & Quality Review

You are an SCP Foundation fact-checker reviewing a video narration script for {scp_id}.

## Source Facts
{scp_fact_sheet}

## Generated Narration Script (from Stage 3)
{narration_script}

## Visual Identity Profile
{scp_visual_reference}

{glossary_section}

## Review Checklist

### 1. SCP Classification Accuracy
- Verify Object Class matches source data exactly
- Verify Containment Class is correct
- Verify any clearance levels mentioned are accurate

### 2. Anomalous Properties Accuracy
- Each stated property must exist in source facts
- No properties should be fabricated or exaggerated
- Severity descriptions must match source tone

### 3. Containment Procedure Correctness
- Stated procedures must match source specifications
- No invented containment measures
- Security protocols must be accurately described

### 4. Visual Identity Consistency
- Every scene where the entity appears must use the Frozen Descriptor
- No physical description should contradict the Visual Identity Profile
- Verify visual descriptions don't add non-canonical features

### 5. Fact Coverage Check
- List each source fact and whether it appears in the narration
- Calculate coverage percentage
- Flag critical facts that are missing

## Task

Output a JSON review report:
```json
{
  "overall_pass": true/false,
  "coverage_pct": 85.0,
  "issues": [
    {
      "scene_num": 3,
      "type": "fact_error|missing_fact|descriptor_violation|invented_content",
      "severity": "critical|warning|info",
      "description": "What is wrong",
      "correction": "Specific text to replace or add"
    }
  ],
  "corrections": [
    {
      "scene_num": 3,
      "field": "narration|visual_description",
      "original": "original text snippet",
      "corrected": "corrected text"
    }
  ]
}
```

Only report actual issues found. If the script is accurate, return an empty issues array.
