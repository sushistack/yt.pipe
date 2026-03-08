package store

import (
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndListFeedback(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a project first
	p := &domain.Project{
		ID:            "proj-feedback-1",
		SCPID:         "SCP-001",
		Status:        domain.StatusPending,
		WorkspacePath: "/tmp/test",
	}
	require.NoError(t, db.CreateProject(p))

	// Create feedback
	f1 := &domain.Feedback{
		ProjectID: p.ID,
		SceneNum:  1,
		AssetType: "image",
		Rating:    "good",
		Comment:   "nice image",
	}
	require.NoError(t, db.CreateFeedback(f1))
	assert.NotZero(t, f1.ID)
	assert.False(t, f1.CreatedAt.IsZero())

	f2 := &domain.Feedback{
		ProjectID: p.ID,
		SceneNum:  2,
		AssetType: "audio",
		Rating:    "bad",
	}
	require.NoError(t, db.CreateFeedback(f2))

	// List by project
	feedbacks, err := db.ListFeedbackByProject(p.ID)
	require.NoError(t, err)
	assert.Len(t, feedbacks, 2)
	// Verify both are present (order may vary when created_at is the same)
	types := map[string]bool{feedbacks[0].AssetType: true, feedbacks[1].AssetType: true}
	assert.True(t, types["image"])
	assert.True(t, types["audio"])
	// Find the image feedback and check comment
	for _, fb := range feedbacks {
		if fb.AssetType == "image" {
			assert.Equal(t, "nice image", fb.Comment)
		}
	}
}

func TestListFeedbackByProject_Empty(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	feedbacks, err := db.ListFeedbackByProject("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, feedbacks)
}

func TestListAllFeedback(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	p1 := &domain.Project{ID: "proj-fb-1", SCPID: "SCP-001", Status: domain.StatusPending, WorkspacePath: "/tmp/1"}
	p2 := &domain.Project{ID: "proj-fb-2", SCPID: "SCP-002", Status: domain.StatusPending, WorkspacePath: "/tmp/2"}
	require.NoError(t, db.CreateProject(p1))
	require.NoError(t, db.CreateProject(p2))

	require.NoError(t, db.CreateFeedback(&domain.Feedback{ProjectID: p1.ID, SceneNum: 1, AssetType: "image", Rating: "good"}))
	require.NoError(t, db.CreateFeedback(&domain.Feedback{ProjectID: p2.ID, SceneNum: 1, AssetType: "audio", Rating: "bad"}))

	all, err := db.ListAllFeedback()
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestCreateFeedback_NullComment(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	p := &domain.Project{ID: "proj-fb-null", SCPID: "SCP-003", Status: domain.StatusPending, WorkspacePath: "/tmp/3"}
	require.NoError(t, db.CreateProject(p))

	f := &domain.Feedback{
		ProjectID: p.ID,
		SceneNum:  1,
		AssetType: "subtitle",
		Rating:    "neutral",
		// Comment intentionally empty
	}
	require.NoError(t, db.CreateFeedback(f))

	feedbacks, err := db.ListFeedbackByProject(p.ID)
	require.NoError(t, err)
	assert.Len(t, feedbacks, 1)
	assert.Empty(t, feedbacks[0].Comment)
}
