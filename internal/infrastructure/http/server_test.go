package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

type mockScheduleRepo struct{}

func (m *mockScheduleRepo) Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error {
	return nil
}
func (m *mockScheduleRepo) GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error) {
	return nil, nil
}
func (m *mockScheduleRepo) GetLatest(ctx context.Context) (*entities.Schedule, error) {
	return &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: time.Now(),
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{
								Pairs: []*entities.Pair{
									{
										Name:  "Математика",
										Class: "301",
										Group: "Иванов И.И.",
									},
								},
							},
						},
					},
				},
			},
			102: {ID: 102, Week: time.Now()},
		},
	}, nil
}
func (m *mockScheduleRepo) GetAll(ctx context.Context) (map[time.Time]*entities.Schedule, error) {
	return nil, nil
}
func (m *mockScheduleRepo) GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

type mockTeacherRepo struct{}

func (m *mockTeacherRepo) Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error {
	return nil
}
func (m *mockTeacherRepo) GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) GetLatest(ctx context.Context, name string) (*entities.Teacher, error) {
	return &entities.Teacher{
		Name: name,
		Week: time.Now(),
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{
								Name:  "Математика",
								Class: "301",
								Group: "101",
							},
						},
					},
				},
			},
		},
	}, nil
}
func (m *mockTeacherRepo) GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error) {
	t, _ := m.GetLatest(ctx, "Иванов И.И.")
	return map[string]*entities.Teacher{
		"Иванов И.И.": t,
	}, nil
}
func (m *mockTeacherRepo) GetAll(ctx context.Context) ([]*entities.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return []string{"Иванов И.И.", "Петрова Е.А."}, nil
}
func (m *mockTeacherRepo) GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

func setupTestServer(t *testing.T) (*Server, *sql.DB) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	// Create users table for tests
	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER UNIQUE NOT NULL,
		username TEXT,
		group_id INTEGER,
		teacher_name TEXT,
		state TEXT NOT NULL DEFAULT ''
	);`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	cfg := &config.Config{
		Env: config.EnvDev,
		Bot: config.BotConfig{
			Token:      "test_token",
			HealthPort: "8081",
		},
	}

	log := testLogger()
	schedRepo := &mockScheduleRepo{}
	teachRepo := &mockTeacherRepo{}
	schedSvc := service.NewScheduleService(log, schedRepo)
	teacherSvc := service.NewTeacherService(log, teachRepo, nil, nil)
	analyticsSvc := service.NewAnalyticsService(log, schedRepo, teachRepo, nil)
	userRepo := repository.NewUserRepository(db)

	srv := New(log, cfg, db, schedSvc, teacherSvc, analyticsSvc, userRepo)
	return srv, db
}

func testLogger() *slog.Logger {
	return slog.Default()
}

func TestHandleHealthz(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Fatalf("expected body OK, got %s", rec.Body.String())
	}
}

func TestHandleCalls(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calls", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Calls []CallItem `json:"calls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}

	if len(res.Calls) != 6 {
		t.Fatalf("expected 6 calls, got %d", len(res.Calls))
	}
}

func TestHandleGroupsList(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Groups []int `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}

	if len(res.Groups) != 2 || res.Groups[0] != 101 || res.Groups[1] != 102 {
		t.Fatalf("expected [101, 102], got %v", res.Groups)
	}
}

func TestHandleTeachersList(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/teachers", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Teachers []string `json:"teachers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}

	if len(res.Teachers) != 2 || res.Teachers[0] != "Иванов И.И." {
		t.Fatalf("expected Иванов И.И., got %v", res.Teachers)
	}
}

func TestStaticSPARoot(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleSubjectsList(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subjects", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Subjects map[string]string `json:"subjects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}
}

func TestHandleCabinetsList(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cabinets", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Cabinets []string `json:"cabinets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}
	if len(res.Cabinets) == 0 || res.Cabinets[0] != "301" {
		t.Fatalf("expected cabinet 301, got %v", res.Cabinets)
	}
}

func TestHandleCabinetSchedule(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	// Missing query param
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cabinet", nil)
	rec := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	// Valid query
	req = httptest.NewRequest(http.MethodGet, "/api/v1/cabinet?number=301", nil)
	rec = httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		Cabinet *entities.CabinetSchedule `json:"cabinet"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}
	if res.Cabinet == nil || res.Cabinet.Cabinet != "301" {
		t.Fatalf("expected cabinet 301, got %+v", res.Cabinet)
	}
}

func TestHandleEndings(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/endings?day=0", nil)
	rec := httptest.NewRecorder()

	srv.httpSrv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res struct {
		DayIndex int                   `json:"day_index"`
		Endings  *entities.EndingsStat `json:"endings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}
	if res.DayIndex != 0 {
		t.Fatalf("expected day_index 0, got %d", res.DayIndex)
	}
	if res.Endings == nil || len(res.Endings.Pairs[1]) == 0 || res.Endings.Pairs[1][0] != 101 {
		t.Fatalf("expected group 101 in endings pairs, got %+v", res.Endings)
	}
}

func TestHandleStats(t *testing.T) {
	srv, db := setupTestServer(t)
	defer db.Close()

	// Missing params
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/stats", nil)
	rec := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	// Group stats
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/stats?group=101", nil)
	rec = httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Teacher stats
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/stats?teacher=Иванов+И.И.", nil)
	rec = httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
