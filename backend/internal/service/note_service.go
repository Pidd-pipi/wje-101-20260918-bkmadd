package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

// NoteService handles tasting notes.
type NoteService struct {
	repo   *repository.TastingNoteRepository
	logger *slog.Logger
}

// NewNoteService creates a NoteService.
func NewNoteService(repo *repository.TastingNoteRepository, logger *slog.Logger) *NoteService {
	return &NoteService{repo: repo, logger: logger}
}

// Create adds a note for a user. Status may be draft or published
// (defaults to published when empty). Published notes require a coffee name
// and a valid roast level; drafts relax those requirements.
func (s *NoteService) Create(userID uint, n *model.TastingNote) (*model.TastingNote, error) {
	status := n.Status
	if status == "" {
		status = constants.NoteStatusPublished
	}
	if !constants.IsValidNoteStatus(status) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("TastingNote[status=%s] create failed: invalid status", n.Status))
	}
	if status == constants.NoteStatusPublished {
		if err := validateForPublish(n.CoffeeName, n.RoastLevel); err != nil {
			return nil, err
		}
	}
	if n.RoastLevel == "" {
		n.RoastLevel = constants.RoastLight
	} else if !constants.IsValidRoastLevel(n.RoastLevel) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("TastingNote[roast_level=%s] create failed: invalid roast level", n.RoastLevel))
	}
	n.UserID = userID
	n.Status = status
	if n.FlavorTags == "" {
		n.FlavorTags = "[]"
	}
	if err := s.repo.Create(n); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogNoteCreateFailed, n.CoffeeName), "error", err)
		return nil, fmt.Errorf("note create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteCreateSuccess, n.CoffeeName), "id", n.ID, "status", n.Status)
	return n, nil
}

// Get returns a published note by id. A draft is only returned to its author;
// everyone else (including unauthenticated visitors) gets a not-found error,
// so drafts behave as if they do not exist.
func (s *NoteService) Get(id, viewerID uint) (*model.TastingNote, error) {
	n, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note get: %w", err)
	}
	if n.Status == constants.NoteStatusDraft && n.UserID != viewerID {
		return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
	}
	return n, nil
}

// Update edits a note owned by the user. A note carrying status=publish is
// published (draft -> published is the only allowed transition); once
// published it can never go back to draft.
func (s *NoteService) Update(userID, id uint, n *model.TastingNote) (*model.TastingNote, error) {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note update find: %w", err)
	}
	if exist.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("TastingNote[id=%d] update failed: user_id=%d not owner", id, userID))
	}

	targetStatus := exist.Status
	if n.Status != "" {
		if !constants.IsValidNoteStatus(n.Status) {
			return nil, util.NewAppError(422, constants.CodeValidationError, "invalid note status")
		}
		if exist.Status == constants.NoteStatusPublished && n.Status == constants.NoteStatusDraft {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("TastingNote[id=%d] update failed: published note cannot revert to draft", id))
		}
		targetStatus = n.Status
	}

	coffeeName := n.CoffeeName
	if targetStatus == constants.NoteStatusPublished {
		if err := validateForPublish(coffeeName, n.RoastLevel); err != nil {
			return nil, err
		}
	}

	exist.CoffeeName = coffeeName
	exist.Origin = n.Origin
	exist.BrewMethod = n.BrewMethod
	exist.NotesText = n.NotesText
	exist.ImageURL = n.ImageURL
	exist.BrewRecipeID = n.BrewRecipeID
	exist.AromaScore = n.AromaScore
	exist.AcidityScore = n.AcidityScore
	exist.BodyScore = n.BodyScore
	exist.OverallScore = n.OverallScore
	if n.FlavorTags != "" {
		exist.FlavorTags = n.FlavorTags
	}
	if n.RoastLevel != "" {
		if !constants.IsValidRoastLevel(n.RoastLevel) {
			return nil, util.NewAppError(422, constants.CodeValidationError, "invalid roast level")
		}
		exist.RoastLevel = n.RoastLevel
	}
	exist.Status = targetStatus

	if err := s.repo.Update(exist); err != nil {
		return nil, fmt.Errorf("note update: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteUpdateSuccess, id), "id", id, "status", exist.Status)
	return exist, nil
}

// Publish transitions the author's draft note to published after validating
// the required fields. It is a one-way operation.
func (s *NoteService) Publish(userID, id uint) (*model.TastingNote, error) {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note publish find: %w", err)
	}
	if exist.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("TastingNote[id=%d] publish failed: user_id=%d not owner", id, userID))
	}
	if exist.Status == constants.NoteStatusPublished {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("TastingNote[id=%d] publish failed: already published", id))
	}
	if err := validateForPublish(exist.CoffeeName, exist.RoastLevel); err != nil {
		return nil, err
	}
	exist.Status = constants.NoteStatusPublished
	if err := s.repo.Update(exist); err != nil {
		return nil, fmt.Errorf("note publish: %w", err)
	}
	s.logger.Info("tasting note published", "id", id, "user_id", userID)
	return exist, nil
}

// Delete removes a note owned by the user.
func (s *NoteService) Delete(userID, id uint) error {
	n, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("note delete find: %w", err)
	}
	if n.UserID != userID {
		return util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("TastingNote[id=%d] delete failed: not owner", id))
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("note delete: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteDeleteSuccess, id), "id", id)
	return nil
}

// List filters published notes.
func (s *NoteService) List(roast, origin, keyword string, hot bool, page, pageSize int) ([]model.TastingNote, int64, error) {
	items, total, err := s.repo.List(roast, origin, keyword, hot, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("note list: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteListSuccess, roast, page), "total", total)
	return items, total, nil
}

// ListByUser returns published notes of a user.
func (s *NoteService) ListByUser(userID uint) ([]model.TastingNote, error) {
	items, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("note list by user: %w", err)
	}
	return items, nil
}

// ListDraftsByUser returns the author's own draft notes.
func (s *NoteService) ListDraftsByUser(userID uint) ([]model.TastingNote, error) {
	items, err := s.repo.ListDraftsByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("note list drafts: %w", err)
	}
	return items, nil
}

// AssertPublished returns an error when the note is a draft, guarding
// interactions (comments, likes) that must not touch unpublished content.
func (s *NoteService) AssertPublished(id uint) (*model.TastingNote, error) {
	n, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note assert published: %w", err)
	}
	if n.Status != constants.NoteStatusPublished {
		return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
	}
	return n, nil
}

// AvgScore returns the average overall score of a user's published notes.
func (s *NoteService) AvgScore(userID uint) (float64, error) {
	avg, err := s.repo.AvgScore(userID)
	if err != nil {
		return 0, fmt.Errorf("note avg score: %w", err)
	}
	return avg, nil
}

// TopOrigins returns the top 3 origins by published note count.
func (s *NoteService) TopOrigins(userID uint) ([]string, error) {
	origins, err := s.repo.TopOrigins(userID)
	if err != nil {
		return nil, fmt.Errorf("note top origins: %w", err)
	}
	return origins, nil
}

func validateForPublish(coffeeName, roastLevel string) error {
	if strings.TrimSpace(coffeeName) == "" {
		return util.NewAppError(422, constants.CodeValidationError, "coffee_name is required to publish")
	}
	if !constants.IsValidRoastLevel(roastLevel) {
		return util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("TastingNote[roast_level=%s] publish failed: invalid roast level", roastLevel))
	}
	return nil
}
