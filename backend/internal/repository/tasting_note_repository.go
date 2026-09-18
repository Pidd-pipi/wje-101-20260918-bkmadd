package repository

import (
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// TastingNoteRepository handles note persistence.
type TastingNoteRepository struct{ db *gorm.DB }

// NewTastingNoteRepository creates the repository.
func NewTastingNoteRepository(db *gorm.DB) *TastingNoteRepository { return &TastingNoteRepository{db: db} }

// Create inserts a note.
func (r *TastingNoteRepository) Create(n *model.TastingNote) error { return translate(r.db.Create(n).Error) }

// FindByID locates a note by id regardless of status (internal use).
func (r *TastingNoteRepository) FindByID(id uint) (*model.TastingNote, error) {
	var n model.TastingNote
	if err := translate(r.db.First(&n, id).Error); err != nil {
		return nil, err
	}
	return &n, nil
}

// FindPublishedByID locates a published note by id; drafts are treated as not found.
func (r *TastingNoteRepository) FindPublishedByID(id uint) (*model.TastingNote, error) {
	var n model.TastingNote
	if err := translate(r.db.Where("status = ?", constants.NoteStatusPublished).First(&n, id).Error); err != nil {
		return nil, err
	}
	return &n, nil
}

// Update persists a note.
func (r *TastingNoteRepository) Update(n *model.TastingNote) error { return translate(r.db.Save(n).Error) }

// Delete removes a note by id.
func (r *TastingNoteRepository) Delete(id uint) error {
	res := r.db.Delete(&model.TastingNote{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List filters published notes by roast/origin/keyword, ordered by score or recency.
func (r *TastingNoteRepository) List(roast, origin, keyword string, hot bool, page, pageSize int) ([]model.TastingNote, int64, error) {
	var items []model.TastingNote
	var total int64
	q := r.db.Model(&model.TastingNote{}).Where("status = ?", constants.NoteStatusPublished)
	if roast != "" {
		q = q.Where("roast_level = ?", roast)
	}
	if origin != "" {
		q = q.Where("origin = ?", origin)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("coffee_name LIKE ? OR notes_text LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id DESC"
	if hot {
		order = "overall_score DESC, id DESC"
	}
	if err := q.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListByUser returns notes of a user filtered by status (empty status means all).
func (r *TastingNoteRepository) ListByUser(userID uint, status string) ([]model.TastingNote, error) {
	var items []model.TastingNote
	q := r.db.Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListDrafts returns a user's draft notes.
func (r *TastingNoteRepository) ListDrafts(userID uint) ([]model.TastingNote, error) {
	return r.ListByUser(userID, constants.NoteStatusDraft)
}

// AvgScore returns the average overall score of a user's published notes.
func (r *TastingNoteRepository) AvgScore(userID uint) (float64, error) {
	var avg float64
	if err := r.db.Model(&model.TastingNote{}).
		Where("user_id = ? AND status = ?", userID, constants.NoteStatusPublished).
		Select("COALESCE(AVG(overall_score), 0)").Scan(&avg).Error; err != nil {
		return 0, err
	}
	return avg, nil
}

// TopOrigins returns the top 3 origins by published note count for a user.
func (r *TastingNoteRepository) TopOrigins(userID uint) ([]string, error) {
	var origins []string
	if err := r.db.Model(&model.TastingNote{}).
		Where("user_id = ? AND status = ? AND origin <> ''", userID, constants.NoteStatusPublished).
		Group("origin").Order("count(*) DESC").Limit(3).
		Pluck("origin", &origins).Error; err != nil {
		return nil, err
	}
	return origins, nil
}
