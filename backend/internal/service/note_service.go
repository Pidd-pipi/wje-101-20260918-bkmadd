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

// validatePublishable ensures a note carries the fields required to go public.
func validatePublishable(n *model.TastingNote) error {
	if strings.TrimSpace(n.CoffeeName) == "" {
		return util.NewAppError(422, constants.CodeValidationError, "coffee_name is required to publish")
	}
	if !constants.IsValidRoastLevel(n.RoastLevel) {
		return util.NewAppError(422, constants.CodeValidationError, "valid roast_level is required to publish")
	}
	return nil
}

// Create adds a draft or published note for a user.
func (s *NoteService) Create(userID uint, n *model.TastingNote) (*model.TastingNote, error) {
	if err := s.prepareForCreate(n); err != nil {
		return nil, err
	}
	n.UserID = userID
	if err := s.repo.Create(n); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogNoteCreateFailed, n.CoffeeName), "error", err)
		return nil, fmt.Errorf("note create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteCreateSuccess, n.CoffeeName), "id", n.ID, "status", n.Status)
	return n, nil
}

// prepareForCreate normalizes and validates a note before insertion.
func (s *NoteService) prepareForCreate(n *model.TastingNote) error {
	if n.Status == "" {
		n.Status = constants.NoteStatusPublished
	}
	if !constants.IsValidNoteStatus(n.Status) {
		return util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("TastingNote[status=%s] create failed: invalid status", n.Status))
	}
	if n.Status == constants.NoteStatusPublished {
		if err := validatePublishable(n); err != nil {
			return err
		}
	} else if n.RoastLevel != "" && !constants.IsValidRoastLevel(n.RoastLevel) {
		return util.NewAppError(422, constants.CodeValidationError, "invalid roast level")
	}
	if n.FlavorTags == "" {
		n.FlavorTags = "[]"
	}
	return nil
}

// GetVisible returns a note visible to viewerID: drafts only for their owner,
// otherwise drafts are treated as non-existent.
func (s *NoteService) GetVisible(id, viewerID uint) (*model.TastingNote, error) {
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

// Get returns a published note by id; drafts are treated as non-existent.
func (s *NoteService) Get(id uint) (*model.TastingNote, error) {
	n, err := s.repo.FindPublishedByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note get: %w", err)
	}
	return n, nil
}

// Update edits a note owned by the user. Editing never changes status; use Publish to publish a draft.
func (s *NoteService) Update(userID, id uint, n *model.TastingNote) (*model.TastingNote, error) {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("note update find: %w", err)
	}
	if exist.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("TastingNote[id=%d] update failed: user_id=%d not owner", id, userID))
	}
	exist.CoffeeName = n.CoffeeName
	exist.Origin = n.Origin
	exist.RoastLevel = n.RoastLevel
	exist.FlavorTags = n.FlavorTags
	exist.NotesText = n.NotesText
	exist.AromaScore = n.AromaScore
	exist.AcidityScore = n.AcidityScore
	exist.BodyScore = n.BodyScore
	exist.OverallScore = n.OverallScore
	exist.BrewMethod = n.BrewMethod
	exist.BrewRecipeID = n.BrewRecipeID
	exist.ImageURL = n.ImageURL
	if exist.FlavorTags == "" {
		exist.FlavorTags = "[]"
	}
	if exist.Status == constants.NoteStatusPublished {
		if err := validatePublishable(exist); err != nil {
			return nil, err
		}
	} else if exist.RoastLevel != "" && !constants.IsValidRoastLevel(exist.RoastLevel) {
		return nil, util.NewAppError(422, constants.CodeValidationError, "invalid roast level")
	}
	if err := s.repo.Update(exist); err != nil {
		return nil, fmt.Errorf("note update: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNoteUpdateSuccess, id), "id", id, "status", exist.Status)
	return exist, nil
}

// Publish publishes a draft owned by the user. Published notes stay published (no rollback).
func (s *NoteService) Publish(userID, id uint) (*model.TastingNote, error) {
	exist, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
		}
		return nil, fmt.Errorf("note publish find: %w", err)
	}
	if exist.UserID != userID {
		return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("TastingNote[id=%d] not found", id))
	}
	if exist.Status == constants.NoteStatusPublished {
		// Publishing is one-way; republishing is a no-op success.
		return exist, nil
	}
	if err := validatePublishable(exist); err != nil {
		return nil, err
	}
	exist.Status = constants.NoteStatusPublished
	if err := s.repo.Update(exist); err != nil {
		return nil, fmt.Errorf("note publish: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogNotePublishSuccess, id), "id", id)
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

// ListByUser returns notes of a user with the given status (empty means all).
func (s *NoteService) ListByUser(userID uint, status string) ([]model.TastingNote, error) {
	items, err := s.repo.ListByUser(userID, status)
	if err != nil {
		return nil, fmt.Errorf("note list by user: %w", err)
	}
	return items, nil
}

// ListDrafts returns a user's own draft notes.
func (s *NoteService) ListDrafts(userID uint) ([]model.TastingNote, error) {
	items, err := s.repo.ListDrafts(userID)
	if err != nil {
		return nil, fmt.Errorf("note list drafts: %w", err)
	}
	return items, nil
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
