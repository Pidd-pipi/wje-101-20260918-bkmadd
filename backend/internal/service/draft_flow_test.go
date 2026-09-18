package service

import (
	"io"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
)

func newDraftTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.TastingNote{}, &model.BrewRecipe{}, &model.CoffeeBean{},
		&model.Comment{}, &model.Like{}, &model.UserFollow{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Shared in-memory DB leaks rows between tests; clean relevant tables.
	db.Exec("DELETE FROM likes")
	db.Exec("DELETE FROM comments")
	db.Exec("DELETE FROM tasting_notes")
	db.Exec("DELETE FROM users")
	return db
}

func newDraftServices(t *testing.T) (*gorm.DB, *NoteService, *LikeService, *CommentService) {
	db := newDraftTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noteRepo := repository.NewTastingNoteRepository(db)
	likeRepo := repository.NewLikeRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	noteSvc := NewNoteService(noteRepo, logger)
	likeSvc := NewLikeService(likeRepo, noteRepo, logger)
	commentSvc := NewCommentService(commentRepo, noteRepo, logger)
	return db, noteSvc, likeSvc, commentSvc
}

func mkUser(t *testing.T, db *gorm.DB, id uint, name string) {
	t.Helper()
	if err := db.Create(&model.User{ID: id, Username: name, Email: name + "@x.local", PasswordHash: "x", Role: "user"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func TestDraftLifecycle(t *testing.T) {
	db, noteSvc, likeSvc, commentSvc := newDraftServices(t)
	mkUser(t, db, 1, "alice")
	mkUser(t, db, 2, "bob")

	// 1. Draft saves do not require coffee name / roast level.
	draft, err := noteSvc.Create(1, &model.TastingNote{NotesText: "still thinking", Status: constants.NoteStatusDraft})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.Status != constants.NoteStatusDraft {
		t.Fatalf("draft status = %s", draft.Status)
	}
	if draft.RoastLevel != constants.RoastLight {
		t.Fatalf("draft default roast = %s", draft.RoastLevel)
	}

	// 2. Draft is invisible in the public feed.
	items, total, err := noteSvc.List("", "", "", false, 1, 10)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("public list leaked draft: items=%d total=%d err=%v", len(items), total, err)
	}

	// 3. Draft is invisible in other users' profile lists and stats.
	if n, _ := noteSvc.ListByUser(1); len(n) != 0 {
		t.Fatalf("ListByUser leaked draft: %d", len(n))
	}
	if avg, _ := noteSvc.AvgScore(1); avg != 0 {
		t.Fatalf("draft counted in avg: %v", avg)
	}
	if origins, _ := noteSvc.TopOrigins(1); len(origins) != 0 {
		t.Fatalf("draft counted in origins: %v", origins)
	}

	// 4. Draft appears in the author's draft box.
	drafts, err := noteSvc.ListDraftsByUser(1)
	if err != nil || len(drafts) != 1 || drafts[0].ID != draft.ID {
		t.Fatalf("draft box: %+v err=%v", drafts, err)
	}

	// 5. Non-author gets 404 on GET (treated as non-existent); anonymous too.
	if _, err := noteSvc.Get(draft.ID, 2); err == nil {
		t.Fatal("other user could read draft")
	}
	if _, err := noteSvc.Get(draft.ID, 0); err == nil {
		t.Fatal("anonymous could read draft")
	}
	// Owner can open it.
	if _, err := noteSvc.Get(draft.ID, 1); err != nil {
		t.Fatalf("owner get draft: %v", err)
	}

	// 6. Draft cannot be liked or commented on (404 even for the owner).
	if _, err := likeSvc.Like(1, draft.ID); err == nil {
		t.Fatal("draft was liked")
	}
	if _, err := likeSvc.Like(2, draft.ID); err == nil {
		t.Fatal("draft was liked by other user")
	}
	if _, err := commentSvc.Create(1, draft.ID, "hi"); err == nil {
		t.Fatal("draft was commented")
	}
	if _, err := commentSvc.ListByNote(draft.ID); err == nil {
		t.Fatal("draft comments listed")
	}

	// 7. Owner can keep editing the draft.
	updated, err := noteSvc.Update(1, draft.ID, &model.TastingNote{
		CoffeeName: "耶加雪菲", RoastLevel: constants.RoastLight, NotesText: "better now", Status: constants.NoteStatusDraft,
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.NotesText != "better now" || updated.Status != constants.NoteStatusDraft {
		t.Fatalf("draft update result: %+v", updated)
	}
	// Non-owner cannot edit the draft.
	if _, err := noteSvc.Update(2, draft.ID, &model.TastingNote{CoffeeName: "hack", RoastLevel: constants.RoastLight}); err == nil {
		t.Fatal("other user edited draft")
	}

	// 8. Publishing an incomplete draft fails validation.
	incomplete, _ := noteSvc.Create(1, &model.TastingNote{Status: constants.NoteStatusDraft})
	if _, err := noteSvc.Publish(1, incomplete.ID); err == nil {
		t.Fatal("published draft without coffee name")
	}
	// Non-owner cannot publish.
	if _, err := noteSvc.Publish(2, draft.ID); err == nil {
		t.Fatal("other user published draft")
	}

	// 9. Publish the draft: one-way transition.
	published, err := noteSvc.Publish(1, draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != constants.NoteStatusPublished {
		t.Fatalf("status after publish = %s", published.Status)
	}
	if _, err := noteSvc.Publish(1, draft.ID); err == nil {
		t.Fatal("republish did not conflict")
	}
	if _, err := noteSvc.Update(1, draft.ID, &model.TastingNote{
		CoffeeName: "耶加雪菲", RoastLevel: constants.RoastLight, Status: constants.NoteStatusDraft,
	}); err == nil {
		t.Fatal("published note reverted to draft")
	}

	// 10. After publishing it shows up in feed, stats and allows interactions.
	if _, total, _ := noteSvc.List("", "", "耶加", false, 1, 10); total != 1 {
		t.Fatalf("published note missing from search, total=%d", total)
	}
	if n, _ := noteSvc.ListByUser(1); len(n) != 1 {
		t.Fatalf("published note missing on profile: %d", len(n))
	}
	if _, err := likeSvc.Like(2, draft.ID); err != nil {
		t.Fatalf("like published: %v", err)
	}
	if c, _ := likeSvc.CountByUserNotes(1); c != 1 {
		t.Fatalf("likes received = %d", c)
	}
	if _, err := commentSvc.Create(2, draft.ID, "nice"); err != nil {
		t.Fatalf("comment published: %v", err)
	}
}

func TestPublishDirectlyAndInvalidStatus(t *testing.T) {
	db, noteSvc, _, _ := newDraftServices(t)
	mkUser(t, db, 1, "alice")

	// Direct publish requires coffee name.
	if _, err := noteSvc.Create(1, &model.TastingNote{RoastLevel: constants.RoastLight}); err == nil {
		t.Fatal("published without coffee name accepted")
	}
	n, err := noteSvc.Create(1, &model.TastingNote{CoffeeName: "曼特宁", RoastLevel: constants.RoastDark})
	if err != nil {
		t.Fatalf("create published: %v", err)
	}
	if n.Status != constants.NoteStatusPublished {
		t.Fatalf("default status = %s", n.Status)
	}
	if _, err := noteSvc.Create(1, &model.TastingNote{CoffeeName: "x", RoastLevel: constants.RoastLight, Status: "weird"}); err == nil {
		t.Fatal("invalid status accepted")
	}
}
