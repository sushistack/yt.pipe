-- Migration 009: Simplify project statuses to dependency-based stage model
-- Old: pending, scenario_review, approved, image_review, tts_review, generating_assets, assembling, complete
-- New: pending, scenario, images, tts, complete

-- Map old statuses to new stages
UPDATE projects SET status = 'scenario' WHERE status = 'scenario_review';
UPDATE projects SET status = 'scenario' WHERE status = 'approved';
UPDATE projects SET status = 'scenario' WHERE status = 'generating_assets';
UPDATE projects SET status = 'images' WHERE status = 'image_review';
UPDATE projects SET status = 'tts' WHERE status = 'tts_review';
UPDATE projects SET status = 'complete' WHERE status = 'assembling';
-- pending and complete remain unchanged
