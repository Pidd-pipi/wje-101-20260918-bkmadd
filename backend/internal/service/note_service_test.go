package service

import (
	"errors"
	"testing"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

func asAppError(err error) (*util.AppError, bool) {
	var appErr *util.AppError
	return appErr, errors.As(err, &appErr)
}

func newNoteSvc() *NoteService {
	return NewNoteService(repository.NewTastingNoteRepository(nil), newTestLogger())
}

func TestNoteCreateInvalidRoast(t *testing.T) {
	svc := newNoteSvc()
	n := &model.TastingNote{CoffeeName: "测试", RoastLevel: "blue"}
	if _, err := svc.Create(1, n); err == nil {
		t.Error("expected error for invalid roast level")
	}
}

// 草稿允许咖啡名称与烘焙度为空，且落库状态为 draft。
func TestNoteCreateDraftAllowsEmptyFields(t *testing.T) {
	svc := newNoteSvc()
	n := &model.TastingNote{RoastLevel: "", NotesText: "待补充", Status: constants.NoteStatusDraft}
	if err := svc.prepareForCreate(n); err != nil {
		t.Fatalf("draft with empty fields must pass validation, got %v", err)
	}
	if n.Status != constants.NoteStatusDraft {
		t.Fatalf("expected draft status, got %s", n.Status)
	}
	if n.FlavorTags != "[]" {
		t.Fatalf("expected flavor tags defaulted to [], got %s", n.FlavorTags)
	}
}

// 发布必须带咖啡名称。
func TestNotePublishRequiresCoffeeName(t *testing.T) {
	svc := newNoteSvc()
	n := &model.TastingNote{CoffeeName: "  ", RoastLevel: constants.RoastLight, Status: constants.NoteStatusPublished}
	if err := svc.prepareForCreate(n); err == nil {
		t.Fatal("expected 422 for missing coffee name")
	} else if appErr, ok := asAppError(err); !ok || appErr.HTTPStatus != 422 {
		t.Fatalf("expected 422 for missing coffee name, got %v", err)
	}
}

// 发布必须带合法烘焙度。
func TestNotePublishRequiresValidRoast(t *testing.T) {
	svc := newNoteSvc()
	n := &model.TastingNote{CoffeeName: "肯尼亚", RoastLevel: "", Status: constants.NoteStatusPublished}
	if err := svc.prepareForCreate(n); err == nil {
		t.Fatal("expected 422 for missing roast level")
	} else if appErr, ok := asAppError(err); !ok || appErr.HTTPStatus != 422 {
		t.Fatalf("expected 422 for missing roast level, got %v", err)
	}
}

// 非法状态被拒绝。
func TestNoteCreateInvalidStatus(t *testing.T) {
	svc := newNoteSvc()
	n := &model.TastingNote{CoffeeName: "肯尼亚", RoastLevel: constants.RoastLight, Status: "archived"}
	if err := svc.prepareForCreate(n); err == nil {
		t.Fatal("expected 422 for invalid status")
	} else if appErr, ok := asAppError(err); !ok || appErr.HTTPStatus != 422 {
		t.Fatalf("expected 422 for invalid status, got %v", err)
	}
}
