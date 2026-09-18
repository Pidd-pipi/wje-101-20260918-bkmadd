package router

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/config"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupDraftHTTP(t *testing.T) (*gin.Engine, *gorm.DB, string, string) {
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
	db.Exec("DELETE FROM likes")
	db.Exec("DELETE FROM comments")
	db.Exec("DELETE FROM tasting_notes")
	db.Exec("DELETE FROM users")

	db.Create(&model.User{ID: 1, Username: "alice", Email: "alice@x.local", PasswordHash: "x", Role: "user"})
	db.Create(&model.User{ID: 2, Username: "bob", Email: "bob@x.local", PasswordHash: "x", Role: "user"})

	cfg := &config.Config{
		JWTSecret: "test-secret", JWTExpire: time.Hour,
		RateLimitReq: 10000, RateLimitWin: time.Minute,
		UploadDir: t.TempDir(),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := Setup(cfg, db, logger)

	token1, _ := util.GenerateToken(1, "alice", constants.RoleUser, cfg.JWTSecret, cfg.JWTExpire)
	token2, _ := util.GenerateToken(2, "bob", constants.RoleUser, cfg.JWTSecret, cfg.JWTExpire)
	return r, db, token1, token2
}

func doJSON(t *testing.T, r http.Handler, method, path, token string, body any) (int, apiEnvelope) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env apiEnvelope
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &env)
	}
	return w.Code, env
}

func TestDraftHTTPLoop(t *testing.T) {
	r, _, alice, bob := setupDraftHTTP(t)

	// Alice saves a draft with minimal content.
	status, env := doJSON(t, r, http.MethodPost, "/api/v1/notes", alice, map[string]any{
		"status": "draft", "notes_text": "半成品",
	})
	if status != http.StatusCreated {
		t.Fatalf("create draft status=%d body=%s", status, env.Message)
	}
	var draft model.TastingNote
	_ = json.Unmarshal(env.Data, &draft)
	if draft.Status != constants.NoteStatusDraft {
		t.Fatalf("draft status = %s", draft.Status)
	}

	// Public feed stays empty.
	status, env = doJSON(t, r, http.MethodGet, "/api/v1/notes", "", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d", status)
	}
	var page struct {
		Total int64 `json:"total"`
	}
	_ = json.Unmarshal(env.Data, &page)
	if page.Total != 0 {
		t.Fatalf("public feed leaked draft: total=%d", page.Total)
	}

	// Bob and anonymous get 404 on the draft detail.
	if s, _ := doJSON(t, r, http.MethodGet, "/api/v1/notes/"+itoa(draft.ID), bob, nil); s != http.StatusNotFound {
		t.Fatalf("bob draft detail status=%d", s)
	}
	if s, _ := doJSON(t, r, http.MethodGet, "/api/v1/notes/"+itoa(draft.ID), "", nil); s != http.StatusNotFound {
		t.Fatalf("anon draft detail status=%d", s)
	}
	// Alice (owner) can open it.
	if s, _ := doJSON(t, r, http.MethodGet, "/api/v1/notes/"+itoa(draft.ID), alice, nil); s != http.StatusOK {
		t.Fatalf("owner draft detail status=%d", s)
	}

	// Bob cannot like or comment on the draft.
	if s, _ := doJSON(t, r, http.MethodPost, "/api/v1/notes/"+itoa(draft.ID)+"/like", bob, nil); s != http.StatusNotFound {
		t.Fatalf("draft like status=%d", s)
	}
	if s, _ := doJSON(t, r, http.MethodPost, "/api/v1/notes/"+itoa(draft.ID)+"/comments", bob, map[string]any{"content": "hi"}); s != http.StatusNotFound {
		t.Fatalf("draft comment status=%d", s)
	}

	// Alice's draft box contains the draft; Bob's is empty.
	status, env = doJSON(t, r, http.MethodGet, "/api/v1/me/notes/drafts", alice, nil)
	if status != http.StatusOK {
		t.Fatalf("my drafts status=%d", status)
	}
	var drafts []model.TastingNote
	_ = json.Unmarshal(env.Data, &drafts)
	if len(drafts) != 1 {
		t.Fatalf("alice drafts = %d", len(drafts))
	}
	// Draft box requires auth.
	if s, _ := doJSON(t, r, http.MethodGet, "/api/v1/me/notes/drafts", "", nil); s != http.StatusUnauthorized {
		t.Fatalf("anon draft box status=%d", s)
	}

	// Bob viewing Alice's profile must not receive drafts.
	status, env = doJSON(t, r, http.MethodGet, "/api/v1/users/1/profile", bob, nil)
	if status != http.StatusOK {
		t.Fatalf("profile status=%d", status)
	}
	if bytes.Contains(env.Data, []byte(`"drafts"`)) {
		t.Fatal("other-user profile leaked drafts field")
	}
	// Alice viewing her own profile does receive drafts.
	_, env = doJSON(t, r, http.MethodGet, "/api/v1/users/1/profile", alice, nil)
	var profile struct {
		Drafts []model.TastingNote `json:"drafts"`
		Notes  []model.TastingNote `json:"notes"`
	}
	_ = json.Unmarshal(env.Data, &profile)
	if len(profile.Drafts) != 1 || len(profile.Notes) != 0 {
		t.Fatalf("own profile drafts=%d notes=%d", len(profile.Drafts), len(profile.Notes))
	}

	// Alice keeps editing, then publishes.
	if s, _ := doJSON(t, r, http.MethodPut, "/api/v1/notes/"+itoa(draft.ID), alice, map[string]any{
		"status": "draft", "coffee_name": "耶加雪菲", "roast_level": "light", "notes_text": "完整了",
	}); s != http.StatusOK {
		t.Fatalf("save draft edit status=%d", s)
	}
	if s, _ := doJSON(t, r, http.MethodPost, "/api/v1/notes/"+itoa(draft.ID)+"/publish", alice, nil); s != http.StatusOK {
		t.Fatalf("publish status=%d", s)
	}
	// After publish: visible in feed, interactions work.
	_, env = doJSON(t, r, http.MethodGet, "/api/v1/notes", "", nil)
	_ = json.Unmarshal(env.Data, &page)
	if page.Total != 1 {
		t.Fatalf("feed after publish total=%d", page.Total)
	}
	if s, _ := doJSON(t, r, http.MethodGet, "/api/v1/notes/"+itoa(draft.ID), bob, nil); s != http.StatusOK {
		t.Fatalf("published detail for bob status=%d", s)
	}
	if s, _ := doJSON(t, r, http.MethodPost, "/api/v1/notes/"+itoa(draft.ID)+"/like", bob, nil); s != http.StatusCreated {
		t.Fatalf("like published status=%d", s)
	}
	// Reverting to draft is rejected.
	if s, _ := doJSON(t, r, http.MethodPut, "/api/v1/notes/"+itoa(draft.ID), alice, map[string]any{
		"status": "draft", "coffee_name": "耶加雪菲", "roast_level": "light",
	}); s != http.StatusUnprocessableEntity {
		t.Fatalf("revert to draft status=%d", s)
	}
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
